package snapshot

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/ppiankov/pgpulse/internal/collector"
	"github.com/ppiankov/pgpulse/internal/config"
)

// Collect runs all queries once and returns a structured snapshot.
func Collect(ctx context.Context, db collector.Querier, cfg config.Config) (*Snapshot, error) {
	s := &Snapshot{
		Timestamp: time.Now().UTC(),
	}

	// Detect PG version.
	var versionNum int
	row := db.QueryRowContext(ctx, "SHOW server_version_num")
	if err := row.Scan(&versionNum); err != nil {
		return nil, fmt.Errorf("pg version: %w", err)
	}
	s.PGVersion = versionNum
	useV13 := versionNum >= 130000
	hasPG14 := versionNum >= 140000
	hasPG17 := versionNum >= 170000

	// Detect pg_stat_statements.
	var hasStmt bool
	row = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_statements LIMIT 1")
	var stmtCount int
	if err := row.Scan(&stmtCount); err == nil {
		hasStmt = true
	}

	// Node role.
	var inRecovery bool
	row = db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()")
	if err := row.Scan(&inRecovery); err != nil {
		return nil, fmt.Errorf("role detection: %w", err)
	}
	if inRecovery {
		s.NodeRole = "replica"
	} else {
		s.NodeRole = "primary"
	}

	// Activity.
	if err := collectActivitySnapshot(ctx, db, s, cfg.SlowQueryThreshold.Seconds()); err != nil {
		return nil, fmt.Errorf("activity: %w", err)
	}

	// Database sizes + max_connections.
	if err := collectDatabaseSnapshot(ctx, db, s); err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	// Statements.
	if hasStmt {
		if err := collectStatementsSnapshot(ctx, db, s, useV13, cfg.StmtLimit); err != nil {
			return nil, fmt.Errorf("statements: %w", err)
		}
	}

	// Locks.
	if err := collectLocksSnapshot(ctx, db, s); err != nil {
		return nil, fmt.Errorf("locks: %w", err)
	}

	// Vacuum.
	if err := collectVacuumSnapshot(ctx, db, s); err != nil {
		return nil, fmt.Errorf("vacuum: %w", err)
	}

	// Bloat estimate.
	if err := collectBloatSnapshot(ctx, db, s); err != nil {
		return nil, fmt.Errorf("bloat: %w", err)
	}

	// WAL (PG14+).
	if hasPG14 {
		if err := collectWALSnapshot(ctx, db, s); err != nil {
			return nil, fmt.Errorf("wal: %w", err)
		}
	}

	// Replication (primary only).
	if !inRecovery {
		if err := collectReplicationSnapshot(ctx, db, s); err != nil {
			return nil, fmt.Errorf("replication: %w", err)
		}
	}

	// Checkpoints.
	if err := collectCheckpointSnapshot(ctx, db, s, hasPG17); err != nil {
		return nil, fmt.Errorf("checkpoints: %w", err)
	}

	// Table stats.
	if err := collectTableStatsSnapshot(ctx, db, s); err != nil {
		return nil, fmt.Errorf("table_stats: %w", err)
	}

	s.Status = DeriveStatus(s)
	return s, nil
}

func collectActivitySnapshot(ctx context.Context, db collector.Querier, s *Snapshot, slowThresholdSec float64) error {
	rows, err := db.QueryContext(ctx, collector.ActivityQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	userCounts := make(map[string]int)
	dbCounts := make(map[string]int)
	var longestDuration float64

	for rows.Next() {
		var state, usename, datname, waitEventType string
		var duration sql.NullFloat64

		if err := rows.Scan(&state, &usename, &datname, &waitEventType, &duration); err != nil {
			return err
		}

		s.Connections.Total++
		userCounts[usename]++
		dbCounts[datname]++

		dur := 0.0
		if duration.Valid {
			dur = duration.Float64
		}

		switch state {
		case "active":
			s.Connections.Active++
			longestDuration = math.Max(longestDuration, dur)
			if dur > slowThresholdSec {
				s.Connections.SlowCount++
			}
			if waitEventType != "" {
				s.Connections.Waiting++
			}
		case "idle":
			s.Connections.Idle++
		case "idle in transaction":
			s.Connections.IdleInTxn++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	s.Connections.LongestSeconds = longestDuration
	s.Connections.ByUser = userCounts
	s.Connections.ByDatabase = dbCounts
	return nil
}

func collectDatabaseSnapshot(ctx context.Context, db collector.Querier, s *Snapshot) error {
	rows, err := db.QueryContext(ctx, collector.DatabaseSizeQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var d DatabaseData
		if err := rows.Scan(&d.Name, &d.SizeBytes); err != nil {
			return err
		}
		s.Databases = append(s.Databases, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var maxConns float64
	row := db.QueryRowContext(ctx, "SHOW max_connections")
	if err := row.Scan(&maxConns); err != nil {
		return err
	}
	s.Connections.Max = int(maxConns)
	if maxConns > 0 {
		s.Connections.UsedRatio = float64(s.Connections.Total) / maxConns
	}
	return nil
}

func collectStatementsSnapshot(ctx context.Context, db collector.Querier, s *Snapshot, useV13 bool, limit int) error {
	orderBy := "total_exec_time"
	if !useV13 {
		orderBy = "total_time"
	}
	q := collector.StmtQuery(useV13, orderBy, limit)

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var st StatementData
		if err := rows.Scan(&st.Query, &st.User, &st.Calls, &st.MeanTimeSeconds, &st.TotalTimeSeconds); err != nil {
			return err
		}
		s.Statements = append(s.Statements, st)
	}
	return rows.Err()
}

func collectLocksSnapshot(ctx context.Context, db collector.Querier, s *Snapshot) error {
	rows, err := db.QueryContext(ctx, collector.LockChainsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	typeCounts := make(map[string]int)
	adj := make(map[int][]int)
	blocked := make(map[int]bool)

	for rows.Next() {
		var blockedPID, blockingPID int
		var lockType string
		if err := rows.Scan(&blockedPID, &blockingPID, &lockType); err != nil {
			return err
		}
		s.Locks.BlockedQueries++
		typeCounts[lockType]++
		adj[blockingPID] = append(adj[blockingPID], blockedPID)
		blocked[blockedPID] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	s.Locks.ByType = typeCounts
	s.Locks.ChainMaxDepth = maxChainDepth(adj, blocked)
	return nil
}

// maxChainDepth computes the longest lock wait chain using iterative DFS.
func maxChainDepth(adj map[int][]int, blocked map[int]bool) int {
	maxDepth := 0
	for root := range adj {
		if blocked[root] {
			continue
		}
		type frame struct {
			pid   int
			depth int
		}
		stack := []frame{{root, 0}}
		visited := make(map[int]bool)
		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[f.pid] || f.depth > 10 {
				continue
			}
			visited[f.pid] = true
			if f.depth > maxDepth {
				maxDepth = f.depth
			}
			for _, child := range adj[f.pid] {
				stack = append(stack, frame{child, f.depth + 1})
			}
		}
	}
	return maxDepth
}

func collectVacuumSnapshot(ctx context.Context, db collector.Querier, s *Snapshot) error {
	rows, err := db.QueryContext(ctx, collector.VacuumDeadTuplesQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var v VacuumTableData
		if err := rows.Scan(&v.Name, &v.DeadTuples, &v.DeadTupleRatio, &v.LastVacuumSeconds, &v.LastAutoVacSeconds); err != nil {
			return err
		}
		s.Vacuum.Tables = append(s.Vacuum.Tables, v)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var workers float64
	row := db.QueryRowContext(ctx, collector.AutovacuumWorkersQuery)
	if err := row.Scan(&workers); err != nil {
		return err
	}
	s.Vacuum.WorkersActive = int(workers)

	var maxWorkers float64
	row = db.QueryRowContext(ctx, "SHOW autovacuum_max_workers")
	if err := row.Scan(&maxWorkers); err != nil {
		return err
	}
	s.Vacuum.WorkersMax = int(maxWorkers)

	return nil
}

func collectBloatSnapshot(ctx context.Context, db collector.Querier, s *Snapshot) error {
	rows, err := db.QueryContext(ctx, collector.BloatEstimateQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var tableBytes, deadRatio float64
		if err := rows.Scan(&name, &tableBytes, &deadRatio); err != nil {
			return err
		}
		s.Bloat = append(s.Bloat, BloatEntry{
			Name:       name,
			TableBytes: tableBytes,
			BloatBytes: tableBytes * deadRatio,
			BloatRatio: deadRatio,
		})
	}
	return rows.Err()
}

func collectWALSnapshot(ctx context.Context, db collector.Querier, s *Snapshot) error {
	var walBytes float64
	row := db.QueryRowContext(ctx, "SELECT wal_bytes FROM pg_stat_wal")
	if err := row.Scan(&walBytes); err != nil {
		return err
	}
	s.WAL = &WALData{BytesTotal: walBytes}
	return nil
}

func collectReplicationSnapshot(ctx context.Context, db collector.Querier, s *Snapshot) error {
	rows, err := db.QueryContext(ctx, collector.ReplicationQuery)
	if err != nil {
		return nil // non-fatal — may lack permissions
	}
	defer rows.Close()

	for rows.Next() {
		var r ReplicaData
		if err := rows.Scan(&r.ApplicationName, &r.ClientAddr, &r.LagBytes, &r.LagSeconds); err != nil {
			return err
		}
		s.Replication = append(s.Replication, r)
	}
	return rows.Err()
}

func collectCheckpointSnapshot(ctx context.Context, db collector.Querier, s *Snapshot, hasPG17 bool) error {
	query := collector.CheckpointQueryPre17
	if hasPG17 {
		query = collector.CheckpointQueryPG17
	}
	row := db.QueryRowContext(ctx, query)
	return row.Scan(&s.Checkpoints.Timed, &s.Checkpoints.Requested, &s.Checkpoints.Buffers)
}

func collectTableStatsSnapshot(ctx context.Context, db collector.Querier, s *Snapshot) error {
	rows, err := db.QueryContext(ctx, collector.TableStatsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t TableStatData
		if err := rows.Scan(&t.Name, &t.SeqScanRatio, &t.SeqScans, &t.IdxScans); err != nil {
			return err
		}
		s.TableStats = append(s.TableStats, t)
	}
	return rows.Err()
}

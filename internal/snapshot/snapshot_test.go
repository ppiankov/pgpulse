package snapshot

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ppiankov/pgpulse/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockQuerier struct {
	db *sql.DB
}

func (m *mockQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

func (m *mockQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return m.db.QueryRowContext(ctx, query, args...)
}

func newMock(t *testing.T) (*mockQuerier, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return &mockQuerier{db: db}, mock
}

func setupFullMock(mock sqlmock.Sqlmock) {
	// PG version.
	mock.ExpectQuery("SHOW server_version_num").
		WillReturnRows(sqlmock.NewRows([]string{"server_version_num"}).AddRow(140000))

	// pg_stat_statements check.
	mock.ExpectQuery("SELECT count.*FROM pg_stat_statements.*").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Node role.
	mock.ExpectQuery("SELECT pg_is_in_recovery.*").
		WillReturnRows(sqlmock.NewRows([]string{"pg_is_in_recovery"}).AddRow(false))

	// Activity.
	mock.ExpectQuery("SELECT COALESCE.*FROM pg_stat_activity.*").
		WillReturnRows(sqlmock.NewRows([]string{"state", "usename", "datname", "wait_event_type", "duration_seconds"}).
			AddRow("active", "app", "mydb", "", 1.5).
			AddRow("idle", "app", "mydb", "", nil))

	// Database sizes.
	mock.ExpectQuery("SELECT datname.*FROM pg_database.*").
		WillReturnRows(sqlmock.NewRows([]string{"datname", "size_bytes"}).
			AddRow("mydb", 1073741824))

	// Max connections.
	mock.ExpectQuery("SHOW max_connections").
		WillReturnRows(sqlmock.NewRows([]string{"max_connections"}).AddRow(100))

	// Statements (top by total time).
	mock.ExpectQuery("SELECT LEFT.*FROM pg_stat_statements.*").
		WillReturnRows(sqlmock.NewRows([]string{"query_fingerprint", "usename", "calls", "mean_exec_time_seconds", "total_exec_time_seconds"}).
			AddRow("SELECT * FROM users", "app", 500, 0.01, 5.0))

	// Locks.
	mock.ExpectQuery("SELECT.*FROM pg_locks.*").
		WillReturnRows(sqlmock.NewRows([]string{"blocked_pid", "blocking_pid", "locktype"}))

	// Vacuum dead tuples.
	mock.ExpectQuery("SELECT.*FROM pg_stat_user_tables.*WHERE n_dead_tup.*").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "n_dead_tup", "dead_tuple_ratio", "last_vacuum_seconds", "last_autovacuum_seconds"}).
			AddRow("public.orders", 5000, 0.05, 3600.0, 1800.0))

	// Autovacuum workers.
	mock.ExpectQuery("SELECT count.*FROM pg_stat_activity WHERE backend_type.*").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Autovacuum max workers.
	mock.ExpectQuery("SHOW autovacuum_max_workers").
		WillReturnRows(sqlmock.NewRows([]string{"autovacuum_max_workers"}).AddRow(3))

	// Bloat estimate.
	mock.ExpectQuery("SELECT.*FROM pg_stat_user_tables.*WHERE n_live_tup.*").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "table_bytes", "dead_ratio"}).
			AddRow("public.orders", 1048576, 0.05))

	// WAL (PG14+).
	mock.ExpectQuery("SELECT wal_bytes FROM pg_stat_wal").
		WillReturnRows(sqlmock.NewRows([]string{"wal_bytes"}).AddRow(1073741824))

	// Replication (primary).
	mock.ExpectQuery("SELECT.*FROM pg_stat_replication").
		WillReturnRows(sqlmock.NewRows([]string{"slot", "client_addr", "lag_bytes", "lag_seconds"}).
			AddRow("replica1", "10.0.0.2", 1024, 0.5))

	// Checkpoints (PG14, pre-17).
	mock.ExpectQuery("SELECT checkpoints_timed.*FROM pg_stat_bgwriter").
		WillReturnRows(sqlmock.NewRows([]string{"checkpoints_timed", "checkpoints_req", "buffers_checkpoint"}).
			AddRow(100, 5, 50000))

	// Table stats.
	mock.ExpectQuery("SELECT.*seq_scan_ratio.*FROM pg_stat_user_tables.*WHERE n_live_tup.*").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "seq_scan_ratio", "seq_scan", "idx_scan"}).
			AddRow("public.orders", 0.1, 100, 900))
}

func TestCollect_FullSnapshot(t *testing.T) {
	q, mock := newMock(t)
	defer q.db.Close()

	setupFullMock(mock)

	cfg := config.Config{
		SlowQueryThreshold: 5 * time.Second,
		StmtLimit:          50,
	}

	snap, err := Collect(context.Background(), q, cfg)
	require.NoError(t, err)

	assert.Equal(t, SeverityHealthy, snap.Status)
	assert.Equal(t, "primary", snap.NodeRole)
	assert.Equal(t, 140000, snap.PGVersion)

	// Connections.
	assert.Equal(t, 2, snap.Connections.Total)
	assert.Equal(t, 1, snap.Connections.Active)
	assert.Equal(t, 1, snap.Connections.Idle)
	assert.Equal(t, 100, snap.Connections.Max)
	assert.InDelta(t, 0.02, snap.Connections.UsedRatio, 0.001)

	// Databases.
	require.Len(t, snap.Databases, 1)
	assert.Equal(t, "mydb", snap.Databases[0].Name)

	// Statements.
	require.Len(t, snap.Statements, 1)
	assert.Equal(t, "SELECT * FROM users", snap.Statements[0].Query)
	assert.Equal(t, float64(500), snap.Statements[0].Calls)

	// Locks.
	assert.Equal(t, 0, snap.Locks.BlockedQueries)
	assert.Equal(t, 0, snap.Locks.ChainMaxDepth)

	// Vacuum.
	require.Len(t, snap.Vacuum.Tables, 1)
	assert.Equal(t, "public.orders", snap.Vacuum.Tables[0].Name)
	assert.Equal(t, 1, snap.Vacuum.WorkersActive)
	assert.Equal(t, 3, snap.Vacuum.WorkersMax)

	// Bloat.
	require.Len(t, snap.Bloat, 1)
	assert.InDelta(t, 0.05, snap.Bloat[0].BloatRatio, 0.001)

	// WAL.
	require.NotNil(t, snap.WAL)
	assert.Equal(t, float64(1073741824), snap.WAL.BytesTotal)

	// Replication.
	require.Len(t, snap.Replication, 1)
	assert.Equal(t, "replica1", snap.Replication[0].ApplicationName)

	// Checkpoints.
	assert.Equal(t, float64(100), snap.Checkpoints.Timed)
	assert.Equal(t, float64(5), snap.Checkpoints.Requested)

	// Table stats.
	require.Len(t, snap.TableStats, 1)
}

func TestDeriveStatus_Critical(t *testing.T) {
	s := &Snapshot{
		Connections: ConnectionsData{Total: 95, Max: 100, UsedRatio: 0.95},
	}
	assert.Equal(t, SeverityCritical, DeriveStatus(s))
}

func TestDeriveStatus_Degraded(t *testing.T) {
	s := &Snapshot{
		Connections: ConnectionsData{Total: 80, Max: 100, UsedRatio: 0.80},
	}
	assert.Equal(t, SeverityDegraded, DeriveStatus(s))
}

func TestDeriveStatus_Healthy(t *testing.T) {
	s := &Snapshot{
		Connections: ConnectionsData{Total: 10, Max: 100, UsedRatio: 0.1},
	}
	assert.Equal(t, SeverityHealthy, DeriveStatus(s))
}

func TestFilterUnhealthy(t *testing.T) {
	s := &Snapshot{
		Vacuum: VacuumData{
			Tables: []VacuumTableData{
				{Name: "healthy_table", DeadTupleRatio: 0.01},
				{Name: "bloated_table", DeadTupleRatio: 0.25},
			},
		},
		Bloat: []BloatEntry{
			{Name: "ok", BloatRatio: 0.02},
			{Name: "bad", BloatRatio: 0.15},
		},
	}

	filtered := FilterUnhealthy(s)
	require.Len(t, filtered.Vacuum.Tables, 1)
	assert.Equal(t, "bloated_table", filtered.Vacuum.Tables[0].Name)
	require.Len(t, filtered.Bloat, 1)
	assert.Equal(t, "bad", filtered.Bloat[0].Name)
}

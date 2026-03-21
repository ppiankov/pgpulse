package snapshot

import "time"

// Severity represents the overall health status.
type Severity string

const (
	SeverityHealthy  Severity = "healthy"
	SeverityDegraded Severity = "degraded"
	SeverityCritical Severity = "critical"
)

// Snapshot is the complete one-shot health picture of a PostgreSQL instance.
type Snapshot struct {
	Timestamp   time.Time       `json:"timestamp"`
	Status      Severity        `json:"status"`
	NodeRole    string          `json:"node_role"`
	PGVersion   int             `json:"pg_version"`
	Connections ConnectionsData `json:"connections"`
	Databases   []DatabaseData  `json:"databases"`
	Statements  []StatementData `json:"statements,omitempty"`
	Locks       LockData        `json:"locks"`
	Vacuum      VacuumData      `json:"vacuum"`
	Bloat       []BloatEntry    `json:"bloat,omitempty"`
	WAL         *WALData        `json:"wal,omitempty"`
	Replication []ReplicaData   `json:"replication,omitempty"`
	Checkpoints CheckpointData  `json:"checkpoints"`
	TableStats  []TableStatData `json:"table_stats,omitempty"`
}

type ConnectionsData struct {
	Total          int            `json:"total"`
	Max            int            `json:"max"`
	UsedRatio      float64        `json:"used_ratio"`
	Active         int            `json:"active"`
	Idle           int            `json:"idle"`
	IdleInTxn      int            `json:"idle_in_transaction"`
	Waiting        int            `json:"waiting"`
	SlowCount      int            `json:"slow_count"`
	LongestSeconds float64        `json:"longest_seconds"`
	ByUser         map[string]int `json:"by_user"`
	ByDatabase     map[string]int `json:"by_database"`
}

type DatabaseData struct {
	Name      string  `json:"name"`
	SizeBytes float64 `json:"size_bytes"`
}

type StatementData struct {
	Query            string  `json:"query"`
	User             string  `json:"user"`
	Calls            float64 `json:"calls"`
	MeanTimeSeconds  float64 `json:"mean_time_seconds"`
	TotalTimeSeconds float64 `json:"total_time_seconds"`
}

type LockData struct {
	BlockedQueries int            `json:"blocked_queries"`
	ChainMaxDepth  int            `json:"chain_max_depth"`
	ByType         map[string]int `json:"by_type"`
}

type VacuumData struct {
	Tables        []VacuumTableData `json:"tables,omitempty"`
	WorkersActive int               `json:"workers_active"`
	WorkersMax    int               `json:"workers_max"`
}

type VacuumTableData struct {
	Name               string  `json:"name"`
	DeadTuples         float64 `json:"dead_tuples"`
	DeadTupleRatio     float64 `json:"dead_tuple_ratio"`
	LastVacuumSeconds  float64 `json:"last_vacuum_seconds"`
	LastAutoVacSeconds float64 `json:"last_autovacuum_seconds"`
}

type BloatEntry struct {
	Name       string  `json:"name"`
	TableBytes float64 `json:"table_bytes"`
	BloatBytes float64 `json:"bloat_bytes"`
	BloatRatio float64 `json:"bloat_ratio"`
}

type WALData struct {
	BytesTotal float64 `json:"bytes_total"`
}

type ReplicaData struct {
	ApplicationName string  `json:"application_name"`
	ClientAddr      string  `json:"client_addr"`
	LagBytes        float64 `json:"lag_bytes"`
	LagSeconds      float64 `json:"lag_seconds"`
}

type CheckpointData struct {
	Timed     float64 `json:"timed"`
	Requested float64 `json:"requested"`
	Buffers   float64 `json:"buffers"`
}

type TableStatData struct {
	Name         string  `json:"name"`
	SeqScanRatio float64 `json:"seq_scan_ratio"`
	SeqScans     float64 `json:"seq_scans"`
	IdxScans     float64 `json:"idx_scans"`
}

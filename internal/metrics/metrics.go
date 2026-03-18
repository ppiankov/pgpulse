package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Up                    prometheus.Gauge
	ScrapeDuration        prometheus.Gauge
	ScrapeErrors          prometheus.Counter
	ActiveQueries         prometheus.Gauge
	QueriesByState        *prometheus.GaugeVec
	ConnectionsByUser     *prometheus.GaugeVec
	ConnectionsByDatabase *prometheus.GaugeVec
	SlowQueries           prometheus.Gauge
	LongestQuerySeconds   prometheus.Gauge
	WaitingQueries        prometheus.Gauge
	ConnectionsMax        prometheus.Gauge
	ConnectionsUsedRatio  prometheus.Gauge
	DatabaseSizeBytes     *prometheus.GaugeVec
	StmtCalls             *prometheus.GaugeVec
	StmtMeanTimeSeconds   *prometheus.GaugeVec
	StmtTotalTimeSeconds  *prometheus.GaugeVec
	StmtTopByCalls        *prometheus.GaugeVec
	StmtTopByMeanTime     *prometheus.GaugeVec

	// Vacuum (WO-14)
	DeadTuples              *prometheus.GaugeVec
	DeadTupleRatio          *prometheus.GaugeVec
	LastVacuumSeconds       *prometheus.GaugeVec
	LastAutovacuumSeconds   *prometheus.GaugeVec
	AutovacuumWorkersActive prometheus.Gauge
	AutovacuumWorkersMax    prometheus.Gauge

	// Bloat (WO-15)
	TableTotalBytes *prometheus.GaugeVec
	TableBytes      *prometheus.GaugeVec
	IndexBytes      *prometheus.GaugeVec
	IndexScansTotal *prometheus.GaugeVec

	// Bloat estimation
	TableBloatBytes *prometheus.GaugeVec
	TableBloatRatio *prometheus.GaugeVec

	// Locks (WO-13)
	LockBlockedQueries prometheus.Gauge
	LockChainMaxDepth  prometheus.Gauge
	LockByType         *prometheus.GaugeVec

	// Regression (WO-12)
	StmtRegressions         prometheus.Gauge
	StmtResetDetected       prometheus.Gauge
	StmtPlanChanges         *prometheus.GaugeVec
	StmtMeanTimeChangeRatio *prometheus.GaugeVec
	StmtCallsDelta          *prometheus.GaugeVec

	// WAL (WO-16)
	WalBytesTotal     prometheus.Gauge
	WalBytesPerSecond prometheus.Gauge

	// Replication (WO-17)
	ReplicationLagBytes          *prometheus.GaugeVec
	ReplicationLagSeconds        *prometheus.GaugeVec
	ReplicationConnectedReplicas prometheus.Gauge

	// Connection lifecycle (WO-18)
	IdleConnections            prometheus.Gauge
	IdleConnectionSecondsTotal prometheus.Gauge
	ConnectionAgeSeconds       prometheus.Histogram

	// Prediction (WO-19)
	ConnectionsExhaustionHours prometheus.Gauge

	// Table stats (WO-20)
	TableSeqScanRatio *prometheus.GaugeVec
	TableSeqScans     *prometheus.GaugeVec
	TableIndexScans   *prometheus.GaugeVec

	// Checkpoint (WO-21)
	CheckpointsTimedTotal     prometheus.Gauge
	CheckpointsRequestedTotal prometheus.Gauge
	BuffersCheckpoint         prometheus.Gauge

	// Node role
	NodeRole *prometheus.GaugeVec
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		Up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_up",
			Help: "Whether the PostgreSQL server is reachable.",
		}),
		ScrapeDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_scrape_duration_seconds",
			Help: "Time taken to collect metrics from PostgreSQL.",
		}),
		ScrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pg_scrape_errors_total",
			Help: "Total number of errors encountered while scraping.",
		}),
		ActiveQueries: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_active_queries",
			Help: "Number of currently active queries.",
		}),
		QueriesByState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_queries_by_state",
			Help: "Number of queries grouped by state.",
		}, []string{"state"}),
		ConnectionsByUser: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_connections_by_user",
			Help: "Number of connections grouped by user.",
		}, []string{"usename"}),
		ConnectionsByDatabase: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_connections_by_database",
			Help: "Number of connections grouped by database.",
		}, []string{"datname"}),
		SlowQueries: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_slow_queries",
			Help: "Number of active queries running longer than the configured threshold.",
		}),
		LongestQuerySeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_longest_query_seconds",
			Help: "Duration in seconds of the longest running query.",
		}),
		WaitingQueries: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_waiting_queries",
			Help: "Number of active queries waiting on locks.",
		}),
		ConnectionsMax: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_connections_max",
			Help: "PostgreSQL max_connections setting.",
		}),
		ConnectionsUsedRatio: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_connections_used_ratio",
			Help: "Ratio of used connections to max_connections.",
		}),
		DatabaseSizeBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_database_size_bytes",
			Help: "Size of each database in bytes.",
		}, []string{"datname"}),
		StmtCalls: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_stat_statements_calls",
			Help: "Number of times the query has been executed.",
		}, []string{"query_fingerprint", "usename"}),
		StmtMeanTimeSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_stat_statements_mean_time_seconds",
			Help: "Mean execution time per query in seconds.",
		}, []string{"query_fingerprint", "usename"}),
		StmtTotalTimeSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_stat_statements_total_time_seconds",
			Help: "Total execution time per query in seconds.",
		}, []string{"query_fingerprint", "usename"}),
		StmtTopByCalls: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_stat_statements_top_by_calls",
			Help: "Call count for top queries ordered by calls.",
		}, []string{"query_fingerprint", "usename"}),
		StmtTopByMeanTime: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_stat_statements_top_by_mean_time_seconds",
			Help: "Mean execution time for top queries ordered by mean time.",
		}, []string{"query_fingerprint", "usename"}),

		// Vacuum
		DeadTuples: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_dead_tuples",
			Help: "Number of dead tuples per table.",
		}, []string{"table"}),
		DeadTupleRatio: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_dead_tuple_ratio",
			Help: "Ratio of dead tuples to total tuples per table.",
		}, []string{"table"}),
		LastVacuumSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_last_vacuum_seconds",
			Help: "Seconds since last manual vacuum per table (-1 if never).",
		}, []string{"table"}),
		LastAutovacuumSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_last_autovacuum_seconds",
			Help: "Seconds since last autovacuum per table (-1 if never).",
		}, []string{"table"}),
		AutovacuumWorkersActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_autovacuum_workers_active",
			Help: "Number of currently active autovacuum workers.",
		}),
		AutovacuumWorkersMax: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_autovacuum_workers_max",
			Help: "Maximum number of autovacuum workers (autovacuum_max_workers).",
		}),

		// Bloat
		TableTotalBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_table_total_bytes",
			Help: "Total size of table including indexes and toast in bytes.",
		}, []string{"table"}),
		TableBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_table_bytes",
			Help: "Heap size of table in bytes.",
		}, []string{"table"}),
		IndexBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_index_bytes",
			Help: "Size of index in bytes.",
		}, []string{"index", "table"}),
		IndexScansTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_index_scans_total",
			Help: "Cumulative number of index scans.",
		}, []string{"index", "table"}),

		// Bloat estimation
		TableBloatBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_table_bloat_bytes",
			Help: "Estimated reclaimable bloat per table based on dead tuple ratio.",
		}, []string{"table"}),
		TableBloatRatio: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_table_bloat_ratio",
			Help: "Estimated bloat ratio per table (dead tuples / total tuples).",
		}, []string{"table"}),

		// Locks
		LockBlockedQueries: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_lock_blocked_queries",
			Help: "Number of queries currently blocked by locks.",
		}),
		LockChainMaxDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_lock_chain_max_depth",
			Help: "Maximum depth of lock wait chains.",
		}),
		LockByType: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_lock_by_type",
			Help: "Number of blocked queries by lock type.",
		}, []string{"lock_type"}),

		// Regression
		StmtRegressions: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_stmt_regressions",
			Help: "Number of queries whose mean execution time regressed beyond threshold.",
		}),
		StmtResetDetected: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_stmt_reset_detected",
			Help: "1 when pg_stat_statements reset was detected this poll cycle.",
		}),
		StmtPlanChanges: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_stmt_plan_changes",
			Help: "Number of new plans generated for a query since last poll (PG14+).",
		}, []string{"query_fingerprint", "usename"}),
		StmtMeanTimeChangeRatio: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_stmt_mean_time_change_ratio",
			Help: "Ratio of current to previous mean execution time per query.",
		}, []string{"query_fingerprint", "usename"}),
		StmtCallsDelta: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_stmt_calls_delta",
			Help: "Change in call count since last poll per query.",
		}, []string{"query_fingerprint", "usename"}),

		// WAL
		WalBytesTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_wal_bytes_total",
			Help: "Total WAL bytes generated.",
		}),
		WalBytesPerSecond: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_wal_bytes_per_second",
			Help: "WAL generation rate in bytes per second.",
		}),

		// Replication
		ReplicationLagBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_replication_lag_bytes",
			Help: "Replication lag in bytes per replica.",
		}, []string{"slot", "client_addr"}),
		ReplicationLagSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_replication_lag_seconds",
			Help: "Replication lag in seconds per replica.",
		}, []string{"slot", "client_addr"}),
		ReplicationConnectedReplicas: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_replication_connected_replicas",
			Help: "Number of connected streaming replicas.",
		}),

		// Connection lifecycle
		IdleConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_idle_connections",
			Help: "Number of idle connections.",
		}),
		IdleConnectionSecondsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_idle_connection_seconds_total",
			Help: "Sum of idle time across all idle connections.",
		}),
		ConnectionAgeSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "pg_connection_age_seconds",
			Help:    "Distribution of connection ages in seconds.",
			Buckets: []float64{1, 10, 60, 300, 900, 3600},
		}),

		// Prediction
		ConnectionsExhaustionHours: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_connections_exhaustion_hours",
			Help: "Predicted hours until max_connections is exhausted (-1 if stable or declining).",
		}),

		// Table stats
		TableSeqScanRatio: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_table_seq_scan_ratio",
			Help: "Ratio of sequential scans to total scans per table.",
		}, []string{"table"}),
		TableSeqScans: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_table_seq_scans",
			Help: "Cumulative sequential scans per table.",
		}, []string{"table"}),
		TableIndexScans: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_table_index_scans",
			Help: "Cumulative index scans per table.",
		}, []string{"table"}),

		// Checkpoint
		CheckpointsTimedTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_checkpoints_timed_total",
			Help: "Number of scheduled checkpoints.",
		}),
		CheckpointsRequestedTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_checkpoints_requested_total",
			Help: "Number of requested checkpoints.",
		}),
		BuffersCheckpoint: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pg_buffers_checkpoint",
			Help: "Buffers written during checkpoints.",
		}),

		// Node role
		NodeRole: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pg_node_role",
			Help: "PostgreSQL node role (1 for current role: primary or replica).",
		}, []string{"role"}),
	}

	reg.MustRegister(
		m.Up,
		m.ScrapeDuration,
		m.ScrapeErrors,
		m.ActiveQueries,
		m.QueriesByState,
		m.ConnectionsByUser,
		m.ConnectionsByDatabase,
		m.SlowQueries,
		m.LongestQuerySeconds,
		m.WaitingQueries,
		m.ConnectionsMax,
		m.ConnectionsUsedRatio,
		m.DatabaseSizeBytes,
		m.StmtCalls,
		m.StmtMeanTimeSeconds,
		m.StmtTotalTimeSeconds,
		m.StmtTopByCalls,
		m.StmtTopByMeanTime,
		m.DeadTuples,
		m.DeadTupleRatio,
		m.LastVacuumSeconds,
		m.LastAutovacuumSeconds,
		m.AutovacuumWorkersActive,
		m.AutovacuumWorkersMax,
		m.TableTotalBytes,
		m.TableBytes,
		m.IndexBytes,
		m.IndexScansTotal,
		m.TableBloatBytes,
		m.TableBloatRatio,
		m.LockBlockedQueries,
		m.LockChainMaxDepth,
		m.LockByType,
		m.StmtRegressions,
		m.StmtResetDetected,
		m.StmtPlanChanges,
		m.StmtMeanTimeChangeRatio,
		m.StmtCallsDelta,
		m.WalBytesTotal,
		m.WalBytesPerSecond,
		m.ReplicationLagBytes,
		m.ReplicationLagSeconds,
		m.ReplicationConnectedReplicas,
		m.IdleConnections,
		m.IdleConnectionSecondsTotal,
		m.ConnectionAgeSeconds,
		m.ConnectionsExhaustionHours,
		m.TableSeqScanRatio,
		m.TableSeqScans,
		m.TableIndexScans,
		m.CheckpointsTimedTotal,
		m.CheckpointsRequestedTotal,
		m.BuffersCheckpoint,
		m.NodeRole,
	)

	return m
}

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
	)

	return m
}

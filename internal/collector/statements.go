package collector

import (
	"context"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

// statementsQueryV13 works for PostgreSQL 13+ where total_time was renamed to total_exec_time.
const statementsQueryV13 = `
SELECT
    LEFT(query, 80) AS query_fingerprint,
    COALESCE(r.rolname, 'unknown') AS usename,
    calls,
    mean_exec_time / 1000.0 AS mean_exec_time_seconds,
    total_exec_time / 1000.0 AS total_exec_time_seconds
FROM pg_stat_statements s
JOIN pg_roles r ON s.userid = r.oid
WHERE query NOT LIKE '%pg_stat%'
ORDER BY total_exec_time DESC
LIMIT 50
`

// statementsQueryV12 works for PostgreSQL 12 and earlier.
const statementsQueryV12 = `
SELECT
    LEFT(query, 80) AS query_fingerprint,
    COALESCE(r.rolname, 'unknown') AS usename,
    calls,
    mean_time / 1000.0 AS mean_exec_time_seconds,
    total_time / 1000.0 AS total_exec_time_seconds
FROM pg_stat_statements s
JOIN pg_roles r ON s.userid = r.oid
WHERE query NOT LIKE '%pg_stat%'
ORDER BY total_time DESC
LIMIT 50
`

func collectStatements(ctx context.Context, db Querier, m *metrics.Metrics, useV13 bool) error {
	query := statementsQueryV12
	if useV13 {
		query = statementsQueryV13
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	m.StmtCalls.Reset()
	m.StmtMeanTimeSeconds.Reset()
	m.StmtTotalTimeSeconds.Reset()

	for rows.Next() {
		var fingerprint, usename string
		var calls, meanTime, totalTime float64

		if err := rows.Scan(&fingerprint, &usename, &calls, &meanTime, &totalTime); err != nil {
			return err
		}

		m.StmtCalls.WithLabelValues(fingerprint, usename).Set(calls)
		m.StmtMeanTimeSeconds.WithLabelValues(fingerprint, usename).Set(meanTime)
		m.StmtTotalTimeSeconds.WithLabelValues(fingerprint, usename).Set(totalTime)
	}

	return rows.Err()
}

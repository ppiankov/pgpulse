package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const stmtBaseV13 = `
SELECT
    LEFT(query, 80) AS query_fingerprint,
    COALESCE(r.rolname, 'unknown') AS usename,
    calls,
    mean_exec_time / 1000.0 AS mean_exec_time_seconds,
    total_exec_time / 1000.0 AS total_exec_time_seconds
FROM pg_stat_statements s
JOIN pg_roles r ON s.userid = r.oid
WHERE query NOT LIKE '%%pg_stat%%'
ORDER BY %s DESC
LIMIT %d
`

const stmtBaseV12 = `
SELECT
    LEFT(query, 80) AS query_fingerprint,
    COALESCE(r.rolname, 'unknown') AS usename,
    calls,
    mean_time / 1000.0 AS mean_exec_time_seconds,
    total_time / 1000.0 AS total_exec_time_seconds
FROM pg_stat_statements s
JOIN pg_roles r ON s.userid = r.oid
WHERE query NOT LIKE '%%pg_stat%%'
ORDER BY %s DESC
LIMIT %d
`

// statementsQueryV13 and statementsQueryV12 are used by regression.go.
var statementsQueryV13 = fmt.Sprintf(stmtBaseV13, "total_exec_time", 50)
var statementsQueryV12 = fmt.Sprintf(stmtBaseV12, "total_time", 50)

func stmtQuery(useV13 bool, orderBy string, limit int) string {
	if useV13 {
		return fmt.Sprintf(stmtBaseV13, orderBy, limit)
	}
	return fmt.Sprintf(stmtBaseV12, orderBy, limit)
}

// valueIndex selects which scanned value to use: 0=calls, 1=meanTime, 2=totalTime.
func collectStmtDimension(ctx context.Context, db Querier, query string, vec *prometheus.GaugeVec, valueIndex int) error {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	vec.Reset()

	for rows.Next() {
		var fingerprint, usename string
		var calls, meanTime, totalTime float64

		if err := rows.Scan(&fingerprint, &usename, &calls, &meanTime, &totalTime); err != nil {
			return err
		}

		vals := [3]float64{calls, meanTime, totalTime}
		vec.WithLabelValues(fingerprint, usename).Set(vals[valueIndex])
	}

	return rows.Err()
}

func collectStatements(ctx context.Context, db Querier, m *metrics.Metrics, useV13 bool, limit int) error {
	// Top by total time — populate all 3 existing metric sets.
	orderTotal := "total_exec_time"
	if !useV13 {
		orderTotal = "total_time"
	}
	q := stmtQuery(useV13, orderTotal, limit)

	rows, err := db.QueryContext(ctx, q)
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

	if err := rows.Err(); err != nil {
		return err
	}

	// Top by calls
	if err := collectStmtDimension(ctx, db, stmtQuery(useV13, "calls", limit),
		m.StmtTopByCalls, 0); err != nil {
		return err
	}

	// Top by mean time
	orderMean := "mean_exec_time"
	if !useV13 {
		orderMean = "mean_time"
	}
	if err := collectStmtDimension(ctx, db, stmtQuery(useV13, orderMean, limit),
		m.StmtTopByMeanTime, 1); err != nil {
		return err
	}

	return nil
}

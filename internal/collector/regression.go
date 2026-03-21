package collector

import (
	"context"
	"fmt"
	"log"
)

type stmtSnapshot struct {
	calls     float64
	meanTime  float64
	totalTime float64
	plans     float64 // PG14+: number of times the query was planned
}

const RegressionQueryV14 = `
SELECT
    LEFT(query, 80) AS query_fingerprint,
    COALESCE(r.rolname, 'unknown') AS usename,
    calls,
    mean_exec_time / 1000.0 AS mean_exec_time_seconds,
    total_exec_time / 1000.0 AS total_exec_time_seconds,
    plans
FROM pg_stat_statements s
JOIN pg_roles r ON s.userid = r.oid
WHERE query NOT LIKE '%%pg_stat%%'
ORDER BY total_exec_time DESC
LIMIT %d
`

func (c *Collector) collectRegression(ctx context.Context) (int, error) {
	var query string
	scanPlans := c.hasPG14
	if scanPlans {
		query = fmt.Sprintf(RegressionQueryV14, c.cfg.StmtLimit)
	} else {
		orderBy := "total_time"
		if c.useV13 {
			orderBy = "total_exec_time"
		}
		query = StmtQuery(c.useV13, orderBy, c.cfg.StmtLimit)
	}

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	current := make(map[string]stmtSnapshot)

	for rows.Next() {
		var fingerprint, usename string
		var calls, meanTime, totalTime, plans float64

		if scanPlans {
			if err := rows.Scan(&fingerprint, &usename, &calls, &meanTime, &totalTime, &plans); err != nil {
				return 0, err
			}
		} else {
			if err := rows.Scan(&fingerprint, &usename, &calls, &meanTime, &totalTime); err != nil {
				return 0, err
			}
		}

		key := fmt.Sprintf("%s|%s", fingerprint, usename)
		current[key] = stmtSnapshot{
			calls:     calls,
			meanTime:  meanTime,
			totalTime: totalTime,
			plans:     plans,
		}
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	// First poll — store snapshot, no deltas to compute.
	if c.prevStmts == nil {
		c.prevStmts = current
		return 0, nil
	}

	// Detect pg_stat_statements_reset(): if total calls dropped significantly,
	// skip regression analysis to avoid false positives.
	var prevTotalCalls, curTotalCalls float64
	for _, s := range c.prevStmts {
		prevTotalCalls += s.calls
	}
	for _, s := range current {
		curTotalCalls += s.calls
	}
	if prevTotalCalls > 0 && curTotalCalls < prevTotalCalls*0.5 {
		log.Printf("pg_stat_statements reset detected (calls dropped from %.0f to %.0f), skipping regression analysis", prevTotalCalls, curTotalCalls)
		c.metrics.StmtResetDetected.Set(1)
		c.metrics.StmtRegressions.Set(0)
		c.prevStmts = current
		return 0, nil
	}
	c.metrics.StmtResetDetected.Set(0)

	c.metrics.StmtMeanTimeChangeRatio.Reset()
	c.metrics.StmtCallsDelta.Reset()
	c.metrics.StmtPlanChanges.Reset()

	var regressionCount float64

	for key, cur := range current {
		prev, ok := c.prevStmts[key]
		if !ok {
			continue // new query, no previous data
		}

		var fingerprint, usename string
		for i := 0; i < len(key); i++ {
			if key[i] == '|' {
				fingerprint = key[:i]
				usename = key[i+1:]
				break
			}
		}

		callsDelta := cur.calls - prev.calls
		c.metrics.StmtCallsDelta.WithLabelValues(fingerprint, usename).Set(callsDelta)

		if prev.meanTime > 0 {
			ratio := cur.meanTime / prev.meanTime
			c.metrics.StmtMeanTimeChangeRatio.WithLabelValues(fingerprint, usename).Set(ratio)

			if ratio > c.regressionThreshold {
				regressionCount++
			}
		}

		// Plan change detection (PG14+).
		if cur.plans > 0 && prev.plans > 0 {
			plansDelta := cur.plans - prev.plans
			if plansDelta > 0 {
				c.metrics.StmtPlanChanges.WithLabelValues(fingerprint, usename).Set(plansDelta)
			}
		}
	}

	c.metrics.StmtRegressions.Set(regressionCount)
	c.prevStmts = current

	return int(regressionCount), nil
}

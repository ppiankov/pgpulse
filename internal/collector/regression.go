package collector

import (
	"context"
	"fmt"
)

type stmtSnapshot struct {
	calls     float64
	meanTime  float64
	totalTime float64
}

func (c *Collector) collectRegression(ctx context.Context) (int, error) {
	orderBy := "total_time"
	if c.useV13 {
		orderBy = "total_exec_time"
	}
	query := stmtQuery(c.useV13, orderBy, c.cfg.StmtLimit)

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	current := make(map[string]stmtSnapshot)

	for rows.Next() {
		var fingerprint, usename string
		var calls, meanTime, totalTime float64

		if err := rows.Scan(&fingerprint, &usename, &calls, &meanTime, &totalTime); err != nil {
			return 0, err
		}

		key := fmt.Sprintf("%s|%s", fingerprint, usename)
		current[key] = stmtSnapshot{
			calls:     calls,
			meanTime:  meanTime,
			totalTime: totalTime,
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

	c.metrics.StmtMeanTimeChangeRatio.Reset()
	c.metrics.StmtCallsDelta.Reset()

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
	}

	c.metrics.StmtRegressions.Set(regressionCount)
	c.prevStmts = current

	return int(regressionCount), nil
}

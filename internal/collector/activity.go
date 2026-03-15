package collector

import (
	"context"
	"database/sql"
	"math"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const activityQuery = `
SELECT
    COALESCE(state, 'unknown') AS state,
    COALESCE(usename, 'unknown') AS usename,
    COALESCE(datname, 'unknown') AS datname,
    COALESCE(wait_event_type, '') AS wait_event_type,
    COALESCE(EXTRACT(EPOCH FROM (now() - query_start)), 0) AS duration_seconds
FROM pg_stat_activity
WHERE pid != pg_backend_pid()
  AND backend_type = 'client backend'
`

func collectActivity(ctx context.Context, db Querier, m *metrics.Metrics, slowThresholdSec float64) (int, error) {
	rows, err := db.QueryContext(ctx, activityQuery)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	stateCounts := make(map[string]float64)
	userCounts := make(map[string]float64)
	dbCounts := make(map[string]float64)

	var activeCount, slowCount, waitingCount float64
	var longestDuration float64
	var totalConns int

	for rows.Next() {
		var state, usename, datname, waitEventType string
		var duration sql.NullFloat64

		if err := rows.Scan(&state, &usename, &datname, &waitEventType, &duration); err != nil {
			return 0, err
		}

		totalConns++
		stateCounts[state]++
		userCounts[usename]++
		dbCounts[datname]++

		dur := 0.0
		if duration.Valid {
			dur = duration.Float64
		}

		if state == "active" {
			activeCount++
			longestDuration = math.Max(longestDuration, dur)

			if dur > slowThresholdSec {
				slowCount++
			}

			if waitEventType != "" {
				waitingCount++
			}
		}
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	m.QueriesByState.Reset()
	for state, count := range stateCounts {
		m.QueriesByState.WithLabelValues(state).Set(count)
	}

	m.ConnectionsByUser.Reset()
	for user, count := range userCounts {
		m.ConnectionsByUser.WithLabelValues(user).Set(count)
	}

	m.ConnectionsByDatabase.Reset()
	for db, count := range dbCounts {
		m.ConnectionsByDatabase.WithLabelValues(db).Set(count)
	}

	m.ActiveQueries.Set(activeCount)
	m.SlowQueries.Set(slowCount)
	m.LongestQuerySeconds.Set(longestDuration)
	m.WaitingQueries.Set(waitingCount)

	return totalConns, nil
}

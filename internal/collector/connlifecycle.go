package collector

import (
	"context"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const ConnLifecycleQuery = `
SELECT
    state,
    EXTRACT(EPOCH FROM (now() - backend_start)) AS age_seconds,
    CASE WHEN state = 'idle'
        THEN EXTRACT(EPOCH FROM (now() - state_change))
        ELSE 0
    END AS idle_seconds
FROM pg_stat_activity
WHERE pid != pg_backend_pid()
  AND backend_type = 'client backend'
`

func collectConnLifecycle(ctx context.Context, db Querier, m *metrics.Metrics) error {
	rows, err := db.QueryContext(ctx, ConnLifecycleQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	var idleCount, idleSecondsTotal float64

	for rows.Next() {
		var state string
		var ageSeconds, idleSeconds float64

		if err := rows.Scan(&state, &ageSeconds, &idleSeconds); err != nil {
			return err
		}

		m.ConnectionAgeSeconds.Observe(ageSeconds)

		if state == "idle" {
			idleCount++
			idleSecondsTotal += idleSeconds
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	m.IdleConnections.Set(idleCount)
	m.IdleConnectionSecondsTotal.Set(idleSecondsTotal)

	return nil
}

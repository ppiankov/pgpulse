package collector

import (
	"context"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const VacuumDeadTuplesQuery = `
SELECT
    schemaname || '.' || relname AS table_name,
    n_dead_tup,
    CASE WHEN n_live_tup + n_dead_tup > 0
        THEN n_dead_tup::float / (n_live_tup + n_dead_tup)
        ELSE 0
    END AS dead_tuple_ratio,
    COALESCE(EXTRACT(EPOCH FROM (now() - last_vacuum)), -1) AS last_vacuum_seconds,
    COALESCE(EXTRACT(EPOCH FROM (now() - last_autovacuum)), -1) AS last_autovacuum_seconds
FROM pg_stat_user_tables
WHERE n_dead_tup > 1000
ORDER BY n_dead_tup DESC
LIMIT 50
`

const AutovacuumWorkersQuery = `
SELECT count(*) FROM pg_stat_activity WHERE backend_type = 'autovacuum worker'
`

func collectVacuum(ctx context.Context, db Querier, m *metrics.Metrics) error {
	rows, err := db.QueryContext(ctx, VacuumDeadTuplesQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	m.DeadTuples.Reset()
	m.DeadTupleRatio.Reset()
	m.LastVacuumSeconds.Reset()
	m.LastAutovacuumSeconds.Reset()

	for rows.Next() {
		var tableName string
		var deadTuples, deadRatio, lastVacuum, lastAutovacuum float64

		if err := rows.Scan(&tableName, &deadTuples, &deadRatio, &lastVacuum, &lastAutovacuum); err != nil {
			return err
		}

		m.DeadTuples.WithLabelValues(tableName).Set(deadTuples)
		m.DeadTupleRatio.WithLabelValues(tableName).Set(deadRatio)
		m.LastVacuumSeconds.WithLabelValues(tableName).Set(lastVacuum)
		m.LastAutovacuumSeconds.WithLabelValues(tableName).Set(lastAutovacuum)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	var workerCount float64
	row := db.QueryRowContext(ctx, AutovacuumWorkersQuery)
	if err := row.Scan(&workerCount); err != nil {
		return err
	}
	m.AutovacuumWorkersActive.Set(workerCount)

	var maxWorkers float64
	row = db.QueryRowContext(ctx, "SHOW autovacuum_max_workers")
	if err := row.Scan(&maxWorkers); err != nil {
		return err
	}
	m.AutovacuumWorkersMax.Set(maxWorkers)

	return nil
}

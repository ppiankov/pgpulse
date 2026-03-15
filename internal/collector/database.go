package collector

import (
	"context"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const databaseSizeQuery = `
SELECT datname, pg_database_size(datname) AS size_bytes
FROM pg_database
WHERE datistemplate = false
`

func collectDatabase(ctx context.Context, db Querier, m *metrics.Metrics, totalConns int) error {
	// Database sizes
	rows, err := db.QueryContext(ctx, databaseSizeQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	m.DatabaseSizeBytes.Reset()

	for rows.Next() {
		var datname string
		var sizeBytes float64

		if err := rows.Scan(&datname, &sizeBytes); err != nil {
			return err
		}

		m.DatabaseSizeBytes.WithLabelValues(datname).Set(sizeBytes)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Max connections
	var maxConns float64
	row := db.QueryRowContext(ctx, "SHOW max_connections")
	if err := row.Scan(&maxConns); err != nil {
		return err
	}

	m.ConnectionsMax.Set(maxConns)

	if maxConns > 0 {
		m.ConnectionsUsedRatio.Set(float64(totalConns) / maxConns)
	}

	return nil
}

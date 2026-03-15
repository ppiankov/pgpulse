package collector

import (
	"context"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const tableSizeQuery = `
SELECT
    schemaname || '.' || relname AS table_name,
    pg_total_relation_size(quote_ident(schemaname) || '.' || quote_ident(relname)) AS total_bytes,
    pg_relation_size(quote_ident(schemaname) || '.' || quote_ident(relname)) AS table_bytes
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(quote_ident(schemaname) || '.' || quote_ident(relname)) DESC
LIMIT 50
`

const indexSizeQuery = `
SELECT
    schemaname || '.' || indexrelname AS index_name,
    schemaname || '.' || relname AS table_name,
    pg_relation_size(quote_ident(schemaname) || '.' || quote_ident(indexrelname)) AS index_bytes,
    idx_scan
FROM pg_stat_user_indexes
ORDER BY pg_relation_size(quote_ident(schemaname) || '.' || quote_ident(indexrelname)) DESC
LIMIT 50
`

func collectBloat(ctx context.Context, db Querier, m *metrics.Metrics) error {
	rows, err := db.QueryContext(ctx, tableSizeQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	m.TableTotalBytes.Reset()
	m.TableBytes.Reset()

	for rows.Next() {
		var tableName string
		var totalBytes, tableBytes float64

		if err := rows.Scan(&tableName, &totalBytes, &tableBytes); err != nil {
			return err
		}

		m.TableTotalBytes.WithLabelValues(tableName).Set(totalBytes)
		m.TableBytes.WithLabelValues(tableName).Set(tableBytes)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	idxRows, err := db.QueryContext(ctx, indexSizeQuery)
	if err != nil {
		return err
	}
	defer idxRows.Close()

	m.IndexBytes.Reset()
	m.IndexScansTotal.Reset()

	for idxRows.Next() {
		var indexName, tableName string
		var indexBytes, idxScan float64

		if err := idxRows.Scan(&indexName, &tableName, &indexBytes, &idxScan); err != nil {
			return err
		}

		m.IndexBytes.WithLabelValues(indexName, tableName).Set(indexBytes)
		m.IndexScansTotal.WithLabelValues(indexName, tableName).Set(idxScan)
	}

	return idxRows.Err()
}

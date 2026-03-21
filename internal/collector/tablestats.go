package collector

import (
	"context"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const TableStatsQuery = `
SELECT
    schemaname || '.' || relname AS table_name,
    CASE WHEN seq_scan + COALESCE(idx_scan, 0) > 0
        THEN seq_scan::float / (seq_scan + COALESCE(idx_scan, 0))
        ELSE 0
    END AS seq_scan_ratio,
    seq_scan,
    COALESCE(idx_scan, 0) AS idx_scan
FROM pg_stat_user_tables
WHERE n_live_tup > 10000
ORDER BY seq_scan DESC
LIMIT 50
`

func collectTableStats(ctx context.Context, db Querier, m *metrics.Metrics) error {
	rows, err := db.QueryContext(ctx, TableStatsQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	m.TableSeqScanRatio.Reset()
	m.TableSeqScans.Reset()
	m.TableIndexScans.Reset()

	for rows.Next() {
		var tableName string
		var seqRatio, seqScans, idxScans float64

		if err := rows.Scan(&tableName, &seqRatio, &seqScans, &idxScans); err != nil {
			return err
		}

		m.TableSeqScanRatio.WithLabelValues(tableName).Set(seqRatio)
		m.TableSeqScans.WithLabelValues(tableName).Set(seqScans)
		m.TableIndexScans.WithLabelValues(tableName).Set(idxScans)
	}

	return rows.Err()
}

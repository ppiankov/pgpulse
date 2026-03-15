package collector

import (
	"context"

	"github.com/ppiankov/pgpulse/internal/metrics"
)

const replicationQuery = `
SELECT
    COALESCE(slot_name, 'none') AS slot,
    COALESCE(client_addr::text, 'local') AS client_addr,
    pg_wal_lsn_diff(sent_lsn, replay_lsn) AS lag_bytes,
    COALESCE(EXTRACT(EPOCH FROM replay_lag), 0) AS lag_seconds
FROM pg_stat_replication
`

func collectReplication(ctx context.Context, db Querier, m *metrics.Metrics) error {
	rows, err := db.QueryContext(ctx, replicationQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	m.ReplicationLagBytes.Reset()
	m.ReplicationLagSeconds.Reset()

	var replicaCount float64

	for rows.Next() {
		var slot, clientAddr string
		var lagBytes, lagSeconds float64

		if err := rows.Scan(&slot, &clientAddr, &lagBytes, &lagSeconds); err != nil {
			return err
		}

		replicaCount++
		m.ReplicationLagBytes.WithLabelValues(slot, clientAddr).Set(lagBytes)
		m.ReplicationLagSeconds.WithLabelValues(slot, clientAddr).Set(lagSeconds)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	m.ReplicationConnectedReplicas.Set(replicaCount)

	return nil
}

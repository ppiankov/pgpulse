package collector

import (
	"context"
)

const checkpointQueryPre17 = `
SELECT checkpoints_timed, checkpoints_req, buffers_checkpoint
FROM pg_stat_bgwriter
`

const checkpointQueryPG17 = `
SELECT num_timed, num_requested, buffers_written
FROM pg_stat_checkpointer
`

func (c *Collector) collectCheckpoint(ctx context.Context) error {
	query := checkpointQueryPre17
	if c.hasPG17 {
		query = checkpointQueryPG17
	}

	var timed, requested, buffers float64
	row := c.db.QueryRowContext(ctx, query)
	if err := row.Scan(&timed, &requested, &buffers); err != nil {
		return err
	}

	c.metrics.CheckpointsTimedTotal.Set(timed)
	c.metrics.CheckpointsRequestedTotal.Set(requested)
	c.metrics.BuffersCheckpoint.Set(buffers)

	return nil
}

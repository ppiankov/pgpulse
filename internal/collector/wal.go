package collector

import (
	"context"
	"time"
)

func (c *Collector) collectWAL(ctx context.Context) error {
	var walBytes float64
	row := c.db.QueryRowContext(ctx, "SELECT wal_bytes FROM pg_stat_wal")
	if err := row.Scan(&walBytes); err != nil {
		return err
	}

	c.metrics.WalBytesTotal.Set(walBytes)

	now := time.Now()
	if c.prevWalBytes > 0 {
		elapsed := now.Sub(c.prevWalTime).Seconds()
		if elapsed > 0 {
			rate := (walBytes - c.prevWalBytes) / elapsed
			c.metrics.WalBytesPerSecond.Set(rate)
		}
	}

	c.prevWalBytes = walBytes
	c.prevWalTime = now

	return nil
}

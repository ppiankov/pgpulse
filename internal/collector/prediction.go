package collector

import (
	"context"
	"time"
)

type connSample struct {
	count float64
	time  time.Time
}

const predictionWindowSize = 12

func (c *Collector) collectPrediction(ctx context.Context, totalConns int) {
	now := time.Now()
	c.connHistory = append(c.connHistory, connSample{
		count: float64(totalConns),
		time:  now,
	})

	// Keep only the last N samples.
	if len(c.connHistory) > predictionWindowSize {
		c.connHistory = c.connHistory[len(c.connHistory)-predictionWindowSize:]
	}

	if len(c.connHistory) < 2 {
		return
	}

	// Get max_connections.
	var maxConns float64
	row := c.db.QueryRowContext(ctx, "SHOW max_connections")
	if err := row.Scan(&maxConns); err != nil || maxConns <= 0 {
		return
	}

	// Linear regression: y = count, x = time (seconds since first sample).
	n := float64(len(c.connHistory))
	t0 := c.connHistory[0].time

	var sumX, sumY, sumXY, sumX2 float64
	for _, s := range c.connHistory {
		x := s.time.Sub(t0).Seconds()
		sumX += x
		sumY += s.count
		sumXY += x * s.count
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		c.metrics.ConnectionsExhaustionHours.Set(-1)
		return
	}

	slope := (n*sumXY - sumX*sumY) / denom // connections per second

	if slope <= 0 {
		c.metrics.ConnectionsExhaustionHours.Set(-1)
		return
	}

	current := c.connHistory[len(c.connHistory)-1].count
	remaining := maxConns - current
	if remaining <= 0 {
		c.metrics.ConnectionsExhaustionHours.Set(0)
		return
	}

	hours := remaining / slope / 3600

	// Cap at 720 hours (30 days). Beyond that the prediction is meaningless noise.
	if hours > 720 {
		c.metrics.ConnectionsExhaustionHours.Set(-1)
		return
	}

	c.metrics.ConnectionsExhaustionHours.Set(hours)
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DSN                string
	MetricsPort        int
	PollInterval       time.Duration
	SlowQueryThreshold time.Duration
}

func Load() (Config, error) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return Config{}, fmt.Errorf("PG_DSN or DATABASE_URL must be set")
	}

	port := 9187
	if v := os.Getenv("METRICS_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid METRICS_PORT: %w", err)
		}
		port = p
	}

	pollInterval := 5 * time.Second
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid POLL_INTERVAL: %w", err)
		}
		pollInterval = d
	}

	slowThreshold := 5 * time.Second
	if v := os.Getenv("SLOW_QUERY_THRESHOLD"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SLOW_QUERY_THRESHOLD: %w", err)
		}
		slowThreshold = d
	}

	return Config{
		DSN:                dsn,
		MetricsPort:        port,
		PollInterval:       pollInterval,
		SlowQueryThreshold: slowThreshold,
	}, nil
}

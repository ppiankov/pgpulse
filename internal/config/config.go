package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DSN                 string
	MetricsPort         int
	PollInterval        time.Duration
	SlowQueryThreshold  time.Duration
	RegressionThreshold float64
	StmtLimit           int

	// Alerting (all optional).
	TelegramBotToken string
	TelegramChatID   string
	AlertWebhookURL  string
	AlertCooldown    time.Duration
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

	regressionThreshold := 2.0
	if v := os.Getenv("REGRESSION_THRESHOLD"); v != "" {
		r, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REGRESSION_THRESHOLD: %w", err)
		}
		regressionThreshold = r
	}

	stmtLimit := 50
	if v := os.Getenv("STMT_LIMIT"); v != "" {
		s, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid STMT_LIMIT: %w", err)
		}
		stmtLimit = s
	}

	alertCooldown := 5 * time.Minute
	if v := os.Getenv("ALERT_COOLDOWN"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ALERT_COOLDOWN: %w", err)
		}
		alertCooldown = d
	}

	return Config{
		DSN:                 dsn,
		MetricsPort:         port,
		PollInterval:        pollInterval,
		SlowQueryThreshold:  slowThreshold,
		RegressionThreshold: regressionThreshold,
		StmtLimit:           stmtLimit,
		TelegramBotToken:    os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:      os.Getenv("TELEGRAM_CHAT_ID"),
		AlertWebhookURL:     os.Getenv("ALERT_WEBHOOK_URL"),
		AlertCooldown:       alertCooldown,
	}, nil
}

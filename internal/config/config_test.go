package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/ppiankov/pgpulse/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		want     config.Config
		wantErr  bool
		errCheck func(t *testing.T, err error)
	}{
		{
			name: "default values",
			env: map[string]string{
				"PG_DSN": "postgres://localhost:5432/db",
			},
			want: config.Config{
				DSN:                "postgres://localhost:5432/db",
				MetricsPort:        9187,
				PollInterval:       5 * time.Second,
				SlowQueryThreshold: 5 * time.Second,
			},
		},
		{
			name: "custom values",
			env: map[string]string{
				"PG_DSN":               "postgres://localhost:5432/custom",
				"METRICS_PORT":         "8080",
				"POLL_INTERVAL":        "10s",
				"SLOW_QUERY_THRESHOLD": "30s",
			},
			want: config.Config{
				DSN:                "postgres://localhost:5432/custom",
				MetricsPort:        8080,
				PollInterval:       10 * time.Second,
				SlowQueryThreshold: 30 * time.Second,
			},
		},
		{
			name: "DATABASE_URL fallback",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost:5432/fallback",
			},
			want: config.Config{
				DSN:                "postgres://localhost:5432/fallback",
				MetricsPort:        9187,
				PollInterval:       5 * time.Second,
				SlowQueryThreshold: 5 * time.Second,
			},
		},
		{
			name:    "missing DSN",
			env:     map[string]string{},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "PG_DSN or DATABASE_URL must be set")
			},
		},
		{
			name: "invalid METRICS_PORT",
			env: map[string]string{
				"PG_DSN":       "postgres://localhost:5432/db",
				"METRICS_PORT": "not-a-number",
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "invalid METRICS_PORT")
			},
		},
		{
			name: "invalid POLL_INTERVAL",
			env: map[string]string{
				"PG_DSN":        "postgres://localhost:5432/db",
				"POLL_INTERVAL": "not-a-duration",
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "invalid POLL_INTERVAL")
			},
		},
		{
			name: "invalid SLOW_QUERY_THRESHOLD",
			env: map[string]string{
				"PG_DSN":               "postgres://localhost:5432/db",
				"SLOW_QUERY_THRESHOLD": "not-a-duration",
			},
			wantErr: true,
			errCheck: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "invalid SLOW_QUERY_THRESHOLD")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env vars
			origEnv := make(map[string]string)
			for k := range tt.env {
				origEnv[k] = os.Getenv(k)
			}

			// Set test env vars
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			defer func() {
				// Restore original env vars
				for k := range tt.env {
					if orig, ok := origEnv[k]; ok {
						os.Setenv(k, orig)
					} else {
						os.Unsetenv(k)
					}
				}
			}()

			got, err := config.Load()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errCheck != nil {
					tt.errCheck(t, err)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

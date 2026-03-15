package collector

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ppiankov/pgpulse/internal/config"
	"github.com/ppiankov/pgpulse/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type mockQuerier struct {
	db *sql.DB
}

func (m *mockQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

func (m *mockQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return m.db.QueryRowContext(ctx, query, args...)
}

func newMockQuerier(t *testing.T) (*mockQuerier, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	return &mockQuerier{db: db}, mock
}

func collectGaugeValue(g prometheus.Gauge) float64 {
	var m dto.Metric
	if err := g.Write(&m); err != nil {
		return 0
	}
	return m.Gauge.GetValue()
}

func collectGaugeVecValue(g *prometheus.GaugeVec, labels ...string) float64 {
	var m dto.Metric
	if err := g.WithLabelValues(labels...).Write(&m); err != nil {
		return 0
	}
	return m.Gauge.GetValue()
}

func TestCollectActivity(t *testing.T) {
	tests := []struct {
		name             string
		slowThresholdSec float64
		wantActive       float64
		wantSlow         float64
		wantWaiting      float64
		wantLongest      float64
		wantStateCounts  map[string]float64
		wantUserCounts   map[string]float64
		wantDBCounts     map[string]float64
		wantError        bool
	}{
		{
			name:             "mixed states with slow query",
			slowThresholdSec: 5.0,
			wantActive:       3,
			wantSlow:         2,
			wantWaiting:      1,
			wantLongest:      15.0,
			wantStateCounts: map[string]float64{
				"active":              3,
				"idle":                1,
				"idle in transaction": 1,
			},
			wantUserCounts: map[string]float64{
				"user1": 3,
				"user2": 2,
			},
			wantDBCounts: map[string]float64{
				"db1": 3,
				"db2": 2,
			},
			wantError: false,
		},
		{
			name:             "no slow queries",
			slowThresholdSec: 5.0,
			wantActive:       2,
			wantSlow:         0,
			wantWaiting:      0,
			wantLongest:      2.0,
			wantStateCounts: map[string]float64{
				"active": 2,
			},
			wantUserCounts: map[string]float64{
				"user1": 1,
				"user2": 1,
			},
			wantDBCounts: map[string]float64{
				"db1": 1,
				"db2": 1,
			},
			wantError: false,
		},
		{
			name:             "query error",
			slowThresholdSec: 5.0,
			wantError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			m := metrics.New(reg)
			ctx := context.Background()

			q, mock := newMockQuerier(t)
			defer q.db.Close()

			if tt.wantError {
				mock.ExpectQuery(activityQuery).WillReturnError(errors.New("query failed"))
				_, err := collectActivity(ctx, q, m, tt.slowThresholdSec)
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if tt.name == "mixed states with slow query" {
				rows := sqlmock.NewRows([]string{"state", "usename", "datname", "wait_event_type", "duration_seconds"}).
					AddRow("active", "user1", "db1", "", 1.0).
					AddRow("active", "user1", "db1", "Lock", 10.0).
					AddRow("active", "user2", "db2", "", 15.0).
					AddRow("idle", "user1", "db1", "", nil).
					AddRow("idle in transaction", "user2", "db2", "Lock", nil)
				mock.ExpectQuery("SELECT COALESCE.*FROM pg_stat_activity.*").WillReturnRows(rows)
			} else {
				rows := sqlmock.NewRows([]string{"state", "usename", "datname", "wait_event_type", "duration_seconds"}).
					AddRow("active", "user1", "db1", "", 1.0).
					AddRow("active", "user2", "db2", "", 2.0)
				mock.ExpectQuery("SELECT COALESCE.*FROM pg_stat_activity.*").WillReturnRows(rows)
			}

			totalConns, err := collectActivity(ctx, q, m, tt.slowThresholdSec)
			if err != nil {
				t.Fatalf("collectActivity failed: %v", err)
			}

			if tt.name == "mixed states with slow query" && totalConns != 5 {
				t.Errorf("totalConns = %d, want 5", totalConns)
			} else if tt.name == "no slow queries" && totalConns != 2 {
				t.Errorf("totalConns = %d, want 2", totalConns)
			}

			if got := collectGaugeValue(m.ActiveQueries); got != tt.wantActive {
				t.Errorf("ActiveQueries = %v, want %v", got, tt.wantActive)
			}

			if got := collectGaugeValue(m.SlowQueries); got != tt.wantSlow {
				t.Errorf("SlowQueries = %v, want %v", got, tt.wantSlow)
			}

			if got := collectGaugeValue(m.WaitingQueries); got != tt.wantWaiting {
				t.Errorf("WaitingQueries = %v, want %v", got, tt.wantWaiting)
			}

			if got := collectGaugeValue(m.LongestQuerySeconds); got != tt.wantLongest {
				t.Errorf("LongestQuerySeconds = %v, want %v", got, tt.wantLongest)
			}

			for state, wantCount := range tt.wantStateCounts {
				if got := collectGaugeVecValue(m.QueriesByState, state); got != wantCount {
					t.Errorf("QueriesByState[%s] = %v, want %v", state, got, wantCount)
				}
			}

			for user, wantCount := range tt.wantUserCounts {
				if got := collectGaugeVecValue(m.ConnectionsByUser, user); got != wantCount {
					t.Errorf("ConnectionsByUser[%s] = %v, want %v", user, got, wantCount)
				}
			}

			for db, wantCount := range tt.wantDBCounts {
				if got := collectGaugeVecValue(m.ConnectionsByDatabase, db); got != wantCount {
					t.Errorf("ConnectionsByDatabase[%s] = %v, want %v", db, got, wantCount)
				}
			}
		})
	}
}

func TestCollectDatabase(t *testing.T) {
	tests := []struct {
		name          string
		maxConns      float64
		totalConns    int
		wantMaxConns  float64
		wantUsedRatio float64
		wantDBSizes   map[string]float64
		wantError     bool
	}{
		{
			name:          "success",
			maxConns:      100,
			totalConns:    50,
			wantMaxConns:  100,
			wantUsedRatio: 0.5,
			wantDBSizes: map[string]float64{
				"db1": 1024,
				"db2": 2048,
			},
			wantError: false,
		},
		{
			name:          "zero max connections",
			maxConns:      0,
			totalConns:    50,
			wantMaxConns:  0,
			wantUsedRatio: 0,
			wantDBSizes: map[string]float64{
				"db1": 1024,
			},
			wantError: false,
		},
		{
			name:       "query error",
			maxConns:   100,
			totalConns: 50,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			m := metrics.New(reg)
			ctx := context.Background()

			q, mock := newMockQuerier(t)
			defer q.db.Close()

			if tt.wantError {
				mock.ExpectQuery("SELECT datname.*FROM pg_database.*").WillReturnError(errors.New("query failed"))
				err := collectDatabase(ctx, q, m, tt.totalConns)
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if tt.name == "success" {
				dbRows := sqlmock.NewRows([]string{"datname", "size_bytes"}).
					AddRow("db1", 1024).
					AddRow("db2", 2048)
				mock.ExpectQuery("SELECT datname.*FROM pg_database.*").WillReturnRows(dbRows)
			} else {
				dbRows := sqlmock.NewRows([]string{"datname", "size_bytes"}).
					AddRow("db1", 1024)
				mock.ExpectQuery("SELECT datname.*FROM pg_database.*").WillReturnRows(dbRows)
			}

			rows := sqlmock.NewRows([]string{"max_connections"}).AddRow(tt.maxConns)
			mock.ExpectQuery("SHOW max_connections").WillReturnRows(rows)

			err := collectDatabase(ctx, q, m, tt.totalConns)
			if err != nil {
				t.Fatalf("collectDatabase failed: %v", err)
			}

			if got := collectGaugeValue(m.ConnectionsMax); got != tt.wantMaxConns {
				t.Errorf("ConnectionsMax = %v, want %v", got, tt.wantMaxConns)
			}

			if got := collectGaugeValue(m.ConnectionsUsedRatio); got != tt.wantUsedRatio {
				t.Errorf("ConnectionsUsedRatio = %v, want %v", got, tt.wantUsedRatio)
			}

			for db, wantSize := range tt.wantDBSizes {
				if got := collectGaugeVecValue(m.DatabaseSizeBytes, db); got != wantSize {
					t.Errorf("DatabaseSizeBytes[%s] = %v, want %v", db, got, wantSize)
				}
			}
		})
	}
}

func TestCollectStatements(t *testing.T) {
	tests := []struct {
		name          string
		useV13        bool
		wantCalls     map[string]float64
		wantMeanTime  map[string]float64
		wantTotalTime map[string]float64
		wantError     bool
	}{
		{
			name:   "v13 query",
			useV13: true,
			wantCalls: map[string]float64{
				"SELECT * FROM test,user1": 100,
				"INSERT INTO test,user2":   50,
			},
			wantMeanTime: map[string]float64{
				"SELECT * FROM test,user1": 0.5,
				"INSERT INTO test,user2":   0.2,
			},
			wantTotalTime: map[string]float64{
				"SELECT * FROM test,user1": 50.0,
				"INSERT INTO test,user2":   10.0,
			},
			wantError: false,
		},
		{
			name:   "v12 query",
			useV13: false,
			wantCalls: map[string]float64{
				"SELECT * FROM test,user1": 100,
			},
			wantMeanTime: map[string]float64{
				"SELECT * FROM test,user1": 0.5,
			},
			wantTotalTime: map[string]float64{
				"SELECT * FROM test,user1": 50.0,
			},
			wantError: false,
		},
		{
			name:      "query error",
			useV13:    true,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			m := metrics.New(reg)
			ctx := context.Background()

			q, mock := newMockQuerier(t)
			defer q.db.Close()

			if tt.wantError {
				mock.ExpectQuery("SELECT LEFT.*FROM pg_stat_statements.*").WillReturnError(errors.New("query failed"))
				err := collectStatements(ctx, q, m, tt.useV13, 50)
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			stmtCols := []string{"query_fingerprint", "usename", "calls", "mean_exec_time_seconds", "total_exec_time_seconds"}
			emptyStmtRows := func() *sqlmock.Rows { return sqlmock.NewRows(stmtCols) }

			if tt.name == "v13 query" {
				rows := sqlmock.NewRows(stmtCols).
					AddRow("SELECT * FROM test", "user1", 100, 0.5, 50.0).
					AddRow("INSERT INTO test", "user2", 50, 0.2, 10.0)
				mock.ExpectQuery("SELECT LEFT.*FROM pg_stat_statements.*").WillReturnRows(rows)
			} else {
				rows := sqlmock.NewRows(stmtCols).
					AddRow("SELECT * FROM test", "user1", 100, 0.5, 50.0)
				mock.ExpectQuery("SELECT LEFT.*FROM pg_stat_statements.*").WillReturnRows(rows)
			}
			// Two additional dimension queries (top-by-calls, top-by-mean-time).
			mock.ExpectQuery("SELECT LEFT.*FROM pg_stat_statements.*").WillReturnRows(emptyStmtRows())
			mock.ExpectQuery("SELECT LEFT.*FROM pg_stat_statements.*").WillReturnRows(emptyStmtRows())

			err := collectStatements(ctx, q, m, tt.useV13, 50)
			if err != nil {
				t.Fatalf("collectStatements failed: %v", err)
			}

			for key, wantCalls := range tt.wantCalls {
				if got := collectGaugeVecValue(m.StmtCalls, parseLabels(key)...); got != wantCalls {
					t.Errorf("StmtCalls[%s] = %v, want %v", key, got, wantCalls)
				}
			}

			for key, wantMean := range tt.wantMeanTime {
				if got := collectGaugeVecValue(m.StmtMeanTimeSeconds, parseLabels(key)...); got != wantMean {
					t.Errorf("StmtMeanTimeSeconds[%s] = %v, want %v", key, got, wantMean)
				}
			}

			for key, wantTotal := range tt.wantTotalTime {
				if got := collectGaugeVecValue(m.StmtTotalTimeSeconds, parseLabels(key)...); got != wantTotal {
					t.Errorf("StmtTotalTimeSeconds[%s] = %v, want %v", key, got, wantTotal)
				}
			}
		})
	}
}

func parseLabels(key string) []string {
	var labels []string
	start := 0
	for i, c := range key {
		if c == ',' {
			labels = append(labels, key[start:i])
			start = i + 1
		}
	}
	if start < len(key) {
		labels = append(labels, key[start:])
	}
	return labels
}

func TestCollector_ProbeExtensions(t *testing.T) {
	tests := []struct {
		name     string
		avail    int
		ver      int
		availErr error
		verErr   error
		wantStmt bool
		wantV13  bool
	}{
		{
			name:     "extension available v13",
			avail:    1,
			ver:      130000,
			availErr: nil,
			verErr:   nil,
			wantStmt: true,
			wantV13:  true,
		},
		{
			name:     "extension available v12",
			avail:    1,
			ver:      120000,
			availErr: nil,
			verErr:   nil,
			wantStmt: true,
			wantV13:  false,
		},
		{
			name:     "extension not available",
			avail:    0,
			ver:      130000,
			availErr: errors.New("no rows"),
			verErr:   nil,
			wantStmt: false,
			wantV13:  false,
		},
		{
			name:     "version detection fails",
			avail:    1,
			ver:      0,
			availErr: nil,
			verErr:   errors.New("version failed"),
			wantStmt: true,
			wantV13:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			m := metrics.New(reg)
			cfg := config.Config{
				PollInterval:       5 * time.Second,
				SlowQueryThreshold: 5 * time.Second,
			}

			q, mock := newMockQuerier(t)
			defer q.db.Close()

			if tt.availErr != nil {
				mock.ExpectQuery("SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'").WillReturnError(tt.availErr)
			} else {
				rows := sqlmock.NewRows([]string{"one"}).AddRow(tt.avail)
				mock.ExpectQuery("SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'").WillReturnRows(rows)
			}

			if tt.verErr != nil {
				mock.ExpectQuery("SHOW server_version_num").WillReturnError(tt.verErr)
			} else if tt.availErr == nil {
				rows := sqlmock.NewRows([]string{"version_num"}).AddRow(tt.ver)
				mock.ExpectQuery("SHOW server_version_num").WillReturnRows(rows)
			}

			c := New(q, m, cfg)
			c.ProbeExtensions(context.Background())

			if c.hasStmt != tt.wantStmt {
				t.Errorf("hasStmt = %v, want %v", c.hasStmt, tt.wantStmt)
			}

			if c.hasStmt && c.useV13 != tt.wantV13 {
				t.Errorf("useV13 = %v, want %v", c.useV13, tt.wantV13)
			}
		})
	}
}

func TestCollector_CollectError(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	cfg := config.Config{
		PollInterval:       5 * time.Second,
		SlowQueryThreshold: 5 * time.Second,
	}

	q, mock := newMockQuerier(t)
	defer q.db.Close()

	mock.ExpectQuery("SELECT COALESCE.*FROM pg_stat_activity.*").WillReturnError(errors.New("activity query failed"))

	c := New(q, m, cfg)
	c.hasStmt = false

	c.collect(context.Background())

	if collectGaugeValue(m.Up) != 0 {
		t.Errorf("Up = %v, want 0 after error", collectGaugeValue(m.Up))
	}
}

func TestCollector_CollectSuccess(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	cfg := config.Config{
		PollInterval:       5 * time.Second,
		SlowQueryThreshold: 5 * time.Second,
	}

	q, mock := newMockQuerier(t)
	defer q.db.Close()

	activityRows := sqlmock.NewRows([]string{"state", "usename", "datname", "wait_event_type", "duration_seconds"}).
		AddRow("active", "user1", "db1", "", 1.0)
	mock.ExpectQuery("SELECT COALESCE.*FROM pg_stat_activity.*").WillReturnRows(activityRows)

	dbRows := sqlmock.NewRows([]string{"datname", "size_bytes"}).
		AddRow("db1", 1024)
	mock.ExpectQuery("SELECT datname.*FROM pg_database.*").WillReturnRows(dbRows)

	maxConnRows := sqlmock.NewRows([]string{"max_connections"}).AddRow(100)
	mock.ExpectQuery("SHOW max_connections").WillReturnRows(maxConnRows)

	c := New(q, m, cfg)
	c.hasStmt = false

	c.collect(context.Background())

	if collectGaugeValue(m.Up) != 1 {
		t.Errorf("Up = %v, want 1 after success", collectGaugeValue(m.Up))
	}
}

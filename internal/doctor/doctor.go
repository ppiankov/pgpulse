package doctor

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// CheckStatus represents the result of a single check.
type CheckStatus string

const (
	StatusPass CheckStatus = "pass"
	StatusWarn CheckStatus = "warn"
	StatusFail CheckStatus = "fail"
)

// Check is a single readiness check result.
type Check struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Detail  string      `json:"detail,omitempty"`
}

// Report is the doctor output.
type Report struct {
	Tool      ToolInfo    `json:"tool"`
	Status    CheckStatus `json:"status"`
	Checks    []Check     `json:"checks"`
	Timestamp string      `json:"timestamp"`
}

// ToolInfo identifies the tool.
type ToolInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RunAll executes all readiness checks and returns a report.
func RunAll(ctx context.Context, dsn string, version string) *Report {
	r := &Report{
		Tool:      ToolInfo{Name: "pgpulse", Version: version},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// 1. DSN validity.
	r.Checks = append(r.Checks, checkDSN(dsn))
	if r.Checks[0].Status == StatusFail {
		r.Status = worstStatus(r.Checks)
		return r
	}

	// 2. Connectivity.
	db, check := checkConnectivity(ctx, dsn)
	r.Checks = append(r.Checks, check)
	if check.Status == StatusFail {
		r.Status = worstStatus(r.Checks)
		return r
	}
	defer db.Close()

	// 3. PG version.
	r.Checks = append(r.Checks, checkVersion(ctx, db))

	// 4. pg_stat_statements.
	r.Checks = append(r.Checks, checkStatStatements(ctx, db))

	// 5. Permissions.
	r.Checks = append(r.Checks, checkPermissions(ctx, db))

	// 6. pg_stat_wal (PG14+).
	r.Checks = append(r.Checks, checkStatWAL(ctx, db))

	// 7. shared_preload_libraries.
	r.Checks = append(r.Checks, checkSharedPreload(ctx, db))

	r.Status = worstStatus(r.Checks)
	return r
}

func checkDSN(dsn string) Check {
	if dsn == "" {
		return Check{Name: "dsn_valid", Status: StatusFail, Message: "PG_DSN is empty"}
	}
	// Try parsing as URL; if that fails, assume libpq key=value format (always valid syntax).
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if _, err := url.Parse(dsn); err != nil {
			return Check{Name: "dsn_valid", Status: StatusFail, Message: "invalid DSN URL", Detail: err.Error()}
		}
	}
	return Check{Name: "dsn_valid", Status: StatusPass, Message: "DSN is parseable"}
}

func checkConnectivity(ctx context.Context, dsn string) (*sql.DB, Check) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, Check{Name: "connectivity", Status: StatusFail, Message: "cannot open connection", Detail: err.Error()}
	}
	db.SetMaxOpenConns(1)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, Check{Name: "connectivity", Status: StatusFail, Message: "ping failed", Detail: err.Error()}
	}
	return db, Check{Name: "connectivity", Status: StatusPass, Message: "connected to PostgreSQL"}
}

func checkVersion(ctx context.Context, db *sql.DB) Check {
	var ver int
	if err := db.QueryRowContext(ctx, "SHOW server_version_num").Scan(&ver); err != nil {
		return Check{Name: "pg_version", Status: StatusFail, Message: "cannot detect version", Detail: err.Error()}
	}
	major := ver / 10000
	if ver < 120000 {
		return Check{Name: "pg_version", Status: StatusFail, Message: fmt.Sprintf("PostgreSQL %d not supported (minimum 12)", major)}
	}
	if ver < 130000 {
		return Check{Name: "pg_version", Status: StatusWarn, Message: fmt.Sprintf("PostgreSQL %d supported but 13+ recommended for full metrics", major)}
	}
	return Check{Name: "pg_version", Status: StatusPass, Message: fmt.Sprintf("PostgreSQL %d detected", major)}
}

func checkStatStatements(ctx context.Context, db *sql.DB) Check {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_statements LIMIT 1").Scan(&count); err != nil {
		return Check{
			Name:    "pg_stat_statements",
			Status:  StatusWarn,
			Message: "pg_stat_statements not available",
			Detail:  "install extension and add to shared_preload_libraries for statement-level metrics",
		}
	}
	return Check{Name: "pg_stat_statements", Status: StatusPass, Message: "pg_stat_statements available"}
}

func checkPermissions(ctx context.Context, db *sql.DB) Check {
	views := []string{
		"pg_stat_activity",
		"pg_stat_user_tables",
		"pg_locks",
	}
	var failures []string
	for _, v := range views {
		var one int
		if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", v)).Scan(&one); err != nil {
			failures = append(failures, v)
		}
	}
	if len(failures) > 0 {
		return Check{
			Name:    "permissions",
			Status:  StatusFail,
			Message: "insufficient permissions",
			Detail:  fmt.Sprintf("cannot read: %s — grant pg_monitor role", strings.Join(failures, ", ")),
		}
	}
	return Check{Name: "permissions", Status: StatusPass, Message: "required views accessible"}
}

func checkStatWAL(ctx context.Context, db *sql.DB) Check {
	var ver int
	if err := db.QueryRowContext(ctx, "SHOW server_version_num").Scan(&ver); err != nil {
		return Check{Name: "pg_stat_wal", Status: StatusWarn, Message: "cannot check WAL stats"}
	}
	if ver < 140000 {
		return Check{Name: "pg_stat_wal", Status: StatusWarn, Message: "pg_stat_wal requires PG14+", Detail: "WAL rate metrics will be skipped"}
	}
	var walBytes float64
	if err := db.QueryRowContext(ctx, "SELECT wal_bytes FROM pg_stat_wal").Scan(&walBytes); err != nil {
		return Check{Name: "pg_stat_wal", Status: StatusWarn, Message: "pg_stat_wal not accessible", Detail: err.Error()}
	}
	return Check{Name: "pg_stat_wal", Status: StatusPass, Message: "pg_stat_wal accessible"}
}

func checkSharedPreload(ctx context.Context, db *sql.DB) Check {
	var libs string
	if err := db.QueryRowContext(ctx, "SHOW shared_preload_libraries").Scan(&libs); err != nil {
		return Check{Name: "shared_preload", Status: StatusWarn, Message: "cannot read shared_preload_libraries"}
	}
	if strings.Contains(libs, "pg_stat_statements") {
		return Check{Name: "shared_preload", Status: StatusPass, Message: "pg_stat_statements in shared_preload_libraries"}
	}
	return Check{
		Name:    "shared_preload",
		Status:  StatusWarn,
		Message: "pg_stat_statements not in shared_preload_libraries",
		Detail:  fmt.Sprintf("current: %s", libs),
	}
}

func worstStatus(checks []Check) CheckStatus {
	worst := StatusPass
	for _, c := range checks {
		if c.Status == StatusFail {
			return StatusFail
		}
		if c.Status == StatusWarn {
			worst = StatusWarn
		}
	}
	return worst
}

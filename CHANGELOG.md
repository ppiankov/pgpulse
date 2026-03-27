# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.1] - 2026-03-27

### Fixed
- Mask username and password in DSN for Telegram and webhook alerts
- Add host to alert header for quick identification of alert source

## [0.6.0] - 2026-03-21

### Added
- Agent-native CLI: `pgpulse status --format json` for one-shot health snapshots
- `--unhealthy` flag to filter status output to problematic items only
- `pgpulse doctor --format json` for runtime readiness validation (7 checks)
- `docs/SKILL.md` for ANCC agent discovery
- `internal/snapshot` package for structured data collection
- `internal/doctor` package for readiness checks
- Exit codes: 0=healthy/pass, 1=degraded/warn, 2=critical/fail

### Changed
- Exported SQL query constants from collector package for reuse

## [0.3.1] - 2026-03-15

### Added
- Configurable `STMT_LIMIT` env var (default 50) for top-N statement queries
- Top queries by call count (`pg_stat_statements_top_by_calls`)
- Top queries by mean execution time (`pg_stat_statements_top_by_mean_time_seconds`)

## [0.3.0] - 2026-03-15

### Added
- WAL generation rate metrics (`pg_wal_bytes_total`, `pg_wal_bytes_per_second`) — PG14+
- Per-replica replication lag (`pg_replication_lag_bytes`, `pg_replication_lag_seconds`)
- Connection lifecycle metrics (idle connections, connection age histogram)
- Connection exhaustion prediction (`pg_connections_exhaustion_hours`)
- Table scan ratio metrics (`pg_table_seq_scan_ratio`, sequential vs index scans)
- Checkpoint pressure metrics (`pg_checkpoints_timed_total`, `pg_checkpoints_requested_total`)
- Auto-detection of PG14 (WAL stats) and PG17 (checkpoint stats)
- Grafana dashboard panels for WAL, replication, lifecycle, and checkpoint

## [0.2.0] - 2026-03-15

### Added
- Query regression detection via statement deltas (`pg_stmt_regressions`, `pg_stmt_mean_time_change_ratio`)
- Lock graph metrics from `pg_locks` (`pg_lock_blocked_queries`, `pg_lock_chain_max_depth`)
- Vacuum and autovacuum health metrics (`pg_dead_tuples`, `pg_dead_tuple_ratio`, `pg_autovacuum_workers_active`)
- Table and index size metrics (`pg_table_total_bytes`, `pg_index_bytes`, `pg_index_scans_total`)
- Configurable `REGRESSION_THRESHOLD` env var (default 2.0)
- Grafana dashboard panels for vacuum, bloat, locks, and regression (47 panels total)

## [0.1.0] - 2026-03-15

### Added
- PostgreSQL metrics exporter with Prometheus endpoint
- Collectors for `pg_stat_activity`, `pg_stat_statements`, and database sizes
- Auto-detection of PostgreSQL version for correct `pg_stat_statements` columns
- Configurable poll interval and slow query threshold via environment variables
- Health check endpoint at `/healthz`
- Grafana dashboard for import (`grafana/pgpulse-dashboard.json`)
- Docker support with multi-stage build
- Helm chart with multi-target support, ServiceMonitor, PrometheusRule, and Grafana ConfigMap
- CI workflow (build, test, lint, security scan)
- Release workflow with goreleaser and Homebrew tap
- systemd unit file and environment template

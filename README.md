[![CI](https://github.com/ppiankov/pgpulse/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/pgpulse/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

# pgpulse

A heartbeat monitor for PostgreSQL — polls `pg_stat_activity`, `pg_stat_statements`, and database sizes, then exposes Prometheus metrics on `/metrics`.

## What pgpulse is

- A lightweight sidecar that connects to PostgreSQL and exposes Prometheus-compatible metrics
- A poll-based exporter (configurable interval) for activity, connections, slow queries, database sizes, and statement statistics
- Compatible with PostgreSQL 12+ (auto-detects `pg_stat_statements` column changes in v13)
- Ships with a ready-to-import Grafana dashboard

## What pgpulse is NOT

- Not a replacement for `pg_stat_monitor` or `pganalyze` — pgpulse exposes raw counters, not analyzed insights
- Not a query profiler — it captures top-50 statements by total time, not full query plans
- Not a connection pooler — it uses 2 connections max and does not proxy traffic
- Not an alerting engine — pair it with Alertmanager or Grafana alerts

## Philosophy

Observe, don't interfere. pgpulse opens a read-only window into PostgreSQL's own statistics views. It adds no extensions, modifies no data, and uses minimal resources. The metrics tell you what's happening; you decide what to do about it.

## Quick start

```bash
# Build
make build

# Run (requires PG_DSN or DATABASE_URL)
export PG_DSN="postgres://localhost:5432/mydb?sslmode=disable"
./bin/pgpulse serve

# Docker
docker build -t pgpulse:dev .
docker run -e PG_DSN="postgres://localhost/mydb" -p 9187:9187 pgpulse:dev
```

Metrics are available at `http://localhost:9187/metrics`, health check at `/healthz`.

### systemd

```bash
# Install binary
sudo cp bin/pgpulse /usr/local/bin/

# Install unit file and environment config
sudo cp deploy/pgpulse.service /etc/systemd/system/
sudo mkdir -p /etc/pgpulse
sudo cp deploy/pgpulse.env.example /etc/pgpulse/pgpulse.env
sudo chmod 600 /etc/pgpulse/pgpulse.env

# Edit PG_DSN in /etc/pgpulse/pgpulse.env, then:
sudo systemctl daemon-reload
sudo systemctl enable --now pgpulse
```

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PG_DSN` or `DATABASE_URL` | *(required)* | PostgreSQL connection string |
| `METRICS_PORT` | `9187` | Port for the HTTP metrics server |
| `POLL_INTERVAL` | `5s` | How often to collect metrics |
| `SLOW_QUERY_THRESHOLD` | `5s` | Duration after which a query is counted as slow |

## Metrics

### Activity (`pg_stat_activity`)
- `pg_active_queries` — currently active queries
- `pg_queries_by_state{state}` — queries grouped by state (active, idle, idle in transaction, etc.)
- `pg_connections_by_user{usename}` — connections per user
- `pg_connections_by_database{datname}` — connections per database
- `pg_slow_queries` — active queries exceeding the slow threshold
- `pg_longest_query_seconds` — duration of the longest running query
- `pg_waiting_queries` — active queries waiting on locks

### Database
- `pg_database_size_bytes{datname}` — size of each database in bytes
- `pg_connections_max` — PostgreSQL `max_connections` setting
- `pg_connections_used_ratio` — ratio of current connections to max

### Statements (`pg_stat_statements`)
- `pg_stat_statements_calls{query_fingerprint,usename}` — execution count per query
- `pg_stat_statements_mean_time_seconds{query_fingerprint,usename}` — mean execution time
- `pg_stat_statements_total_time_seconds{query_fingerprint,usename}` — total execution time

### Scrape health
- `pg_up` — 1 if PostgreSQL is reachable, 0 otherwise
- `pg_scrape_duration_seconds` — time taken to collect metrics
- `pg_scrape_errors_total` — cumulative scrape error count

## Architecture

```
cmd/pgpulse/main.go          CLI entry point (delegates to internal/cli)
internal/
  cli/                        Cobra commands: serve, version
  config/                     Environment-based configuration
  collector/                  Poll loop + collectors (activity, database, statements)
    querier.go                Interface for testability
  metrics/                    Prometheus metric definitions
grafana/
  pgpulse-dashboard.json      Importable Grafana dashboard
deploy/
  pgpulse.service             systemd unit file
  pgpulse.env.example         Environment file template
```

## Grafana dashboard

Import `grafana/pgpulse-dashboard.json` into Grafana. It includes panels for connection overview, query activity, per-user/database breakdowns, top statements by burn rate, and database sizes.

## Known limitations

- Statement fingerprints are truncated to 80 characters
- Top-50 statements only (ordered by total execution time)
- No support for multiple PostgreSQL instances in a single process
- No TLS client certificate auth for the metrics endpoint

## Roadmap

- [ ] CI workflow with build, test, lint, security scan
- [ ] Release workflow with goreleaser and Homebrew tap
- [ ] Collector and config test coverage

## License

[MIT](LICENSE)

[![CI](https://github.com/ppiankov/pgpulse/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/pgpulse/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

# pgpulse

A heartbeat monitor for PostgreSQL — polls `pg_stat_activity`, `pg_stat_statements`, `pg_locks`, and more, then exposes Prometheus metrics on `/metrics`.

## What pgpulse is

- A lightweight sidecar that connects to PostgreSQL and exposes 30+ Prometheus-compatible metrics
- A poll-based exporter for activity, connections, slow queries, vacuum health, table/index bloat, lock graphs, replication lag, WAL rate, checkpoint pressure, and query regression detection
- Compatible with PostgreSQL 12+ (auto-detects version for correct column names)
- Ships with a 63-panel Grafana dashboard and a Helm chart with ServiceMonitor
- Zero config beyond a DSN — sensible defaults for everything

## What pgpulse is NOT

- Not a replacement for `pg_stat_monitor` or `pganalyze` — pgpulse exposes raw counters and deltas, not analyzed insights
- Not a query profiler — it captures top-N statements by total time, calls, and mean time, not full query plans
- Not a connection pooler — it uses 2 connections max and does not proxy traffic
- Not an alerting engine — pair it with Alertmanager or Grafana alerts

## Philosophy

Observe, don't interfere. pgpulse opens a read-only window into PostgreSQL's own statistics views. It adds no extensions, modifies no data, and uses minimal resources. The metrics tell you what's happening; you decide what to do about it.

## PostgreSQL prerequisites

pgpulse queries PostgreSQL's built-in statistics views. Most metrics work out of the box, but some features require configuration.

### Required: a monitoring role

Create a dedicated role with read access to statistics views:

```sql
CREATE ROLE pgpulse WITH LOGIN PASSWORD 'your-secure-password';
GRANT pg_monitor TO pgpulse;
```

The `pg_monitor` role (PostgreSQL 10+) grants read access to `pg_stat_activity`, `pg_stat_replication`, `pg_locks`, and all other statistics views pgpulse needs.

### Optional: pg_stat_statements

Statement-level metrics (top queries by time/calls, query regression detection) require the `pg_stat_statements` extension. pgpulse auto-detects its presence and skips statement metrics if unavailable.

To enable:

```sql
-- 1. Add to postgresql.conf (requires restart):
--    shared_preload_libraries = 'pg_stat_statements'

-- 2. Create the extension in your database:
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

pgpulse auto-detects PostgreSQL version and uses the correct column names (`total_exec_time` on PG13+, `total_time` on PG12).

### Version-specific features

| Feature | Minimum PG version | Notes |
|---------|-------------------|-------|
| Core metrics (activity, connections, sizes) | 12 | Always available |
| `pg_stat_statements` | 12 | Requires extension (see above) |
| WAL generation rate (`pg_stat_wal`) | 14 | Auto-skipped on older versions |
| Checkpoint stats (`pg_stat_checkpointer`) | 17 | Falls back to `pg_stat_bgwriter` on older versions |
| Replication lag (`replay_lag`) | 10 | Graceful skip if not a primary |
| Node role detection (`pg_is_in_recovery`) | 12 | Auto-detects primary vs replica each poll |

### Connection string

pgpulse connects using a standard PostgreSQL DSN:

```
postgres://pgpulse@hostname:5432/postgres?sslmode=require
```

For production, always use `sslmode=require` or `sslmode=verify-full`. Connect to the `postgres` database (or any database where `pg_stat_statements` is installed).

### pgbouncer: connect directly to PostgreSQL

pgpulse **must connect directly to PostgreSQL**, not through pgbouncer or other connection poolers. pgbouncer does not proxy system views (`pg_stat_activity`, `pg_stat_statements`, `pg_locks`, `pg_stat_replication`), so most metrics will fail or return empty results.

| | Direct (port 5432) | pgbouncer (port 6432) |
|---|---|---|
| `pg_stat_activity` | works | empty/error |
| `pg_stat_statements` | works | error |
| `pg_locks` | works | empty |
| `pg_stat_replication` | works | error |
| Database sizes | works | error |

Always point `PG_DSN` at PostgreSQL's native port (default 5432), not the pgbouncer port.

### HA clusters (Patroni, repmgr, Stolon)

pgpulse auto-detects whether each target is a **primary** or **replica** by querying `pg_is_in_recovery()` every poll cycle. This means:

- Specify **all nodes** as separate targets — pgpulse figures out the roles
- **Failover is automatic** — if Patroni promotes a replica, the `pg_node_role` metric flips on the next poll
- The `pg_node_role{role="primary|replica"}` metric lets you filter and compare nodes in Grafana
- **Replication metrics** are only collected on the primary (auto-skipped on replicas)
- All other metrics (activity, vacuum, bloat, locks, etc.) are collected on every node

Example for a 3-node Patroni cluster:

```bash
# Binary — one pgpulse per node
PG_DSN="postgres://pgpulse@node1:5432/postgres" ./bin/pgpulse serve
PG_DSN="postgres://pgpulse@node2:5432/postgres" ./bin/pgpulse serve
PG_DSN="postgres://pgpulse@node3:5432/postgres" ./bin/pgpulse serve

# Helm — all nodes as targets
helm install pgpulse charts/pgpulse/ \
  --set 'targets[0].name=node1' \
  --set 'targets[0].dsn=postgres://pgpulse@node1:5432/postgres' \
  --set 'targets[1].name=node2' \
  --set 'targets[1].dsn=postgres://pgpulse@node2:5432/postgres' \
  --set 'targets[2].name=node3' \
  --set 'targets[2].dsn=postgres://pgpulse@node3:5432/postgres' \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.labels.release=kube-prometheus-stack
```

In Grafana, use the `instance` and `role` template variables to filter by node or compare primary vs replicas.

## Quick start

```bash
# Build
make build

# Run
export PG_DSN="postgres://pgpulse@localhost:5432/postgres?sslmode=disable"
./bin/pgpulse serve

# Docker
docker build -t pgpulse:dev .
docker run -e PG_DSN="postgres://pgpulse@localhost/postgres" -p 9187:9187 pgpulse:dev
```

Metrics at `http://localhost:9187/metrics`, health check at `/healthz`.

### Helm (Kubernetes)

```bash
helm install pgpulse charts/pgpulse/ \
  --set 'targets[0].name=prod-primary' \
  --set 'targets[0].dsn=postgres://pgpulse@primary:5432/postgres' \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.labels.release=kube-prometheus-stack \
  --set prometheusRule.enabled=true \
  --set prometheusRule.labels.release=kube-prometheus-stack
```

Multi-target example (primary + replica):

```bash
helm install pgpulse charts/pgpulse/ \
  --set 'targets[0].name=primary' \
  --set 'targets[0].dsn=postgres://pgpulse@primary:5432/postgres' \
  --set 'targets[1].name=replica' \
  --set 'targets[1].existingSecret=my-pg-replica-secret' \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.labels.release=kube-prometheus-stack \
  --set prometheusRule.enabled=true \
  --set prometheusRule.labels.release=kube-prometheus-stack
```

Each target gets its own Deployment. ServiceMonitor auto-discovers all targets. PrometheusRule ships with 5 opinionated alerts. Grafana dashboard auto-loads via sidecar ConfigMap.

### systemd

```bash
sudo cp bin/pgpulse /usr/local/bin/
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
| `REGRESSION_THRESHOLD` | `2.0` | Mean time ratio above which a query is flagged as regressed |
| `STMT_LIMIT` | `50` | Number of top statements to track per dimension |

## Metrics

### Activity (`pg_stat_activity`)
- `pg_active_queries` — currently active queries
- `pg_queries_by_state{state}` — queries grouped by state
- `pg_connections_by_user{usename}` — connections per user
- `pg_connections_by_database{datname}` — connections per database
- `pg_slow_queries` — active queries exceeding the slow threshold
- `pg_longest_query_seconds` — duration of the longest running query
- `pg_waiting_queries` — active queries waiting on locks

### Connection lifecycle
- `pg_idle_connections` — number of idle connections
- `pg_idle_connection_seconds_total` — sum of idle time across all idle connections
- `pg_connection_age_seconds` — histogram of connection ages
- `pg_connections_exhaustion_hours` — predicted hours until max_connections exhausted (-1 if stable)

### Database
- `pg_database_size_bytes{datname}` — size of each database in bytes
- `pg_connections_max` — PostgreSQL `max_connections` setting
- `pg_connections_used_ratio` — ratio of current connections to max

### Statements (`pg_stat_statements`)
- `pg_stat_statements_calls{query_fingerprint,usename}` — execution count (ordered by total time)
- `pg_stat_statements_mean_time_seconds{query_fingerprint,usename}` — mean execution time
- `pg_stat_statements_total_time_seconds{query_fingerprint,usename}` — total execution time
- `pg_stat_statements_top_by_calls{query_fingerprint,usename}` — top queries by call count
- `pg_stat_statements_top_by_mean_time_seconds{query_fingerprint,usename}` — top queries by mean time

### Query regression detection
- `pg_stmt_regressions` — count of queries whose mean time regressed beyond threshold
- `pg_stmt_mean_time_change_ratio{query_fingerprint,usename}` — current/previous mean time ratio
- `pg_stmt_calls_delta{query_fingerprint,usename}` — change in call count since last poll

### Vacuum health
- `pg_dead_tuples{table}` — dead tuple count per table
- `pg_dead_tuple_ratio{table}` — ratio of dead to total tuples
- `pg_last_vacuum_seconds{table}` — seconds since last manual vacuum (-1 if never)
- `pg_last_autovacuum_seconds{table}` — seconds since last autovacuum (-1 if never)
- `pg_autovacuum_workers_active` — current autovacuum worker count
- `pg_autovacuum_workers_max` — max autovacuum workers setting

### Table and index sizes
- `pg_table_total_bytes{table}` — total size including indexes and toast
- `pg_table_bytes{table}` — heap size only
- `pg_index_bytes{index,table}` — individual index size
- `pg_index_scans_total{index,table}` — cumulative index scans (0 = unused index)

### Table statistics
- `pg_table_seq_scan_ratio{table}` — ratio of sequential scans to total scans
- `pg_table_seq_scans{table}` — cumulative sequential scans
- `pg_table_index_scans{table}` — cumulative index scans

### Lock graph (`pg_locks`)
- `pg_lock_blocked_queries` — total queries blocked by locks
- `pg_lock_chain_max_depth` — deepest lock wait chain
- `pg_lock_by_type{lock_type}` — blocked queries by lock type

### WAL (PG14+)
- `pg_wal_bytes_total` — total WAL bytes generated
- `pg_wal_bytes_per_second` — WAL generation rate

### Replication
- `pg_replication_lag_bytes{slot,client_addr}` — replication lag per replica
- `pg_replication_lag_seconds{slot,client_addr}` — replication lag in seconds
- `pg_replication_connected_replicas` — number of connected replicas

### Checkpoint
- `pg_checkpoints_timed_total` — scheduled checkpoints
- `pg_checkpoints_requested_total` — requested checkpoints
- `pg_buffers_checkpoint` — buffers written during checkpoints

### Scrape health
- `pg_up` — 1 if PostgreSQL is reachable, 0 otherwise
- `pg_scrape_duration_seconds` — time taken to collect metrics
- `pg_scrape_errors_total` — cumulative scrape error count

## Architecture

```
cmd/pgpulse/main.go              CLI entry point (delegates to internal/cli)
internal/
  cli/                            Cobra commands: serve, version
  config/                         Environment-based configuration
  collector/                      Poll loop + 12 collectors
    activity.go                   pg_stat_activity (connections, queries, states)
    database.go                   Database sizes, max_connections
    statements.go                 pg_stat_statements (top-N by 3 dimensions)
    regression.go                 Statement delta analysis (stateful)
    vacuum.go                     Dead tuples, autovacuum workers
    bloat.go                      Table/index sizes
    tablestats.go                 Sequential vs index scan ratios
    locks.go                      Lock graph with chain depth
    wal.go                        WAL generation rate (PG14+, stateful)
    replication.go                Replication lag per replica
    connlifecycle.go              Idle connections, connection age histogram
    prediction.go                 Connection exhaustion prediction (stateful)
    checkpoint.go                 Checkpoint pressure (PG17-aware)
    querier.go                    Interface for testability
  metrics/                        Prometheus metric definitions
charts/pgpulse/                   Helm chart with ServiceMonitor + PrometheusRule
grafana/
  pgpulse-dashboard.json          63-panel importable Grafana dashboard
deploy/
  pgpulse.service                 systemd unit file
  pgpulse.env.example             Environment file template
```

## Grafana dashboard

Import `grafana/pgpulse-dashboard.json` into Grafana or use the Helm chart with `dashboard.enabled=true` for automatic provisioning. The dashboard includes 63 panels across 9 rows: overview stats, query activity, connections, top queries, database sizes, vacuum health, table/index sizes, lock graph, query regression detection, WAL/replication, connection lifecycle, and checkpoint pressure.

## Known limitations

- Statement fingerprints are truncated to 80 characters
- No support for multiple PostgreSQL instances in a single process
- No TLS client certificate auth for the metrics endpoint
- Connection exhaustion prediction requires at least 2 poll cycles of data
- WAL metrics require PostgreSQL 14+ (`pg_stat_wal`)

## License

[MIT](LICENSE)

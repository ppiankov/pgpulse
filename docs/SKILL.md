# pgpulse

PostgreSQL health diagnostics for agents and Prometheus.

## Install

```bash
brew install ppiankov/tap/pgpulse
```

Or from source:

```bash
go install github.com/ppiankov/pgpulse/cmd/pgpulse@latest
```

## Commands

### pgpulse status

One-shot health snapshot of a PostgreSQL instance.

```bash
pgpulse status --format json
pgpulse status --format json --unhealthy
```

**Flags:**
- `--format json` — structured JSON output (default)
- `--unhealthy` — filter to only problematic items

**Exit codes:**
- `0` — healthy
- `1` — degraded (warnings present)
- `2` — critical (connection saturation, deep lock chains, replication lag)

**JSON output schema:**

```json
{
  "timestamp": "2026-03-21T12:00:00Z",
  "status": "healthy|degraded|critical",
  "node_role": "primary|replica",
  "pg_version": 140000,
  "connections": {
    "total": 25, "max": 100, "used_ratio": 0.25,
    "active": 4, "idle": 21, "idle_in_transaction": 0,
    "waiting": 0, "slow_count": 0, "longest_seconds": 1.5,
    "by_user": {"app": 20, "admin": 5},
    "by_database": {"mydb": 25}
  },
  "databases": [{"name": "mydb", "size_bytes": 1073741824}],
  "statements": [{"query": "SELECT ...", "user": "app", "calls": 500, "mean_time_seconds": 0.01, "total_time_seconds": 5.0}],
  "locks": {"blocked_queries": 0, "chain_max_depth": 0, "by_type": {}},
  "vacuum": {
    "tables": [{"name": "public.orders", "dead_tuples": 5000, "dead_tuple_ratio": 0.05, "last_vacuum_seconds": 3600, "last_autovacuum_seconds": 1800}],
    "workers_active": 1, "workers_max": 3
  },
  "bloat": [{"name": "public.orders", "table_bytes": 1048576, "bloat_bytes": 52428, "bloat_ratio": 0.05}],
  "wal": {"bytes_total": 1073741824},
  "replication": [{"application_name": "replica1", "client_addr": "10.0.0.2", "lag_bytes": 1024, "lag_seconds": 0.5}],
  "checkpoints": {"timed": 100, "requested": 5, "buffers": 50000},
  "table_stats": [{"name": "public.orders", "seq_scan_ratio": 0.1, "seq_scans": 100, "idx_scans": 900}]
}
```

### pgpulse doctor

Validates runtime readiness for metrics collection.

```bash
pgpulse doctor --format json
```

**Exit codes:**
- `0` — all checks pass
- `1` — warnings (missing optional extensions)
- `2` — failures (cannot connect, insufficient permissions)

**JSON output schema:**

```json
{
  "tool": {"name": "pgpulse", "version": "0.6.1"},
  "status": "pass|warn|fail",
  "checks": [
    {"name": "dsn_valid", "status": "pass", "message": "DSN is parseable"},
    {"name": "connectivity", "status": "pass", "message": "connected to PostgreSQL"},
    {"name": "pg_version", "status": "pass", "message": "PostgreSQL 14 detected"},
    {"name": "pg_stat_statements", "status": "pass", "message": "pg_stat_statements available"},
    {"name": "permissions", "status": "pass", "message": "required views accessible"},
    {"name": "pg_stat_wal", "status": "pass", "message": "pg_stat_wal accessible"},
    {"name": "shared_preload", "status": "pass", "message": "pg_stat_statements in shared_preload_libraries"}
  ],
  "timestamp": "2026-03-21T12:00:00Z"
}
```

### pgpulse serve

Long-running Prometheus metrics exporter (default mode).

```bash
pgpulse serve
```

Exposes `/metrics` in Prometheus text exposition format and `/healthz` for liveness.

**Exit codes:**
- `0` — clean shutdown
- `1` — startup error

### pgpulse version

Print version string.

## What this does NOT do

- Does not monitor MySQL, MongoDB, or any non-PostgreSQL database
- Does not trace application-level queries — only aggregated statistics from pg_stat_statements
- Does not collect metrics through connection poolers (pgbouncer, pgpool) — requires direct PostgreSQL connection
- Does not modify PostgreSQL state — read-only access to statistics views only
- Does not replace Prometheus or Grafana — exports metrics, does not store or visualize them
- Does not perform query rewriting, index recommendations, or automated tuning

## Parsing examples

```bash
# Check if database is healthy
STATUS=$(pgpulse status --format json | jq -r '.status')
if [ "$STATUS" != "healthy" ]; then
  echo "Database degraded: $STATUS"
fi

# Get connection usage ratio
pgpulse status --format json | jq '.connections.used_ratio'

# List tables with high dead tuple ratio
pgpulse status --format json | jq '.vacuum.tables[] | select(.dead_tuple_ratio > 0.1) | .name'

# Show only unhealthy items
pgpulse status --format json --unhealthy | jq '.locks.chain_max_depth'

# Check readiness before deploying
pgpulse doctor --format json | jq '.checks[] | select(.status != "pass")'
```

## Input

| Variable | Required | Description |
|----------|----------|-------------|
| `PG_DSN` or `DATABASE_URL` | Yes | PostgreSQL connection string |
| `SLOW_QUERY_THRESHOLD` | No | Duration threshold for slow queries (default: 5s) |
| `STMT_LIMIT` | No | Number of top statements to track (default: 50) |
| `METRICS_PORT` | No | Port for /metrics endpoint in serve mode (default: 9187) |

## Handoffs

- **Receives from:** deployscope identifies a PostgreSQL workload is unhealthy, agent runs `pgpulse status --format json` for diagnosis
- **Hands off to:** human operator via work order for schema changes, vacuum tuning, index creation, or configuration adjustments
- **Does not answer:** why application queries are slow (use APM), whether to add indexes (use pg_qualstats), whether failover is safe (use patroni/repmgr)

## Trust boundary

Read-only PostgreSQL connection using statistics views (`pg_stat_activity`, `pg_stat_statements`, `pg_locks`, `pg_stat_user_tables`, `pg_stat_replication`, `pg_stat_wal`). Never executes DDL, DML, or any mutating operation. Requires `pg_monitor` role or equivalent read permissions.

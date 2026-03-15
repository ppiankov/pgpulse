# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- PostgreSQL metrics exporter with Prometheus endpoint
- Collectors for `pg_stat_activity`, `pg_stat_statements`, and database sizes
- Auto-detection of PostgreSQL version for correct `pg_stat_statements` columns
- Configurable poll interval and slow query threshold via environment variables
- Health check endpoint at `/healthz`
- Grafana dashboard for import (`grafana/pgpulse-dashboard.json`)
- Docker support with multi-stage build

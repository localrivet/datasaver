# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

datasaver is a single-binary pure Go application for automated database backups. Supports PostgreSQL and SQLite. Runs as a sidecar container or standalone binary, handles backups, GFS rotation, and recovery.

## Build Commands

```bash
# Build binary
go build -o datasaver ./cmd/datasaver

# Run tests
go test ./...

# Build Docker image
docker build -t datasaver .

# Run with docker-compose
docker compose up -d
```

## Architecture

```
cmd/datasaver/main.go     # CLI entry point with cobra commands
internal/
  backup/
    engine.go            # Core backup logic (uses database driver)
    scheduler.go         # Cron scheduling
    validator.go         # Backup validation
    retry.go             # Retry logic for transient failures
  storage/
    interface.go         # Storage abstraction
    local.go             # Local filesystem backend
    s3.go                # S3-compatible storage (minio-go)
  rotation/
    policy.go            # Retention policies
    gfs.go               # Grandfather-Father-Son rotation
  restore/engine.go      # Restore operations
  config/config.go       # Configuration (env vars + YAML)
  metrics/prometheus.go  # Prometheus metrics
  notify/webhook.go      # Webhook notifications
pkg/
  database/
    interface.go         # Database driver interface
    postgres.go          # PostgreSQL driver (pg_dump/pg_restore)
    sqlite.go            # SQLite driver (pure Go, no CGO)
    factory.go           # Driver factory
  postgres/
    connection.go        # Legacy postgres client
    metadata.go          # Backup metadata JSON
```

## Key Patterns

- **Database Driver Interface**: `pkg/database.Driver` abstracts database operations
- **Storage Backend Interface**: All storage operations go through `storage.Backend` interface
- **Pure Go**: SQLite uses modernc.org/sqlite (no CGO required)
- **Configuration**: Environment variables take precedence over config file
- **Rotation**: GFS (Grandfather-Father-Son) - daily/weekly/monthly retention
- **CLI**: cobra-based commands (daemon, backup, list, restore, cleanup, health, verify)

## Environment Variables

Essential:
- `DATASAVER_DB_TYPE` - "postgres" (default) or "sqlite"
- `DATASAVER_DB_HOST`, `DATASAVER_DB_NAME`, `DATASAVER_DB_USER`, `DATASAVER_DB_PASSWORD` (PostgreSQL)
- `DATASAVER_DB_PATH` - Path to SQLite database file
- `DATASAVER_SCHEDULE` - Cron format (e.g., "0 2 * * *")
- `DATASAVER_STORAGE_BACKEND` - "local" or "s3"
- `DATASAVER_STORAGE_PATH` - Path for local storage

## Task Master AI Instructions
**Import Task Master's development workflow commands and guidelines, treat as if import is in the main CLAUDE.md file.**
@./.taskmaster/CLAUDE.md

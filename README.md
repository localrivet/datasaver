# datasaver

A single-binary pure Go application for automated database backups. Supports PostgreSQL and SQLite. Designed to run as a sidecar container or standalone binary, handling backups, rotation, and recovery automatically.

## Features

- **Multi-Database Support**: PostgreSQL and SQLite (pure Go, no CGO)
- **Automated Backups**: Cron-style scheduling
- **Intelligent Rotation**: Grandfather-Father-Son (GFS) retention policy
- **Multiple Storage Backends**: Local filesystem and S3-compatible storage
- **One-Command Restore**: Simple recovery from any backup
- **Monitoring**: Health endpoint, Prometheus metrics, webhook notifications
- **Compression**: gzip support (zstd planned)
- **Zero Dependencies**: Single binary, no external tools required for SQLite

## Quick Start

### Docker Compose

Add to your `compose.yml`:

```yaml
datasaver:
  image: ghcr.io/localrivet/datasaver:latest
  environment:
    DATASAVER_DB_HOST: postgres
    DATASAVER_DB_NAME: mydb
    DATASAVER_DB_USER: postgres
    DATASAVER_DB_PASSWORD: secret
    DATASAVER_SCHEDULE: "0 2 * * *"
    DATASAVER_STORAGE_BACKEND: local
    DATASAVER_STORAGE_PATH: /backups
    DATASAVER_KEEP_DAILY: 7
    DATASAVER_KEEP_WEEKLY: 4
    DATASAVER_KEEP_MONTHLY: 6
  volumes:
    - backup_data:/backups
  depends_on:
    - postgres
  restart: unless-stopped
```

### Binary

```bash
# Run as daemon
datasaver daemon --config config.yml

# Perform immediate backup
datasaver backup

# List backups
datasaver list

# Restore from backup
datasaver restore backup_20240111_0200

# Check health
datasaver health
```

## Configuration

### Environment Variables

```bash
# Database Type (postgres or sqlite)
DATASAVER_DB_TYPE=postgres         # postgres (default) or sqlite

# PostgreSQL Connection
DATASAVER_DB_HOST=postgres
DATASAVER_DB_PORT=5432
DATASAVER_DB_NAME=mydb
DATASAVER_DB_USER=postgres
DATASAVER_DB_PASSWORD=secret

# Or use connection string (PostgreSQL)
DATASAVER_DATABASE_URL=postgres://user:pass@host:5432/dbname

# SQLite Configuration
DATASAVER_DB_TYPE=sqlite
DATASAVER_DB_PATH=/data/myapp.db   # Path to SQLite database file

# Backup Schedule (cron format)
DATASAVER_SCHEDULE="0 2 * * *"

# Storage
DATASAVER_STORAGE_BACKEND=local    # local or s3
DATASAVER_STORAGE_PATH=/backups    # for local storage

# S3 Storage
DATASAVER_S3_BUCKET=my-backups
DATASAVER_S3_ENDPOINT=s3.amazonaws.com
DATASAVER_S3_REGION=us-east-1
DATASAVER_S3_ACCESS_KEY=xxx
DATASAVER_S3_SECRET_KEY=xxx
DATASAVER_S3_USE_SSL=true

# Retention Policy
DATASAVER_KEEP_DAILY=7
DATASAVER_KEEP_WEEKLY=4
DATASAVER_KEEP_MONTHLY=6
DATASAVER_MAX_AGE_DAYS=90

# Compression
DATASAVER_COMPRESSION=gzip         # gzip or none

# Monitoring
DATASAVER_METRICS_PORT=9090
DATASAVER_HEALTH_PORT=8080
DATASAVER_WEBHOOK_URL=https://hooks.example.com/backup
DATASAVER_ALERT_AFTER_HOURS=26
```

### Config File (YAML)

```yaml
database:
  url: "postgres://user:pass@postgres:5432/mydb"

schedule: "0 2 * * *"

storage:
  backend: s3
  s3:
    bucket: my-backups
    endpoint: s3.amazonaws.com
    region: us-east-1
    access_key: ${AWS_ACCESS_KEY}
    secret_key: ${AWS_SECRET_KEY}

retention:
  daily: 7
  weekly: 4
  monthly: 6
  max_age_days: 90

compression: gzip

monitoring:
  metrics_port: 9090
  health_port: 8080
  webhook_url: https://hooks.example.com/backup
  alert_after_hours: 26
```

## CLI Commands

### `datasaver daemon`

Run as scheduled backup daemon. Starts the scheduler, health endpoint, and metrics server.

```bash
datasaver daemon
datasaver daemon --config /etc/datasaver/config.yml
```

### `datasaver backup`

Perform an immediate backup.

```bash
datasaver backup
```

### `datasaver list`

List all available backups.

```bash
datasaver list
```

Output:

```
ID                        DATE                 SIZE         TYPE
backup_20240111_0200      2024-01-11 02:00     125.50 MB    daily
backup_20240110_0200      2024-01-10 02:00     124.80 MB    daily
```

### `datasaver restore <backup-id>`

Restore from a specific backup.

```bash
datasaver restore backup_20240111_0200

# Restore to different database
datasaver restore backup_20240111_0200 --target-db mydb_restored

# Dry run (test without applying)
datasaver restore backup_20240111_0200 --dry-run
```

### `datasaver cleanup`

Manually run the cleanup routine to delete old backups.

```bash
datasaver cleanup
```

### `datasaver health`

Check backup system health.

```bash
datasaver health
```

Output:

```
Status: healthy
Last backup: 2024-01-11 02:00:15
Total backups: 23
Storage used: 2.80 GB
```

### `datasaver verify <backup-id>`

Validate backup integrity.

```bash
datasaver verify backup_20240111_0200
```

## Monitoring

### Health Endpoint

`GET /health` returns the current backup status:

```
status: healthy
last_backup: 2024-01-11T02:00:15Z
next_backup: 2024-01-12T02:00:00Z
```

### Prometheus Metrics

Available at `/metrics`:

- `datasaver_backup_duration_seconds` - Backup duration histogram
- `datasaver_backup_size_bytes` - Last backup size
- `datasaver_backups_total` - Total backup attempts
- `datasaver_backup_failures_total` - Failed backups
- `datasaver_last_backup_timestamp` - Last backup time
- `datasaver_last_backup_success` - Last backup status (1=success, 0=failure)
- `datasaver_storage_used_bytes` - Total storage used

### Webhook Notifications

POST to configured URL on backup events:

```json
{
  "event": "backup.completed",
  "timestamp": "2024-01-11T02:00:15Z",
  "backup_id": "backup_20240111_0200",
  "status": "success",
  "message": "Backup backup_20240111_0200 completed successfully",
  "details": {
    "size_bytes": 131621888,
    "duration_ms": 12500
  }
}
```

## Retention Policy (GFS)

The Grandfather-Father-Son rotation keeps:

- **Daily**: Last N daily backups (default: 7)
- **Weekly**: First backup of each week (default: 4 weeks)
- **Monthly**: First backup of each month (default: 6 months)

Backups are automatically tagged:

- Every backup is "daily"
- Sunday backups are also "weekly"
- 1st of month backups are also "monthly"

## Building

```bash
# Build binary
go build -o datasaver ./cmd/datasaver

# Build Docker image
docker build -t datasaver .
```

## License

MIT

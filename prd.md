# PRD: PostgreSQL Backup System (datasaver)

## Problem Statement

When running Postgres in Docker containers in production, you need reliable automated backups that just work. Current solutions either require external services (expensive, vendor lock-in), complex orchestration (overkill), or manual scripting (brittle, no rotation logic). Need a single-binary Go application that runs as a sidecar container, handles backups, rotation, and recovery automatically.

## Target Users

- **Primary**: You - managing Postgres databases across 22+ companies
- **Secondary**: Anyone else running production Postgres in Docker who wants simple, reliable backups
- **Anti-target**: Large enterprises with dedicated DBA teams (they have complex requirements we won't serve)

## Why This Matters for Your Setup

**Current Pain Points**:

- Running production Postgres across multiple companies/projects
- Each needs reliable backups without babysitting
- Can't afford to lose data in any deployment
- Don't want to pay for managed backup services for 22+ databases
- Need to know immediately if backups stop working

**What This Solves**:

- **Deploy once, use everywhere**: Same Docker image across all projects
- **Zero maintenance**: Set schedule, it just runs
- **Cost effective**: Store backups wherever you want (local volumes, cheap S3)
- **Peace of mind**: Health checks + alerts mean you know if something breaks
- **Fast recovery**: Database dies? One command to restore from last good backup

**Deployment Reality**:

- Add 5 lines to any docker-compose.yml
- Point it at your Postgres container
- Set backup schedule and retention
- Done. Backups run automatically.

## Core Features

### 1. Automated Backup Execution

- **Schedule**: Cron-style scheduling (e.g., `0 2 * * *` for 2 AM daily)
- **Methods**:
  - Full dumps using `pg_dump` (primary)
  - WAL archiving for PITR (optional, v2)
- **Compression**: gzip by default, configurable (gzip/zstd/none)
- **Format**: Custom format (`pg_dump -Fc`) for flexibility and compression

### 2. Intelligent Rotation

- **Retention Policies**:
  - Keep last N backups (e.g., 7 daily)
  - Keep last N weekly backups (e.g., 4 weekly)
  - Keep last N monthly backups (e.g., 6 monthly)
  - Age-based deletion (e.g., delete after 90 days)
- **Strategy**: Grandfather-Father-Son (GFS) rotation
- **Automatic Cleanup**: Runs after each backup

### 3. Storage Backends

- **Local Filesystem** (v1): Mount a volume, write there
- **S3-Compatible** (v1): AWS S3, MinIO, Backblaze B2, etc.
- **Future**: GCS, Azure Blob (v2)

### 4. Recovery Operations

- **List Available Backups**: CLI command to see all backups with metadata
- **One-Command Restore**: `datasaver restore <backup-id>`
- **Point-in-Time Recovery**: Using WAL archives (v2)
- **Validation**: Test restore to temporary database (optional)

### 5. Monitoring & Alerting

- **Health Endpoint**: `/health` returns last backup status, next scheduled backup
- **Prometheus Metrics**: Backup duration, size, success/failure counts
- **Webhook Notifications**: POST to URL on success/failure
- **Alerting**: Configurable dead man's switch (alert if no backup in X hours)

## Technical Architecture

### Component Design

```
┌─────────────────────────────────────────┐
│         Docker Compose Stack            │
│                                         │
│  ┌──────────┐         ┌──────────────┐ │
│  │          │         │              │ │
│  │ Postgres │◄────────│  datasaver    │ │
│  │          │ pg_dump │  (Go binary) │ │
│  │          │         │              │ │
│  └──────────┘         └──────┬───────┘ │
│                              │         │
│                              ▼         │
│                       ┌──────────────┐ │
│                       │ Volume/S3    │ │
│                       │ Storage      │ │
│                       └──────────────┘ │
└─────────────────────────────────────────┘
```

### Go Application Structure

```
datasaver/
├── cmd/
│   └── datasaver/
│       └── main.go           # CLI entry point
├── internal/
│   ├── backup/
│   │   ├── engine.go         # Core backup logic
│   │   ├── scheduler.go      # Cron scheduling
│   │   └── validator.go      # Backup validation
│   ├── storage/
│   │   ├── interface.go      # Storage abstraction
│   │   ├── local.go          # Local filesystem
│   │   └── s3.go             # S3-compatible storage
│   ├── rotation/
│   │   ├── policy.go         # Retention policies
│   │   └── gfs.go            # GFS rotation logic
│   ├── restore/
│   │   ├── engine.go         # Restore operations
│   │   └── pitr.go           # Point-in-time recovery
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── metrics/
│   │   └── prometheus.go     # Metrics collection
│   └── notify/
│       └── webhook.go        # Notification handling
├── pkg/
│   └── postgres/
│       ├── connection.go     # Postgres client
│       └── metadata.go       # Backup metadata
├── Dockerfile
├── docker-compose.yml        # Example deployment
└── README.md
```

### Configuration

**Environment Variables** (primary method):

```bash
# Database Connection
DATASAVER_DB_HOST=postgres
DATASAVER_DB_PORT=5432
DATASAVER_DB_NAME=mydb
DATASAVER_DB_USER=postgres
DATASAVER_DB_PASSWORD=secret

# Or use connection string
DATASAVER_DATABASE_URL=postgres://user:pass@host:5432/dbname

# Backup Schedule
DATASAVER_SCHEDULE="0 2 * * *"  # 2 AM daily

# Storage
DATASAVER_STORAGE_BACKEND=s3    # local|s3
DATASAVER_STORAGE_PATH=/backups # for local
DATASAVER_S3_BUCKET=my-backups
DATASAVER_S3_ENDPOINT=s3.amazonaws.com
DATASAVER_S3_REGION=us-east-1
DATASAVER_S3_ACCESS_KEY=xxx
DATASAVER_S3_SECRET_KEY=xxx

# Retention
DATASAVER_KEEP_DAILY=7
DATASAVER_KEEP_WEEKLY=4
DATASAVER_KEEP_MONTHLY=6
DATASAVER_MAX_AGE_DAYS=90

# Compression
DATASAVER_COMPRESSION=gzip      # gzip|zstd|none

# Monitoring
DATASAVER_METRICS_PORT=9090
DATASAVER_WEBHOOK_URL=https://hooks.example.com/backup
DATASAVER_ALERT_AFTER_HOURS=26  # Alert if no backup in 26 hours
```

**Config File** (optional, YAML):

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
  webhook_url: https://hooks.example.com/backup
  alert_after_hours: 26
```

### Docker Compose Integration

```yaml
version: "3.8"

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: mydb
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres_data:/var/lib/postgresql/data
    networks:
      - app_network

  datasaver:
    image: ghcr.io/yourusername/datasaver:latest
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
    networks:
      - app_network
    restart: unless-stopped

volumes:
  postgres_data:
  backup_data:

networks:
  app_network:
```

## CLI Interface

### Daemon Mode (Primary)

```bash
# Run as scheduled backup daemon
datasaver daemon

# Run with config file
datasaver daemon --config /etc/datasaver/config.yml
```

### One-Time Operations

```bash
# Perform immediate backup
datasaver backup

# List available backups
datasaver list
# Output:
# ID                    DATE                SIZE      TYPE
# backup_20240111_0200  2024-01-11 02:00   125.5 MB  daily
# backup_20240110_0200  2024-01-10 02:00   124.8 MB  daily

# Restore from backup
datasaver restore backup_20240111_0200

# Restore to different database
datasaver restore backup_20240111_0200 --target-db mydb_restored

# Test restore without applying
datasaver restore backup_20240111_0200 --dry-run

# Clean up old backups manually
datasaver cleanup

# Validate backup integrity
datasaver verify backup_20240111_0200
```

### Health Check

```bash
# Check backup system health
datasaver health
# Output:
# Status: healthy
# Last backup: 2024-01-11 02:00:15 (success)
# Next backup: 2024-01-12 02:00:00
# Total backups: 23
# Storage used: 2.8 GB
```

## Backup Metadata Format

Each backup includes a metadata JSON file:

```json
{
  "id": "backup_20240111_020015",
  "timestamp": "2024-01-11T02:00:15Z",
  "type": "daily",
  "database": {
    "name": "mydb",
    "host": "postgres",
    "version": "16.1"
  },
  "backup": {
    "method": "pg_dump",
    "format": "custom",
    "compression": "gzip",
    "size_bytes": 131621888,
    "compressed_size_bytes": 45678912,
    "duration_seconds": 12.5,
    "checksum": "sha256:abcd1234..."
  },
  "files": [
    "backup_20240111_020015.dump.gz",
    "backup_20240111_020015.meta.json"
  ],
  "retention": {
    "keep_until": "2024-02-11T02:00:15Z",
    "policy": "daily"
  }
}
```

## Rotation Logic (GFS)

### Backup Type Classification

- **Daily**: Backups taken every day
- **Weekly**: First successful backup of the week (Sunday)
- **Monthly**: First successful backup of the month

### Retention Example

With settings: daily=7, weekly=4, monthly=6

Day 1: Create backup → Tag as "daily"
Day 7 (Sunday): Create backup → Tag as "daily" + "weekly"
Day 30: Create backup → Tag as "daily" + "weekly" + "monthly"

After Day 8:

- Delete Day 1 backup (exceeded 7 daily retention)
- Keep Day 7 backup (still within weekly retention)

After Day 35 (5 weeks):

- Keep last 7 daily backups
- Keep last 4 weekly backups (weeks 2, 3, 4, 5)
- Keep monthly backup from day 30

### Cleanup Algorithm

```
1. Scan all backups
2. For each backup type (daily/weekly/monthly):
   - Sort by date descending
   - Keep first N according to retention policy
   - Mark others for deletion if not protected by another policy
3. Apply max_age_days filter
4. Delete marked backups
```

## Performance Considerations

### Target Database Sizes

This is optimized for typical SaaS/startup databases:

- **Sweet spot**: 100MB - 50GB databases
- **Works fine**: Up to 100GB
- **Gets slow**: 100GB+ (but still works, just takes longer)

Most of your production databases likely fall in the 1-20GB range, which means:

- Backups complete in 1-5 minutes
- Minimal impact on database performance
- Storage costs are negligible

### Backup Duration

- Small databases (<1GB): ~5-30 seconds
- Medium databases (1-10GB): ~1-5 minutes
- Large databases (10-100GB): ~5-30 minutes
- Use `--jobs` flag for parallel dump (v2)

### Resource Usage

- **CPU**: Minimal during backup (mostly I/O bound)
- **Memory**: ~50-200MB baseline, scales with database size
- **Disk I/O**: High during backup creation
- **Network**: High if using S3 storage

### Optimization

- Run backups during low-traffic periods (2-4 AM default)
- Use zstd compression for faster compression with good ratios
- For large databases, consider WAL archiving + periodic full dumps

## Error Handling & Recovery

### Backup Failures

- **Database Connection Failed**: Retry with exponential backoff (3 attempts)
- **Disk Full**: Alert immediately, attempt to delete oldest backup, retry
- **Upload Failed**: Keep local copy, retry upload, alert if multiple failures
- **Corruption Detected**: Alert, mark backup as invalid, retry

### Recovery Failures

- **Backup Not Found**: List available backups, error clearly
- **Corrupted Backup**: Verify checksum, suggest next-oldest backup
- **Database Locked**: Provide option to force restore (drop connections)

### Health Monitoring

- Track last successful backup timestamp
- Alert if backup hasn't run within `alert_after_hours` window
- Monitor backup size trends (alert on >50% deviation)
- Track failed backup attempts

## Security Considerations

### Credentials Management

- Support environment variables (primary)
- Support Docker secrets mounting
- Never log credentials or connection strings
- Use read-only database user for backups when possible

### Backup Encryption

- S3 server-side encryption (SSE-S3, SSE-KMS)
- Client-side encryption option (v2)
- Encrypt metadata files containing sensitive info

### Access Control

- Restrict container to minimum required permissions
- Read-only access to database (except for restore operations)
- Write access only to backup storage path

## Success Metrics

### Must Have (v1)

- ✅ Automated backups run on schedule
- ✅ GFS rotation works correctly
- ✅ One-command restore succeeds
- ✅ S3 and local storage both work
- ✅ Health endpoint returns accurate status
- ✅ Zero data loss in normal operation

### Nice to Have (v2)

- ✅ WAL archiving for PITR
- ✅ Parallel dumps for large databases
- ✅ Client-side encryption
- ✅ Backup validation/testing
- ✅ GCS and Azure Blob storage

## Launch Checklist

**Week 1-2: Core Implementation**

- [ ] Basic Go project structure
- [ ] Database connection and pg_dump execution
- [ ] Local filesystem storage
- [ ] Simple scheduling (cron)
- [ ] Basic CLI (backup, list, restore)

**Week 3: Storage & Rotation**

- [ ] S3 storage backend
- [ ] GFS rotation logic
- [ ] Metadata file generation
- [ ] Cleanup algorithm

**Week 4: Monitoring & Polish**

- [ ] Health endpoint
- [ ] Prometheus metrics
- [ ] Webhook notifications
- [ ] Error handling and logging
- [ ] Docker image and compose example

**Week 5: Testing & Documentation**

- [ ] Integration tests with real Postgres
- [ ] Restore testing
- [ ] Comprehensive README
- [ ] Example configurations
- [ ] Initial release (v0.1.0)

## Open Questions

1. **Parallel Dumps**: Should v1 support `--jobs` flag for parallel dumps, or defer to v2?

   - **Recommendation**: Defer to v2, keep v1 simple

2. **Backup Validation**: Should we automatically test-restore backups periodically?

   - **Recommendation**: Optional feature, disabled by default (requires extra resources)

3. **Multi-Database Support**: Support backing up multiple databases in one instance?

   - **Recommendation**: No, one instance per database keeps it simple. Use multiple containers.

4. **Custom Hooks**: Pre/post backup scripts?

   - **Recommendation**: v2 feature, not needed initially

5. **Incremental Backups**: Worth the complexity?
   - **Recommendation**: No, WAL archiving handles this better for PITR scenarios

## Competitive Analysis

### What You're Replacing

- **Cron + pg_dump scripts**: Works but brittle, no rotation logic, no monitoring
- **Manual backups**: When you remember to do them
- **Managed service backups** (AWS RDS): Vendor lock-in, expensive, not an option for self-hosted

### Why This Approach Wins

- **Set and forget**: Runs automatically, handles rotation
- **Single binary**: No dependencies, easy to deploy across all your projects
- **GFS rotation built-in**: Smart retention without thinking about it
- **Monitoring native**: Know immediately if backups stop working
- **Recovery is trivial**: One command to restore

### Alternatives Considered

- **pgBackRest**: Too complex, steep learning curve, overkill for your needs
- **WAL-G**: Great for massive databases, way too heavyweight
- **Postgres native backup tools**: Still need scripting for rotation/monitoring

## Deployment Approach

- **Internal tool first**: Built for your production needs
- **Open source potential**: MIT License if you decide to share it
- **Docker image**: Build once, deploy everywhere across your 22 companies
- **No external dependencies**: Self-contained binary

## Usage Pattern

Add to any project's docker-compose.yml:

```yaml
datasaver:
  image: your-registry/datasaver:latest
  environment:
    DATASAVER_DB_HOST: postgres
    DATASAVER_SCHEDULE: "0 2 * * *"
    DATASAVER_S3_BUCKET: ${PROJECT_NAME}-backups
  volumes:
    - /mnt/backups:/backups # or pure S3
```

That's it. Backups run automatically, rotation handled, monitoring available.

## Future Enhancements (When Needed)

### Phase 2: Advanced Features

- WAL archiving and PITR (if you need point-in-time recovery)
- Parallel dumps (`--jobs` support for large databases)
- Client-side encryption
- Backup validation/testing
- GCS and Azure Blob storage

### Phase 3: Nice to Have

- Multi-database support per instance (backup all DBs in a project)
- Pre/post backup hooks
- Advanced alerting (Slack, Discord, PagerDuty)
- Backup analytics/trends
- Incremental backup support

### Deployment Considerations

- Could open source if it proves useful to others
- Keep it simple and focused on the core use case

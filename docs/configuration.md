# Configuration

datasaver can be configured via environment variables or a YAML config file.

## Environment Variables

### Database Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DATASAVER_DB_TYPE` | Database type: `postgres` or `sqlite` | `postgres` |
| `DATASAVER_DB_HOST` | PostgreSQL host | `localhost` |
| `DATASAVER_DB_PORT` | PostgreSQL port | `5432` |
| `DATASAVER_DB_NAME` | Database name | - |
| `DATASAVER_DB_USER` | Database user | - |
| `DATASAVER_DB_PASSWORD` | Database password | - |
| `DATASAVER_DB_PATH` | SQLite database file path | - |

### Storage Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DATASAVER_STORAGE_BACKEND` | Storage backend: `local` or `s3` | `local` |
| `DATASAVER_STORAGE_PATH` | Local storage path | `./backups` |
| `DATASAVER_S3_BUCKET` | S3 bucket name | - |
| `DATASAVER_S3_ENDPOINT` | S3 endpoint (for MinIO) | - |
| `DATASAVER_S3_REGION` | S3 region | `us-east-1` |
| `DATASAVER_S3_ACCESS_KEY` | S3 access key | - |
| `DATASAVER_S3_SECRET_KEY` | S3 secret key | - |

### Backup Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DATASAVER_SCHEDULE` | Cron schedule for backups | `0 2 * * *` |
| `DATASAVER_VERIFY_BACKUP` | Verify backup after creation | `false` |
| `DATASAVER_VERIFY_CHECKSUM` | Verify checksum on restore | `false` |

### Retention Policy (GFS)

| Variable | Description | Default |
|----------|-------------|---------|
| `DATASAVER_RETAIN_DAILY` | Daily backups to keep | `7` |
| `DATASAVER_RETAIN_WEEKLY` | Weekly backups to keep | `4` |
| `DATASAVER_RETAIN_MONTHLY` | Monthly backups to keep | `12` |

### Monitoring

| Variable | Description | Default |
|----------|-------------|---------|
| `DATASAVER_HEALTH_PORT` | Health check endpoint port | `8080` |
| `DATASAVER_METRICS_PORT` | Prometheus metrics port | `9090` |
| `DATASAVER_WEBHOOK_URL` | Webhook URL for notifications | - |
| `DATASAVER_ALERT_AFTER_HOURS` | Alert if no backup in N hours | `26` |

### MCP (Model Context Protocol)

| Variable | Description | Default |
|----------|-------------|---------|
| `DATASAVER_MCP_API_KEY` | API key for MCP endpoint | - |
| `DATASAVER_BASE_URL` | External URL for OAuth discovery | - |

## YAML Configuration

Create a `config.yaml` file:

```yaml
database:
  type: postgres
  host: localhost
  port: 5432
  name: myapp
  user: backup_user
  password: secret

storage:
  backend: s3
  path: ./backups
  s3:
    bucket: my-backups
    endpoint: s3.amazonaws.com
    region: us-west-2
    access_key: AKIAIOSFODNN7EXAMPLE
    secret_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

schedule: "0 */6 * * *"  # Every 6 hours

retention:
  daily: 7
  weekly: 4
  monthly: 12

backup:
  verify_after_backup: true
  verify_checksum: true

monitoring:
  health_port: 8080
  metrics_port: 9090
  webhook_url: https://hooks.slack.com/services/...
  alert_after_hours: 26
```

Run with config file:

```bash
datasaver daemon -c /path/to/config.yaml
```

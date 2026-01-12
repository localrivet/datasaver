# MCP Integration

datasaver exposes an MCP (Model Context Protocol) endpoint that allows AI assistants like Claude to manage backups programmatically.

## Setup

1. Set an API key:

```bash
export DATASAVER_MCP_API_KEY="your-secure-api-key"
```

2. Optionally set the base URL for OAuth discovery:

```bash
export DATASAVER_BASE_URL="https://your-domain.com"
```

3. Start the daemon:

```bash
datasaver daemon
```

The MCP endpoint will be available at `http://localhost:8080/mcp`.

## OAuth Discovery

MCP clients discover authentication via standard OAuth endpoints:

- `GET /.well-known/oauth-protected-resource` - Resource metadata
- `GET /.well-known/oauth-authorization-server` - Auth server metadata

## Authentication

All MCP requests require Bearer token authentication:

```
Authorization: Bearer <your-api-key>
```

## Available Tools

### backup_now

Trigger an immediate backup.

```json
{
  "name": "backup_now",
  "arguments": {}
}
```

### list_backups

List all available backups.

```json
{
  "name": "list_backups",
  "arguments": {
    "limit": 10
  }
}
```

### restore_backup

Restore from a specific backup.

```json
{
  "name": "restore_backup",
  "arguments": {
    "backup_id": "20240115-020000",
    "target_db": "myapp_restored",
    "dry_run": true
  }
}
```

### verify_backup

Verify backup integrity.

```json
{
  "name": "verify_backup",
  "arguments": {
    "backup_id": "20240115-020000"
  }
}
```

### get_status

Get current backup system status.

```json
{
  "name": "get_status",
  "arguments": {}
}
```

## Claude Desktop Configuration

Add to your Claude Desktop `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "datasaver": {
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer your-api-key"
      }
    }
  }
}
```

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/localrivet/datasaver/internal/backup"
	"github.com/localrivet/datasaver/internal/restore"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input/Output types for tools

type EmptyInput struct{}

type BackupNowOutput struct {
	BackupID       string `json:"backup_id"`
	Timestamp      string `json:"timestamp"`
	SizeBytes      int64  `json:"size_bytes"`
	CompressedSize int64  `json:"compressed_size"`
	DurationMs     int64  `json:"duration_ms"`
	Checksum       string `json:"checksum"`
}

type ListBackupsInput struct {
	Limit int `json:"limit" jsonschema:"Maximum number of backups to return (default: 20)"`
}

type BackupItem struct {
	ID             string `json:"id"`
	Timestamp      string `json:"timestamp"`
	Database       string `json:"database"`
	SizeBytes      int64  `json:"size_bytes"`
	CompressedSize int64  `json:"compressed_size"`
	Type           string `json:"type"`
	Checksum       string `json:"checksum"`
}

type ListBackupsOutput struct {
	Count   int          `json:"count"`
	Backups []BackupItem `json:"backups"`
}

type GetBackupInput struct {
	BackupID string `json:"backup_id" jsonschema:"The backup ID to get details for"`
}

type GetBackupOutput struct {
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
	Type      string                 `json:"type"`
	Database  map[string]interface{} `json:"database"`
	Backup    map[string]interface{} `json:"backup"`
	Files     []string               `json:"files"`
	Retention map[string]interface{} `json:"retention"`
}

type RestoreBackupInput struct {
	BackupID string `json:"backup_id" jsonschema:"The backup ID to restore from"`
	TargetDB string `json:"target_db,omitempty" jsonschema:"Optional: restore to a different database name"`
	DryRun   bool   `json:"dry_run,omitempty" jsonschema:"If true, validate the restore without applying changes"`
}

type RestoreBackupOutput struct {
	BackupID string `json:"backup_id"`
	TargetDB string `json:"target_db"`
	Success  bool   `json:"success"`
	DryRun   bool   `json:"dry_run"`
}

type BackupStatusOutput struct {
	Status       string `json:"status"`
	TotalBackups int    `json:"total_backups"`
	StorageBytes int64  `json:"storage_bytes"`
	LastBackup   string `json:"last_backup,omitempty"`
	LastRun      string `json:"last_run,omitempty"`
	LastError    string `json:"last_error,omitempty"`
}

type CleanupOutput struct {
	DeletedCount int    `json:"deleted_count"`
	Message      string `json:"message"`
}

type VerifyBackupInput struct {
	BackupID string `json:"backup_id" jsonschema:"The backup ID to verify"`
}

type VerifyBackupOutput struct {
	BackupID   string   `json:"backup_id"`
	Valid      bool     `json:"valid"`
	FileExists bool     `json:"file_exists"`
	SizeMatch  bool     `json:"size_match"`
	ChecksumOK bool     `json:"checksum_ok"`
	Errors     []string `json:"errors,omitempty"`
}

// RegisterBackupTools registers all backup-related tools with the MCP server.
func RegisterBackupTools(server *mcp.Server, toolCtx *ToolContext) {
	// backup_now - Trigger an immediate backup
	mcp.AddTool(server, &mcp.Tool{
		Name:        "backup_now",
		Description: "Trigger an immediate database backup",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, BackupNowOutput, error) {
		result, err := toolCtx.BackupEngine.Run(ctx)
		if err != nil {
			return nil, BackupNowOutput{}, err
		}

		return nil, BackupNowOutput{
			BackupID:       result.ID,
			Timestamp:      result.Timestamp.Format(time.RFC3339),
			SizeBytes:      result.Size,
			CompressedSize: result.CompressedSize,
			DurationMs:     result.Duration.Milliseconds(),
			Checksum:       result.Checksum,
		}, nil
	})

	// list_backups - List all available backups
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_backups",
		Description: "List all available database backups",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListBackupsInput) (*mcp.CallToolResult, ListBackupsOutput, error) {
		limit := input.Limit
		if limit <= 0 {
			limit = 20
		}

		backups, err := toolCtx.BackupEngine.ListBackups(ctx)
		if err != nil {
			return nil, ListBackupsOutput{}, err
		}

		// Sort by timestamp descending
		sort.Slice(backups, func(i, j int) bool {
			return backups[i].Timestamp.After(backups[j].Timestamp)
		})

		// Apply limit
		if len(backups) > limit {
			backups = backups[:limit]
		}

		// Convert to response format
		items := make([]BackupItem, len(backups))
		for i, b := range backups {
			items[i] = BackupItem{
				ID:             b.ID,
				Timestamp:      b.Timestamp.Format(time.RFC3339),
				Database:       b.Database.Name,
				SizeBytes:      b.Backup.SizeBytes,
				CompressedSize: b.Backup.CompressedSize,
				Type:           b.Type,
				Checksum:       b.Backup.Checksum,
			}
		}

		return nil, ListBackupsOutput{
			Count:   len(items),
			Backups: items,
		}, nil
	})

	// get_backup - Get details of a specific backup
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_backup",
		Description: "Get detailed information about a specific backup",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetBackupInput) (*mcp.CallToolResult, GetBackupOutput, error) {
		meta, err := toolCtx.BackupEngine.GetBackup(ctx, input.BackupID)
		if err != nil {
			return nil, GetBackupOutput{}, err
		}

		return nil, GetBackupOutput{
			ID:        meta.ID,
			Timestamp: meta.Timestamp.Format(time.RFC3339),
			Type:      meta.Type,
			Database: map[string]interface{}{
				"name":    meta.Database.Name,
				"host":    meta.Database.Host,
				"version": meta.Database.Version,
			},
			Backup: map[string]interface{}{
				"method":          meta.Backup.Method,
				"format":          meta.Backup.Format,
				"compression":     meta.Backup.Compression,
				"size_bytes":      meta.Backup.SizeBytes,
				"compressed_size": meta.Backup.CompressedSize,
				"duration_s":      meta.Backup.DurationSeconds,
				"checksum":        meta.Backup.Checksum,
			},
			Files: meta.Files,
			Retention: map[string]interface{}{
				"keep_until": meta.Retention.KeepUntil.Format(time.RFC3339),
				"policy":     meta.Retention.Policy,
			},
		}, nil
	})

	// restore_backup - Restore from a backup
	mcp.AddTool(server, &mcp.Tool{
		Name:        "restore_backup",
		Description: "Restore the database from a backup. Use with caution!",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input RestoreBackupInput) (*mcp.CallToolResult, RestoreBackupOutput, error) {
		result, err := toolCtx.RestoreEngine.Restore(ctx, restore.RestoreOptions{
			BackupID: input.BackupID,
			TargetDB: input.TargetDB,
			DryRun:   input.DryRun,
		})
		if err != nil {
			return nil, RestoreBackupOutput{}, err
		}

		return nil, RestoreBackupOutput{
			BackupID: result.BackupID,
			TargetDB: result.TargetDB,
			Success:  result.Success,
			DryRun:   input.DryRun,
		}, nil
	})

	// backup_status - Get current backup system status
	mcp.AddTool(server, &mcp.Tool{
		Name:        "backup_status",
		Description: "Get the current status of the backup system",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, BackupStatusOutput, error) {
		backups, err := toolCtx.BackupEngine.ListBackups(ctx)
		if err != nil {
			return nil, BackupStatusOutput{}, err
		}

		var totalSize int64
		var lastBackup time.Time
		for _, b := range backups {
			totalSize += b.Backup.CompressedSize
			if b.Timestamp.After(lastBackup) {
				lastBackup = b.Timestamp
			}
		}

		status := "healthy"
		if len(backups) == 0 {
			status = "warning: no backups found"
		} else if time.Since(lastBackup) > toolCtx.Config.AlertDuration() {
			status = "warning: backup overdue"
		}

		lastRun := toolCtx.BackupEngine.LastRun()
		lastErr := toolCtx.BackupEngine.LastError()

		output := BackupStatusOutput{
			Status:       status,
			TotalBackups: len(backups),
			StorageBytes: totalSize,
		}

		if !lastBackup.IsZero() {
			output.LastBackup = lastBackup.Format(time.RFC3339)
		}
		if !lastRun.IsZero() {
			output.LastRun = lastRun.Format(time.RFC3339)
		}
		if lastErr != nil {
			output.LastError = lastErr.Error()
		}

		return nil, output, nil
	})

	// cleanup_backups - Run backup cleanup based on retention policy
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cleanup_backups",
		Description: "Run backup cleanup to remove old backups based on retention policy",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, CleanupOutput, error) {
		count, err := toolCtx.BackupEngine.Cleanup(ctx)
		if err != nil {
			return nil, CleanupOutput{}, err
		}

		return nil, CleanupOutput{
			DeletedCount: count,
			Message:      fmt.Sprintf("Cleaned up %d old backups", count),
		}, nil
	})

	// verify_backup - Validate backup integrity
	mcp.AddTool(server, &mcp.Tool{
		Name:        "verify_backup",
		Description: "Validate the integrity of a specific backup",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input VerifyBackupInput) (*mcp.CallToolResult, VerifyBackupOutput, error) {
		meta, err := toolCtx.BackupEngine.GetBackup(ctx, input.BackupID)
		if err != nil {
			return nil, VerifyBackupOutput{}, err
		}

		validator := backup.NewValidator(toolCtx.Storage, toolCtx.Logger)
		result, err := validator.Validate(ctx, meta)
		if err != nil {
			return nil, VerifyBackupOutput{}, err
		}

		return nil, VerifyBackupOutput{
			BackupID:   input.BackupID,
			Valid:      result.Valid,
			FileExists: result.FileExists,
			SizeMatch:  result.SizeMatch,
			ChecksumOK: result.ChecksumOK,
			Errors:     result.Errors,
		}, nil
	})
}

// registerBackupToolsToRegistry registers tools to a registry for direct invocation.
func registerBackupToolsToRegistry(registry *ToolRegistry, toolCtx *ToolContext) {
	registry.Register("backup_now", func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		result, err := toolCtx.BackupEngine.Run(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"backup_id":       result.ID,
			"timestamp":       result.Timestamp.Format(time.RFC3339),
			"size_bytes":      result.Size,
			"compressed_size": result.CompressedSize,
			"duration_ms":     result.Duration.Milliseconds(),
		}, nil
	})

	registry.Register("list_backups", func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		backups, err := toolCtx.BackupEngine.ListBackups(ctx)
		if err != nil {
			return nil, err
		}
		return backups, nil
	})

	registry.Register("backup_status", func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		backups, _ := toolCtx.BackupEngine.ListBackups(ctx)
		var totalSize int64
		var lastBackup time.Time
		for _, b := range backups {
			totalSize += b.Backup.CompressedSize
			if b.Timestamp.After(lastBackup) {
				lastBackup = b.Timestamp
			}
		}
		return map[string]any{
			"total_backups": len(backups),
			"storage_bytes": totalSize,
			"last_backup":   lastBackup,
		}, nil
	})
}

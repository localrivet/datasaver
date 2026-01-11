package mcp

import (
	"context"
	"log/slog"

	"github.com/almatuck/datasaver/internal/backup"
	"github.com/almatuck/datasaver/internal/config"
	"github.com/almatuck/datasaver/internal/mcp/tools"
	"github.com/almatuck/datasaver/internal/notify"
	"github.com/almatuck/datasaver/internal/restore"
	"github.com/almatuck/datasaver/internal/storage"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates a new MCP server with all backup tools registered.
func NewServer(ctx context.Context, cfg *config.Config, store storage.Backend, notifier *notify.Notifier, logger *slog.Logger) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "datasaver",
		Version: "1.0.0",
	}, nil)

	toolCtx := &tools.ToolContext{
		Config:        cfg,
		Storage:       store,
		BackupEngine:  backup.NewEngine(cfg, store, notifier, logger),
		RestoreEngine: restore.NewEngine(cfg, store, logger),
		Logger:        logger,
	}

	// Register backup tools
	tools.RegisterBackupTools(server, toolCtx)

	return server
}

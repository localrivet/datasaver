package tools

import (
	"log/slog"

	"github.com/almatuck/datasaver/internal/backup"
	"github.com/almatuck/datasaver/internal/config"
	"github.com/almatuck/datasaver/internal/restore"
	"github.com/almatuck/datasaver/internal/storage"
)

// ToolContext carries context for all MCP tools.
type ToolContext struct {
	Config        *config.Config
	Storage       storage.Backend
	BackupEngine  *backup.Engine
	RestoreEngine *restore.Engine
	Logger        *slog.Logger
}

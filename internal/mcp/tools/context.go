package tools

import (
	"log/slog"

	"github.com/localrivet/datasaver/internal/backup"
	"github.com/localrivet/datasaver/internal/config"
	"github.com/localrivet/datasaver/internal/restore"
	"github.com/localrivet/datasaver/internal/storage"
)

// ToolContext carries context for all MCP tools.
type ToolContext struct {
	Config        *config.Config
	Storage       storage.Backend
	BackupEngine  *backup.Engine
	RestoreEngine *restore.Engine
	Logger        *slog.Logger
}

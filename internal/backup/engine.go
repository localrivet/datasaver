package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/localrivet/datasaver/internal/config"
	"github.com/localrivet/datasaver/internal/notify"
	"github.com/localrivet/datasaver/internal/rotation"
	"github.com/localrivet/datasaver/internal/storage"
	"github.com/localrivet/datasaver/pkg/database"
	"github.com/localrivet/datasaver/pkg/postgres"
)

type Engine struct {
	cfg       *config.Config
	storage   storage.Backend
	rotator   *rotation.GFSRotator
	notifier  *notify.Notifier
	logger    *slog.Logger
	lastRun   time.Time
	lastError error
}

func NewEngine(cfg *config.Config, store storage.Backend, notifier *notify.Notifier, logger *slog.Logger) *Engine {
	policy := rotation.NewPolicy(
		cfg.Retention.Daily,
		cfg.Retention.Weekly,
		cfg.Retention.Monthly,
		cfg.Retention.MaxAgeDays,
	)

	return &Engine{
		cfg:      cfg,
		storage:  store,
		rotator:  rotation.NewGFSRotator(policy),
		notifier: notifier,
		logger:   logger,
	}
}

type BackupResult struct {
	ID              string
	Timestamp       time.Time
	Size            int64
	CompressedSize  int64
	Duration        time.Duration
	Checksum        string
	Verified        bool   // True if backup was verified after creation
	VerifyError     error  // Non-nil if verification failed
	Error           error
}

func (e *Engine) Run(ctx context.Context) (*BackupResult, error) {
	startTime := time.Now()
	backupID := postgres.GenerateBackupID(startTime)

	e.logger.Info("starting backup", "id", backupID, "db_type", e.cfg.Database.Type)

	result := &BackupResult{
		ID:        backupID,
		Timestamp: startTime,
	}

	dbCfg := database.Config{
		Type:     e.cfg.Database.Type,
		Host:     e.cfg.Database.Host,
		Port:     e.cfg.Database.Port,
		Name:     e.cfg.Database.Name,
		User:     e.cfg.Database.User,
		Password: e.cfg.Database.Password,
		URL:      e.cfg.Database.URL,
		Path:     e.cfg.Database.Path,
	}

	driver, err := database.NewDriver(dbCfg)
	if err != nil {
		result.Error = fmt.Errorf("failed to create database driver: %w", err)
		e.handleBackupError(result)
		return result, result.Error
	}

	if err := driver.Connect(ctx); err != nil {
		result.Error = fmt.Errorf("failed to connect to database: %w", err)
		e.handleBackupError(result)
		return result, result.Error
	}
	defer driver.Close()

	dbVersion, err := driver.Version(ctx)
	if err != nil {
		e.logger.Warn("failed to get database version", "error", err)
		dbVersion = "unknown"
	}

	tmpDir, err := os.MkdirTemp("", "datasaver-*")
	if err != nil {
		result.Error = fmt.Errorf("failed to create temp directory: %w", err)
		e.handleBackupError(result)
		return result, result.Error
	}
	defer os.RemoveAll(tmpDir)

	var dumpFile string
	if e.cfg.IsSQLite() {
		dumpFile = filepath.Join(tmpDir, backupID+".sql")
	} else {
		dumpFile = filepath.Join(tmpDir, backupID+".dump")
	}

	dumpOutput, err := os.Create(dumpFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to create dump file: %w", err)
		e.handleBackupError(result)
		return result, result.Error
	}

	if err := driver.Dump(ctx, dumpOutput); err != nil {
		dumpOutput.Close()
		result.Error = fmt.Errorf("database dump failed: %w", err)
		e.handleBackupError(result)
		return result, result.Error
	}
	dumpOutput.Close()

	dumpInfo, err := os.Stat(dumpFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to stat dump file: %w", err)
		e.handleBackupError(result)
		return result, result.Error
	}
	result.Size = dumpInfo.Size()

	var finalFile string
	var finalSize int64

	switch e.cfg.Compression {
	case "gzip":
		compressedFile := dumpFile + ".gz"
		if err := compressGzip(dumpFile, compressedFile); err != nil {
			result.Error = fmt.Errorf("compression failed: %w", err)
			e.handleBackupError(result)
			return result, result.Error
		}
		finalFile = compressedFile
		info, _ := os.Stat(compressedFile)
		finalSize = info.Size()
	case "none":
		finalFile = dumpFile
		finalSize = result.Size
	default:
		finalFile = dumpFile
		finalSize = result.Size
	}

	result.CompressedSize = finalSize

	checksum, err := postgres.CalculateChecksum(finalFile)
	if err != nil {
		e.logger.Warn("failed to calculate checksum", "error", err)
	}
	result.Checksum = checksum

	f, err := os.Open(finalFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to open backup file: %w", err)
		e.handleBackupError(result)
		return result, result.Error
	}
	defer f.Close()

	storagePath := filepath.Base(finalFile)
	if err := e.storage.Write(ctx, storagePath, f); err != nil {
		result.Error = fmt.Errorf("failed to write backup to storage: %w", err)
		e.handleBackupError(result)
		return result, result.Error
	}

	dbName := e.cfg.Database.Name
	if dbName == "" {
		dbName = e.cfg.Database.Path
	}
	dbHost := e.cfg.Database.Host
	if e.cfg.IsSQLite() {
		dbHost = "local"
	}
	metadata := postgres.NewBackupMetadata(backupID, dbName, dbHost, dbVersion)
	metadata.Backup.Method = driver.Type()
	metadata.Backup.Compression = e.cfg.Compression

	result.Duration = time.Since(startTime)
	metadata.SetBackupInfo(result.Size, result.CompressedSize, result.Duration, result.Checksum)

	keepUntil, policy := e.rotator.GetRetentionInfo(startTime)
	metadata.SetRetention(keepUntil, policy)
	metadata.Type = policy
	metadata.AddFile(storagePath)

	metaJSON, err := metadata.ToJSON()
	if err != nil {
		e.logger.Warn("failed to serialize metadata", "error", err)
	} else {
		metaPath := backupID + ".meta.json"
		if err := e.storage.Write(ctx, metaPath, bytes.NewReader(metaJSON)); err != nil {
			e.logger.Warn("failed to write metadata", "error", err)
		}
		metadata.AddFile(metaPath)
	}

	// Verify backup if configured
	if e.cfg.Backup.VerifyAfterBackup {
		e.logger.Info("verifying backup integrity", "id", backupID)
		validator := NewValidatorWithDBType(e.storage, e.logger, e.cfg.Database.Type)
		if err := validator.VerifyRestoreIntegrity(ctx, metadata); err != nil {
			result.VerifyError = err
			e.logger.Error("backup verification FAILED", "id", backupID, "error", err)
			// This is critical - the backup may be corrupted
			if e.notifier != nil {
				e.notifier.NotifyFailure(backupID, fmt.Errorf("backup verification failed: %w", err))
			}
		} else {
			result.Verified = true
			e.logger.Info("backup verified successfully", "id", backupID)
		}
	}

	e.lastRun = startTime
	e.lastError = nil

	e.logger.Info("backup completed",
		"id", backupID,
		"size", result.Size,
		"compressed_size", result.CompressedSize,
		"duration", result.Duration,
		"type", metadata.Type,
		"verified", result.Verified,
	)

	if e.notifier != nil {
		e.notifier.NotifySuccess(backupID, result.Size, result.Duration)
	}

	return result, nil
}

func (e *Engine) Cleanup(ctx context.Context) (int, error) {
	e.logger.Info("running backup cleanup")

	backups, err := e.ListBackups(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list backups: %w", err)
	}

	toDelete := e.rotator.DetermineBackupsToDelete(backups)

	deletedCount := 0
	for _, backup := range toDelete {
		e.logger.Info("deleting old backup", "id", backup.ID)

		for _, file := range backup.Files {
			if err := e.storage.Delete(ctx, file); err != nil {
				e.logger.Warn("failed to delete backup file", "file", file, "error", err)
			}
		}
		deletedCount++
	}

	e.logger.Info("cleanup completed", "deleted", deletedCount)

	return deletedCount, nil
}

func (e *Engine) ListBackups(ctx context.Context) ([]*postgres.BackupMetadata, error) {
	files, err := e.storage.List(ctx, "")
	if err != nil {
		return nil, err
	}

	var backups []*postgres.BackupMetadata

	for _, file := range files {
		if !strings.HasSuffix(file.Path, ".meta.json") {
			continue
		}

		reader, err := e.storage.Read(ctx, file.Path)
		if err != nil {
			e.logger.Warn("failed to read metadata file", "path", file.Path, "error", err)
			continue
		}

		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			e.logger.Warn("failed to read metadata content", "path", file.Path, "error", err)
			continue
		}

		meta, err := postgres.ParseMetadata(data)
		if err != nil {
			e.logger.Warn("failed to parse metadata", "path", file.Path, "error", err)
			continue
		}

		backups = append(backups, meta)
	}

	return backups, nil
}

func (e *Engine) GetBackup(ctx context.Context, backupID string) (*postgres.BackupMetadata, error) {
	metaPath := backupID + ".meta.json"

	reader, err := e.storage.Read(ctx, metaPath)
	if err != nil {
		return nil, fmt.Errorf("backup not found: %s", backupID)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	return postgres.ParseMetadata(data)
}

func (e *Engine) LastRun() time.Time {
	return e.lastRun
}

func (e *Engine) LastError() error {
	return e.lastError
}

func (e *Engine) handleBackupError(result *BackupResult) {
	e.lastError = result.Error
	e.logger.Error("backup failed", "id", result.ID, "error", result.Error)

	if e.notifier != nil {
		e.notifier.NotifyFailure(result.ID, result.Error)
	}
}

func compressGzip(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	_, err = io.Copy(gw, in)
	return err
}

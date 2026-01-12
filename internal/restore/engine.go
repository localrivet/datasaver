package restore

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/almatuck/datasaver/internal/config"
	"github.com/almatuck/datasaver/internal/storage"
	"github.com/almatuck/datasaver/pkg/postgres"
)

type Engine struct {
	cfg     *config.Config
	storage storage.Backend
	logger  *slog.Logger
}

func NewEngine(cfg *config.Config, store storage.Backend, logger *slog.Logger) *Engine {
	return &Engine{
		cfg:     cfg,
		storage: store,
		logger:  logger,
	}
}

type RestoreOptions struct {
	BackupID       string
	TargetDB       string
	DryRun         bool
	Force          bool
	VerifyChecksum bool // Verify checksum before restoring
}

type RestoreResult struct {
	BackupID       string
	TargetDB       string
	Success        bool
	ChecksumValid  bool
	Error          error
}

func (e *Engine) Restore(ctx context.Context, opts RestoreOptions) (*RestoreResult, error) {
	result := &RestoreResult{
		BackupID: opts.BackupID,
		TargetDB: opts.TargetDB,
	}

	e.logger.Info("starting restore", "backup_id", opts.BackupID, "target_db", opts.TargetDB)

	metaPath := opts.BackupID + ".meta.json"
	metaReader, err := e.storage.Read(ctx, metaPath)
	if err != nil {
		result.Error = fmt.Errorf("backup not found: %s", opts.BackupID)
		return result, result.Error
	}

	metaData, err := io.ReadAll(metaReader)
	metaReader.Close()
	if err != nil {
		result.Error = fmt.Errorf("failed to read metadata: %w", err)
		return result, result.Error
	}

	metadata, err := postgres.ParseMetadata(metaData)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse metadata: %w", err)
		return result, result.Error
	}

	var backupFile string
	for _, f := range metadata.Files {
		if !strings.HasSuffix(f, ".meta.json") {
			backupFile = f
			break
		}
	}

	if backupFile == "" {
		result.Error = fmt.Errorf("no backup file found in metadata")
		return result, result.Error
	}

	if opts.DryRun {
		e.logger.Info("dry run: would restore from", "file", backupFile)
		result.Success = true
		return result, nil
	}

	tmpDir, err := os.MkdirTemp("", "datasaver-restore-*")
	if err != nil {
		result.Error = fmt.Errorf("failed to create temp directory: %w", err)
		return result, result.Error
	}
	defer os.RemoveAll(tmpDir)

	reader, err := e.storage.Read(ctx, backupFile)
	if err != nil {
		result.Error = fmt.Errorf("failed to read backup file: %w", err)
		return result, result.Error
	}
	defer reader.Close()

	localPath := filepath.Join(tmpDir, backupFile)

	var finalReader io.Reader = reader

	if strings.HasSuffix(backupFile, ".gz") {
		gzReader, err := gzip.NewReader(reader)
		if err != nil {
			result.Error = fmt.Errorf("failed to create gzip reader: %w", err)
			return result, result.Error
		}
		defer gzReader.Close()
		finalReader = gzReader
		localPath = strings.TrimSuffix(localPath, ".gz")
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to create local file: %w", err)
		return result, result.Error
	}

	if _, err := io.Copy(localFile, finalReader); err != nil {
		localFile.Close()
		result.Error = fmt.Errorf("failed to write local file: %w", err)
		return result, result.Error
	}
	localFile.Close()

	// Verify checksum before restoring if enabled or configured
	if opts.VerifyChecksum || e.cfg.Backup.VerifyChecksum {
		if metadata.Backup.Checksum != "" {
			e.logger.Info("verifying backup checksum", "expected", metadata.Backup.Checksum)

			// For compressed files, we need to verify the original compressed file checksum
			// So we read it again and calculate
			compressedPath := filepath.Join(tmpDir, backupFile)
			compReader, err := e.storage.Read(ctx, backupFile)
			if err != nil {
				result.Error = fmt.Errorf("failed to read backup for checksum: %w", err)
				return result, result.Error
			}

			compFile, err := os.Create(compressedPath)
			if err != nil {
				compReader.Close()
				result.Error = fmt.Errorf("failed to create temp file for checksum: %w", err)
				return result, result.Error
			}

			if _, err := io.Copy(compFile, compReader); err != nil {
				compReader.Close()
				compFile.Close()
				result.Error = fmt.Errorf("failed to write temp file for checksum: %w", err)
				return result, result.Error
			}
			compReader.Close()
			compFile.Close()

			actualChecksum, err := postgres.CalculateChecksum(compressedPath)
			if err != nil {
				result.Error = fmt.Errorf("failed to calculate checksum: %w", err)
				return result, result.Error
			}

			if actualChecksum != metadata.Backup.Checksum {
				result.Error = fmt.Errorf("checksum mismatch: expected %s, got %s - backup may be corrupted",
					metadata.Backup.Checksum, actualChecksum)
				e.logger.Error("CRITICAL: checksum verification failed",
					"expected", metadata.Backup.Checksum,
					"actual", actualChecksum)
				return result, result.Error
			}

			result.ChecksumValid = true
			e.logger.Info("checksum verified successfully")
		} else {
			e.logger.Warn("no checksum available in backup metadata, skipping verification")
		}
	}

	targetDB := opts.TargetDB
	if targetDB == "" {
		targetDB = metadata.Database.Name
	}

	host, port, _, user, password := e.parseConnectionInfo()

	restoreOpts := postgres.DumpOptions{
		Database: targetDB,
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
	}

	if err := postgres.Restore(ctx, localPath, restoreOpts); err != nil {
		result.Error = fmt.Errorf("pg_restore failed: %w", err)
		return result, result.Error
	}

	result.Success = true
	result.TargetDB = targetDB

	e.logger.Info("restore completed",
		"backup_id", opts.BackupID,
		"target_db", targetDB,
	)

	return result, nil
}

func (e *Engine) parseConnectionInfo() (host string, port int, dbName, user, password string) {
	if e.cfg.Database.URL != "" {
		u, err := url.Parse(e.cfg.Database.URL)
		if err == nil {
			host = u.Hostname()
			port, _ = strconv.Atoi(u.Port())
			if port == 0 {
				port = 5432
			}
			dbName = strings.TrimPrefix(u.Path, "/")
			user = u.User.Username()
			password, _ = u.User.Password()
			return
		}
	}

	host = e.cfg.Database.Host
	port = e.cfg.Database.Port
	dbName = e.cfg.Database.Name
	user = e.cfg.Database.User
	password = e.cfg.Database.Password
	return
}

package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/localrivet/datasaver/internal/storage"
	"github.com/localrivet/datasaver/pkg/postgres"
	_ "modernc.org/sqlite"
)

type Validator struct {
	storage storage.Backend
	logger  *slog.Logger
	dbType  string
}

func NewValidator(store storage.Backend, logger *slog.Logger) *Validator {
	return &Validator{
		storage: store,
		logger:  logger,
		dbType:  "postgres", // default
	}
}

func NewValidatorWithDBType(store storage.Backend, logger *slog.Logger, dbType string) *Validator {
	return &Validator{
		storage: store,
		logger:  logger,
		dbType:  dbType,
	}
}

type ValidationResult struct {
	BackupID     string
	Valid        bool
	FileExists   bool
	SizeMatch    bool
	ChecksumOK   bool
	Errors       []string
}

func (v *Validator) Validate(ctx context.Context, metadata *postgres.BackupMetadata) (*ValidationResult, error) {
	result := &ValidationResult{
		BackupID: metadata.ID,
		Valid:    true,
	}

	if len(metadata.Files) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "no files listed in metadata")
		return result, nil
	}

	backupFile := ""
	for _, f := range metadata.Files {
		if f != metadata.ID+".meta.json" {
			backupFile = f
			break
		}
	}

	if backupFile == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "backup file not found in metadata")
		return result, nil
	}

	exists, err := v.storage.Exists(ctx, backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to check file existence: %w", err)
	}
	result.FileExists = exists

	if !exists {
		result.Valid = false
		result.Errors = append(result.Errors, "backup file does not exist")
		return result, nil
	}

	size, err := v.storage.Size(ctx, backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get file size: %w", err)
	}

	result.SizeMatch = size == metadata.Backup.CompressedSize
	if !result.SizeMatch {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf(
			"size mismatch: expected %d, got %d",
			metadata.Backup.CompressedSize, size,
		))
	}

	if metadata.Backup.Checksum != "" {
		reader, err := v.storage.Read(ctx, backupFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read backup file: %w", err)
		}

		tmpFile, err := createTempFile(reader)
		reader.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		defer tmpFile.cleanup()

		checksum, err := postgres.CalculateChecksum(tmpFile.path)
		if err != nil {
			v.logger.Warn("failed to calculate checksum", "error", err)
		} else {
			result.ChecksumOK = checksum == metadata.Backup.Checksum
			if !result.ChecksumOK {
				result.Valid = false
				result.Errors = append(result.Errors, "checksum mismatch")
			}
		}
	} else {
		result.ChecksumOK = true
	}

	return result, nil
}

type tempFile struct {
	path string
}

func createTempFile(reader io.Reader) (*tempFile, error) {
	f, err := os.CreateTemp("", "datasaver-validate-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		os.Remove(f.Name())
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	return &tempFile{path: f.Name()}, nil
}

func (t *tempFile) cleanup() {
	if t.path != "" {
		os.Remove(t.path)
	}
}

// VerifyRestoreIntegrity attempts to restore the backup to verify it's valid
// This is the most thorough check - it actually tries to use the backup
func (v *Validator) VerifyRestoreIntegrity(ctx context.Context, metadata *postgres.BackupMetadata) error {
	backupFile := v.findBackupFile(metadata)
	if backupFile == "" {
		return fmt.Errorf("no backup file found in metadata")
	}

	// Read the backup file
	reader, err := v.storage.Read(ctx, backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	tmpFile, err := createTempFile(reader)
	reader.Close()
	if err != nil {
		return err
	}
	defer tmpFile.cleanup()

	compressed := strings.HasSuffix(backupFile, ".gz")

	switch strings.ToLower(v.dbType) {
	case "sqlite", "sqlite3":
		return v.verifySQLiteRestore(ctx, tmpFile.path, compressed)
	case "postgres", "postgresql", "pg", "":
		return v.verifyPostgresRestore(ctx, tmpFile.path, compressed)
	default:
		return fmt.Errorf("unsupported database type: %s", v.dbType)
	}
}

func (v *Validator) findBackupFile(metadata *postgres.BackupMetadata) string {
	for _, f := range metadata.Files {
		if !strings.HasSuffix(f, ".meta.json") {
			return f
		}
	}
	return ""
}

func (v *Validator) verifySQLiteRestore(ctx context.Context, backupPath string, compressed bool) error {
	// Read and decompress if needed
	content, err := v.readBackupContent(backupPath, compressed)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Create temp database
	tmpDB, err := os.CreateTemp("", "datasaver-verify-*.db")
	if err != nil {
		return fmt.Errorf("failed to create temp database: %w", err)
	}
	tmpPath := tmpDB.Name()
	tmpDB.Close()
	defer os.Remove(tmpPath)

	// Try sqlite3 CLI first
	if v.hasSQLite3CLI() {
		cmd := exec.CommandContext(ctx, "sqlite3", tmpPath)
		cmd.Stdin = bytes.NewReader(content)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("sqlite3 import failed: %w, output: %s", err, string(output))
		}

		// Run integrity check
		integrityCmd := exec.CommandContext(ctx, "sqlite3", tmpPath, "PRAGMA integrity_check;")
		output, err = integrityCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("integrity check failed: %w", err)
		}
		if strings.TrimSpace(string(output)) != "ok" {
			return fmt.Errorf("integrity check failed: %s", string(output))
		}
		return nil
	}

	// Fallback to pure Go
	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return fmt.Errorf("failed to open temp database: %w", err)
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check query failed: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}

	return nil
}

func (v *Validator) verifyPostgresRestore(ctx context.Context, backupPath string, compressed bool) error {
	actualPath := backupPath

	// Decompress if needed
	if compressed {
		tmpFile, err := os.CreateTemp("", "datasaver-verify-*.dump")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		if err := v.decompressFile(backupPath, tmpFile); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to decompress: %w", err)
		}
		tmpFile.Close()
		actualPath = tmpFile.Name()
	}

	// Use pg_restore --list to validate the archive without needing a database
	cmd := exec.CommandContext(ctx, "pg_restore", "--list", actualPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_restore validation failed: %w, output: %s", err, string(output))
	}

	if len(output) == 0 {
		return fmt.Errorf("backup appears to be empty")
	}

	v.logger.Debug("postgres backup verified", "entries", strings.Count(string(output), "\n"))
	return nil
}

func (v *Validator) readBackupContent(path string, compressed bool) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader = f
	if compressed {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	}

	return io.ReadAll(reader)
}

func (v *Validator) decompressFile(src string, dst *os.File) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	_, err = io.Copy(dst, gr)
	return err
}

func (v *Validator) hasSQLite3CLI() bool {
	_, err := exec.LookPath("sqlite3")
	return err == nil
}

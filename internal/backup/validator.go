package backup

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/almatuck/datasaver/internal/storage"
	"github.com/almatuck/datasaver/pkg/postgres"
)

type Validator struct {
	storage storage.Backend
	logger  *slog.Logger
}

func NewValidator(store storage.Backend, logger *slog.Logger) *Validator {
	return &Validator{
		storage: store,
		logger:  logger,
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
	return &tempFile{path: ""}, nil
}

func (t *tempFile) cleanup() {
}

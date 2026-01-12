package backup

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/localrivet/datasaver/internal/storage"
	"github.com/localrivet/datasaver/pkg/postgres"
)

// TestValidator_EmptyBackup tests handling of empty backup files
func TestValidator_EmptyBackup(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create empty backup file
	if err := store.Write(ctx, "empty.dump", bytes.NewReader([]byte{})); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Create metadata pointing to empty file but claiming non-zero size
	metadata := &postgres.BackupMetadata{
		ID:        "test-empty",
		Timestamp: time.Now(),
		Files:     []string{"empty.dump", "test-empty.meta.json"},
		Backup: postgres.BackupInfo{
			SizeBytes:      1000, // Claims 1000 bytes
			CompressedSize: 1000, // but file is empty
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := NewValidatorWithDBType(store, logger, "postgres")

	result, err := validator.Validate(ctx, metadata)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	// Should detect size mismatch (empty file vs expected size)
	if result.SizeMatch {
		t.Error("Expected size mismatch for empty backup")
	}

	if result.Valid {
		t.Error("Expected backup to be marked invalid")
	}
}

// TestValidator_CorruptedChecksum tests detection of corrupted backups via checksum
func TestValidator_CorruptedChecksum(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create a backup file with known content
	content := []byte("valid backup content that should have specific checksum")
	if err := store.Write(ctx, "backup.dump", bytes.NewReader(content)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Create metadata with WRONG checksum
	metadata := &postgres.BackupMetadata{
		ID:        "test-corrupt",
		Timestamp: time.Now(),
		Files:     []string{"backup.dump", "test-corrupt.meta.json"},
		Backup: postgres.BackupInfo{
			SizeBytes:       int64(len(content)),
			CompressedSize:  int64(len(content)),
			Checksum:        "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	// Write metadata
	metaJSON, _ := metadata.ToJSON()
	if err := store.Write(ctx, "test-corrupt.meta.json", bytes.NewReader(metaJSON)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := NewValidator(store, logger)

	result, err := validator.Validate(ctx, metadata)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.ChecksumOK {
		t.Error("Expected checksum mismatch for corrupted backup")
	}

	if result.Valid {
		t.Error("Expected backup to be marked invalid")
	}
}

// TestValidator_MissingBackupFile tests handling of missing backup files
func TestValidator_MissingBackupFile(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create metadata pointing to non-existent file
	metadata := &postgres.BackupMetadata{
		ID:        "test-missing",
		Timestamp: time.Now(),
		Files:     []string{"nonexistent.dump", "test-missing.meta.json"},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := NewValidator(store, logger)

	result, err := validator.Validate(ctx, metadata)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.FileExists {
		t.Error("Expected FileExists to be false for missing file")
	}

	if result.Valid {
		t.Error("Expected backup to be marked invalid")
	}
}

// TestValidator_NoFilesInMetadata tests handling of metadata with no files
func TestValidator_NoFilesInMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create metadata with no files
	metadata := &postgres.BackupMetadata{
		ID:        "test-nofiles",
		Timestamp: time.Now(),
		Files:     []string{},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := NewValidator(store, logger)

	result, err := validator.Validate(ctx, metadata)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.Valid {
		t.Error("Expected backup to be marked invalid when no files in metadata")
	}
}

// TestValidator_PartiallyCorruptedGzip tests handling of corrupted gzip files
func TestValidator_PartiallyCorruptedGzip(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create a file that looks like gzip but is corrupted
	// Gzip header: 1f 8b 08
	corruptedGzip := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00}
	if err := store.Write(ctx, "corrupted.sql.gz", bytes.NewReader(corruptedGzip)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	metadata := &postgres.BackupMetadata{
		ID:        "test-corrupt-gz",
		Timestamp: time.Now(),
		Files:     []string{"corrupted.sql.gz", "test-corrupt-gz.meta.json"},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := NewValidatorWithDBType(store, logger, "sqlite")

	// VerifyRestoreIntegrity should fail for corrupted gzip
	err = validator.VerifyRestoreIntegrity(ctx, metadata)
	if err == nil {
		t.Error("Expected error when verifying corrupted gzip backup")
	}
}

// TestValidator_SizeMismatch tests detection of size mismatches
func TestValidator_SizeMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create backup file
	content := []byte("test content")
	if err := store.Write(ctx, "size-test.dump", bytes.NewReader(content)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Create metadata with wrong size
	metadata := &postgres.BackupMetadata{
		ID:        "test-size",
		Timestamp: time.Now(),
		Files:     []string{"size-test.dump", "test-size.meta.json"},
		Backup: postgres.BackupInfo{
			SizeBytes:      9999, // Wrong size
			CompressedSize: 9999,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := NewValidator(store, logger)

	result, err := validator.Validate(ctx, metadata)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.SizeMatch {
		t.Error("Expected size mismatch")
	}

	if result.Valid {
		t.Error("Expected backup to be marked invalid due to size mismatch")
	}
}

// TestValidator_ValidBackup tests that valid backups pass validation
func TestValidator_ValidBackup(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create backup file
	content := []byte("valid backup content")
	if err := store.Write(ctx, "valid.dump", bytes.NewReader(content)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Calculate correct checksum
	tmpFile, err := os.CreateTemp("", "checksum-*")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	_, _ = tmpFile.Write(content)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	checksum, err := postgres.CalculateChecksum(tmpFile.Name())
	if err != nil {
		t.Fatalf("CalculateChecksum() error: %v", err)
	}

	// Create metadata with correct values
	metadata := &postgres.BackupMetadata{
		ID:        "test-valid",
		Timestamp: time.Now(),
		Files:     []string{"valid.dump", "test-valid.meta.json"},
		Backup: postgres.BackupInfo{
			SizeBytes:      int64(len(content)),
			CompressedSize: int64(len(content)),
			Checksum:       checksum,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := NewValidator(store, logger)

	result, err := validator.Validate(ctx, metadata)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Expected backup to be valid, errors: %v", result.Errors)
	}

	if !result.FileExists {
		t.Error("Expected FileExists to be true")
	}

	if !result.SizeMatch {
		t.Error("Expected SizeMatch to be true")
	}

	if !result.ChecksumOK {
		t.Error("Expected ChecksumOK to be true")
	}
}

// TestValidator_TruncatedBackup tests detection of truncated backup files
func TestValidator_TruncatedBackup(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create truncated backup (claims to be 1000 bytes but only has 10)
	content := []byte("truncated")
	if err := store.Write(ctx, "truncated.dump", bytes.NewReader(content)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	metadata := &postgres.BackupMetadata{
		ID:        "test-truncated",
		Timestamp: time.Now(),
		Files:     []string{"truncated.dump", "test-truncated.meta.json"},
		Backup: postgres.BackupInfo{
			SizeBytes:      1000, // Claims larger size
			CompressedSize: 1000,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator := NewValidator(store, logger)

	result, err := validator.Validate(ctx, metadata)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.SizeMatch {
		t.Error("Expected size mismatch for truncated backup")
	}

	if result.Valid {
		t.Error("Expected backup to be marked invalid")
	}
}

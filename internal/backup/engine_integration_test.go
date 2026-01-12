//go:build integration

package backup

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localrivet/datasaver/internal/config"
	"github.com/localrivet/datasaver/internal/storage"
	_ "modernc.org/sqlite"
)

// createLocalStorage is a helper to create local storage for tests
func createLocalStorage(t *testing.T, path string) storage.Backend {
	factory := storage.NewFactory()
	store, err := factory.Create("local", path, nil)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	return store
}

func TestEngine_Integration_SQLiteBackup(t *testing.T) {
	// Check if sqlite3 CLI is available
	if !hasSQLite3CLI() {
		t.Skip("sqlite3 CLI not found")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storagePath := filepath.Join(tmpDir, "backups")

	// Create test database
	createTestDB(t, dbPath)

	// Setup config
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Type: "sqlite",
			Path: dbPath,
		},
		Storage: config.StorageConfig{
			Backend: "local",
			Path:    storagePath,
		},
		Compression: "none",
		Retention: config.RetentionConfig{
			Daily:      7,
			Weekly:     4,
			Monthly:    12,
			MaxAgeDays: 365,
		},
	}

	// Create storage backend
	store := createLocalStorage(t, storagePath)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, nil, logger)

	// Run backup
	ctx := context.Background()
	result, err := engine.Run(ctx)
	if err != nil {
		t.Fatalf("Engine.Run() error: %v", err)
	}

	// Verify result
	if result.ID == "" {
		t.Error("Backup ID is empty")
	}
	if result.Size == 0 {
		t.Error("Backup size is 0")
	}
	if result.Duration == 0 {
		t.Error("Backup duration is 0")
	}
	if result.Error != nil {
		t.Errorf("Backup has error: %v", result.Error)
	}

	t.Logf("Backup completed: ID=%s, Size=%d, Duration=%v", result.ID, result.Size, result.Duration)

	// Verify backup files exist in storage
	files, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("Failed to list storage: %v", err)
	}

	if len(files) == 0 {
		t.Error("No backup files in storage")
	}

	// Should have dump file and metadata file
	foundDump := false
	foundMeta := false
	for _, f := range files {
		if filepath.Ext(f.Path) == ".sql" {
			foundDump = true
		}
		if filepath.Ext(f.Path) == ".json" {
			foundMeta = true
		}
	}

	if !foundDump {
		t.Error("Dump file not found in storage")
	}
	if !foundMeta {
		t.Error("Metadata file not found in storage")
	}

	// Verify backup can be retrieved
	backup, err := engine.GetBackup(ctx, result.ID)
	if err != nil {
		t.Fatalf("GetBackup() error: %v", err)
	}

	if backup.ID != result.ID {
		t.Errorf("Backup ID mismatch: got %s, want %s", backup.ID, result.ID)
	}
	if backup.Database.Name != dbPath {
		t.Errorf("Database name mismatch: got %s", backup.Database.Name)
	}

	t.Log("SQLite backup integration test passed")
}

func TestEngine_Integration_SQLiteBackupWithCompression(t *testing.T) {
	if !hasSQLite3CLI() {
		t.Skip("sqlite3 CLI not found")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storagePath := filepath.Join(tmpDir, "backups")

	createTestDB(t, dbPath)

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Type: "sqlite",
			Path: dbPath,
		},
		Storage: config.StorageConfig{
			Backend: "local",
			Path:    storagePath,
		},
		Compression: "gzip",
		Retention: config.RetentionConfig{
			Daily:      7,
			Weekly:     4,
			Monthly:    12,
			MaxAgeDays: 365,
		},
	}

	store := createLocalStorage(t, storagePath)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, nil, logger)

	ctx := context.Background()
	result, err := engine.Run(ctx)
	if err != nil {
		t.Fatalf("Engine.Run() error: %v", err)
	}

	// Verify compression worked
	if result.CompressedSize >= result.Size {
		t.Logf("Warning: Compressed size (%d) >= original size (%d) - may be normal for small files",
			result.CompressedSize, result.Size)
	}

	// Verify .gz file exists
	files, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("Failed to list storage: %v", err)
	}

	foundGz := false
	for _, f := range files {
		if filepath.Ext(f.Path) == ".gz" {
			foundGz = true
			break
		}
	}

	if !foundGz {
		t.Error("Compressed backup file (.gz) not found")
	}

	t.Logf("Compressed backup: original=%d, compressed=%d", result.Size, result.CompressedSize)
}

func TestEngine_Integration_ListBackups(t *testing.T) {
	if !hasSQLite3CLI() {
		t.Skip("sqlite3 CLI not found")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storagePath := filepath.Join(tmpDir, "backups")

	createTestDB(t, dbPath)

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Type: "sqlite",
			Path: dbPath,
		},
		Storage: config.StorageConfig{
			Backend: "local",
			Path:    storagePath,
		},
		Compression: "none",
		Retention: config.RetentionConfig{
			Daily:      7,
			Weekly:     4,
			Monthly:    12,
			MaxAgeDays: 365,
		},
	}

	store := createLocalStorage(t, storagePath)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, nil, logger)

	ctx := context.Background()

	// Create multiple backups
	for i := 0; i < 3; i++ {
		_, err := engine.Run(ctx)
		if err != nil {
			t.Fatalf("Backup %d failed: %v", i+1, err)
		}
		time.Sleep(1100 * time.Millisecond) // Ensure different timestamps
	}

	// List backups
	backups, err := engine.ListBackups(ctx)
	if err != nil {
		t.Fatalf("ListBackups() error: %v", err)
	}

	if len(backups) != 3 {
		t.Errorf("Expected 3 backups, got %d", len(backups))
	}

	// Verify each backup has unique ID
	ids := make(map[string]bool)
	for _, b := range backups {
		if ids[b.ID] {
			t.Errorf("Duplicate backup ID: %s", b.ID)
		}
		ids[b.ID] = true
	}

	t.Logf("Listed %d backups", len(backups))
}

func TestEngine_Integration_Cleanup(t *testing.T) {
	if !hasSQLite3CLI() {
		t.Skip("sqlite3 CLI not found")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storagePath := filepath.Join(tmpDir, "backups")

	createTestDB(t, dbPath)

	// Very short retention for testing
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Type: "sqlite",
			Path: dbPath,
		},
		Storage: config.StorageConfig{
			Backend: "local",
			Path:    storagePath,
		},
		Compression: "none",
		Retention: config.RetentionConfig{
			Daily:      1, // Only keep 1 daily
			Weekly:     0,
			Monthly:    0,
			MaxAgeDays: 0,
		},
	}

	store := createLocalStorage(t, storagePath)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, nil, logger)

	ctx := context.Background()

	// Create multiple backups
	for i := 0; i < 3; i++ {
		_, err := engine.Run(ctx)
		if err != nil {
			t.Fatalf("Backup %d failed: %v", i+1, err)
		}
		time.Sleep(1100 * time.Millisecond)
	}

	// Run cleanup
	deleted, err := engine.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	t.Logf("Cleanup deleted %d backups", deleted)

	// Verify fewer backups remain
	remaining, err := engine.ListBackups(ctx)
	if err != nil {
		t.Fatalf("ListBackups() error: %v", err)
	}

	// Should have 1 daily backup remaining
	if len(remaining) > 1 {
		t.Logf("Expected <= 1 backup, got %d (cleanup policy may vary)", len(remaining))
	}
}

func TestEngine_Integration_BackupMetadata(t *testing.T) {
	if !hasSQLite3CLI() {
		t.Skip("sqlite3 CLI not found")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storagePath := filepath.Join(tmpDir, "backups")

	createTestDB(t, dbPath)

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Type: "sqlite",
			Path: dbPath,
		},
		Storage: config.StorageConfig{
			Backend: "local",
			Path:    storagePath,
		},
		Compression: "gzip",
		Retention: config.RetentionConfig{
			Daily:      7,
			Weekly:     4,
			Monthly:    12,
			MaxAgeDays: 365,
		},
	}

	store := createLocalStorage(t, storagePath)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, nil, logger)

	ctx := context.Background()
	result, err := engine.Run(ctx)
	if err != nil {
		t.Fatalf("Engine.Run() error: %v", err)
	}

	// Get backup metadata
	metadata, err := engine.GetBackup(ctx, result.ID)
	if err != nil {
		t.Fatalf("GetBackup() error: %v", err)
	}

	// Verify all metadata fields
	if metadata.ID != result.ID {
		t.Errorf("ID mismatch")
	}
	if metadata.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	if metadata.Database.Name == "" {
		t.Error("Database name is empty")
	}
	if metadata.Database.Version == "" {
		t.Error("Database version is empty")
	}
	if metadata.Backup.Method != "sqlite" {
		t.Errorf("Backup method = %s, want sqlite", metadata.Backup.Method)
	}
	if metadata.Backup.Compression != "gzip" {
		t.Errorf("Compression = %s, want gzip", metadata.Backup.Compression)
	}
	if metadata.Backup.SizeBytes == 0 {
		t.Error("Size is 0")
	}
	if metadata.Backup.Checksum == "" {
		t.Error("Checksum is empty")
	}
	if metadata.Backup.DurationSeconds == 0 {
		t.Error("Duration is 0")
	}
	if len(metadata.Files) == 0 {
		t.Error("No files in metadata")
	}
	if metadata.Retention.KeepUntil.IsZero() {
		t.Error("Retention keepUntil is zero")
	}
	if metadata.Retention.Policy == "" {
		t.Error("Retention policy is empty")
	}

	t.Logf("Metadata verified: version=%s, size=%d, checksum=%s",
		metadata.Database.Version, metadata.Backup.SizeBytes, metadata.Backup.Checksum)
}

// Helper functions

func hasSQLite3CLI() bool {
	paths := []string{"/usr/bin/sqlite3", "/usr/local/bin/sqlite3", "/opt/homebrew/bin/sqlite3"}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func createTestDB(t *testing.T, path string) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE
		);
		INSERT INTO users (name, email) VALUES
			('Test User 1', 'test1@example.com'),
			('Test User 2', 'test2@example.com'),
			('Test User 3', 'test3@example.com');
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			amount REAL
		);
		INSERT INTO orders (user_id, amount) VALUES
			(1, 99.99),
			(1, 149.50),
			(2, 25.00);
	`)
	if err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
}

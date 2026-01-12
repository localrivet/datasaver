package restore

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/localrivet/datasaver/internal/config"
	"github.com/localrivet/datasaver/internal/storage"
	"github.com/localrivet/datasaver/pkg/postgres"
)

// Mock storage backend for testing
type mockStorage struct {
	files   map[string][]byte
	readErr error
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		files: make(map[string][]byte),
	}
}

func (m *mockStorage) Write(ctx context.Context, path string, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.files[path] = data
	return nil
}

func (m *mockStorage) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	data, ok := m.files[path]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockStorage) Delete(ctx context.Context, path string) error {
	delete(m.files, path)
	return nil
}

func (m *mockStorage) List(ctx context.Context, prefix string) ([]storage.FileInfo, error) {
	var files []storage.FileInfo
	for path := range m.files {
		files = append(files, storage.FileInfo{
			Path:         path,
			Size:         int64(len(m.files[path])),
			LastModified: time.Now(),
		})
	}
	return files, nil
}

func (m *mockStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, ok := m.files[path]
	return ok, nil
}

func (m *mockStorage) Size(ctx context.Context, path string) (int64, error) {
	data, ok := m.files[path]
	if !ok {
		return 0, storage.ErrNotFound
	}
	return int64(len(data)), nil
}

func TestNewEngine(t *testing.T) {
	cfg := &config.Config{}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	engine := NewEngine(cfg, store, logger)

	if engine == nil {
		t.Error("NewEngine() returned nil")
	}
	if engine.cfg != cfg {
		t.Error("NewEngine() cfg mismatch")
	}
	if engine.storage != store {
		t.Error("NewEngine() storage mismatch")
	}
}

func TestRestoreOptions(t *testing.T) {
	opts := RestoreOptions{
		BackupID: "backup-001",
		TargetDB: "target_db",
		DryRun:   true,
		Force:    false,
	}

	if opts.BackupID != "backup-001" {
		t.Errorf("BackupID = %v, want backup-001", opts.BackupID)
	}
	if opts.TargetDB != "target_db" {
		t.Errorf("TargetDB = %v, want target_db", opts.TargetDB)
	}
	if !opts.DryRun {
		t.Error("DryRun = false, want true")
	}
	if opts.Force {
		t.Error("Force = true, want false")
	}
}

func TestRestoreResult(t *testing.T) {
	result := RestoreResult{
		BackupID: "backup-001",
		TargetDB: "target_db",
		Success:  true,
		Error:    nil,
	}

	if result.BackupID != "backup-001" {
		t.Errorf("BackupID = %v, want backup-001", result.BackupID)
	}
	if result.TargetDB != "target_db" {
		t.Errorf("TargetDB = %v, want target_db", result.TargetDB)
	}
	if !result.Success {
		t.Error("Success = false, want true")
	}
}

func TestEngine_Restore_BackupNotFound(t *testing.T) {
	cfg := &config.Config{}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, logger)

	opts := RestoreOptions{
		BackupID: "nonexistent-backup",
	}

	result, err := engine.Restore(context.Background(), opts)

	if err == nil {
		t.Error("Restore() should error when backup not found")
	}
	if result.Success {
		t.Error("Restore() Success = true, want false")
	}
}

func TestEngine_Restore_DryRun(t *testing.T) {
	cfg := &config.Config{}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, logger)

	// Create mock metadata
	metadata := &postgres.BackupMetadata{
		ID:        "backup-001",
		Timestamp: time.Now(),
		Database: postgres.DatabaseMetadata{
			Name: "testdb",
		},
		Files: []string{"backup-001.dump", "backup-001.meta.json"},
	}

	metaJSON, _ := json.Marshal(metadata)
	store.files["backup-001.meta.json"] = metaJSON
	store.files["backup-001.dump"] = []byte("mock backup data")

	opts := RestoreOptions{
		BackupID: "backup-001",
		TargetDB: "restore_target",
		DryRun:   true,
	}

	result, err := engine.Restore(context.Background(), opts)

	if err != nil {
		t.Errorf("Restore() dry run error = %v", err)
	}
	if !result.Success {
		t.Error("Restore() dry run Success = false, want true")
	}
}

func TestEngine_Restore_NoBackupFile(t *testing.T) {
	cfg := &config.Config{}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, logger)

	// Create metadata with only the meta file
	metadata := &postgres.BackupMetadata{
		ID:    "backup-001",
		Files: []string{"backup-001.meta.json"}, // No actual backup file
	}

	metaJSON, _ := json.Marshal(metadata)
	store.files["backup-001.meta.json"] = metaJSON

	opts := RestoreOptions{
		BackupID: "backup-001",
	}

	result, err := engine.Restore(context.Background(), opts)

	if err == nil {
		t.Error("Restore() should error when no backup file found")
	}
	if result.Success {
		t.Error("Restore() Success = true, want false")
	}
}

func TestEngine_Restore_InvalidMetadata(t *testing.T) {
	cfg := &config.Config{}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, logger)

	// Create invalid metadata
	store.files["backup-001.meta.json"] = []byte("invalid json{{{")

	opts := RestoreOptions{
		BackupID: "backup-001",
	}

	result, err := engine.Restore(context.Background(), opts)

	if err == nil {
		t.Error("Restore() should error with invalid metadata")
	}
	if result.Success {
		t.Error("Restore() Success = true, want false")
	}
}

func TestEngine_parseConnectionInfo_FromURL(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			URL: "postgres://user:pass@localhost:5432/testdb",
		},
	}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, logger)

	host, port, dbName, user, password := engine.parseConnectionInfo()

	if host != "localhost" {
		t.Errorf("host = %v, want localhost", host)
	}
	if port != 5432 {
		t.Errorf("port = %v, want 5432", port)
	}
	if dbName != "testdb" {
		t.Errorf("dbName = %v, want testdb", dbName)
	}
	if user != "user" {
		t.Errorf("user = %v, want user", user)
	}
	if password != "pass" {
		t.Errorf("password = %v, want pass", password)
	}
}

func TestEngine_parseConnectionInfo_FromConfig(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "dbhost",
			Port:     5433,
			Name:     "mydb",
			User:     "dbuser",
			Password: "dbpass",
		},
	}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, logger)

	host, port, dbName, user, password := engine.parseConnectionInfo()

	if host != "dbhost" {
		t.Errorf("host = %v, want dbhost", host)
	}
	if port != 5433 {
		t.Errorf("port = %v, want 5433", port)
	}
	if dbName != "mydb" {
		t.Errorf("dbName = %v, want mydb", dbName)
	}
	if user != "dbuser" {
		t.Errorf("user = %v, want dbuser", user)
	}
	if password != "dbpass" {
		t.Errorf("password = %v, want dbpass", password)
	}
}

func TestEngine_parseConnectionInfo_URLDefaultPort(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			URL: "postgres://user:pass@localhost/testdb", // No port specified
		},
	}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, logger)

	_, port, _, _, _ := engine.parseConnectionInfo()

	if port != 5432 {
		t.Errorf("port = %v, want 5432 (default)", port)
	}
}

func TestEngine_parseConnectionInfo_InvalidURL(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			URL:      "://invalid-url",
			Host:     "fallback-host",
			Port:     5432,
			Name:     "fallback-db",
			User:     "fallback-user",
			Password: "fallback-pass",
		},
	}
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(cfg, store, logger)

	host, _, _, _, _ := engine.parseConnectionInfo()

	// Should fall back to config values when URL is invalid
	if host != "fallback-host" {
		t.Errorf("host = %v, want fallback-host", host)
	}
}

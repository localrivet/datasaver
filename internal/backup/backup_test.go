package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/localrivet/datasaver/internal/storage"
	"github.com/localrivet/datasaver/pkg/postgres"
)

// Mock storage backend for testing
type mockStorage struct {
	files     map[string][]byte
	sizeErr   error
	existsErr error
	readErr   error
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
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, ok := m.files[path]
	return ok, nil
}

func (m *mockStorage) Size(ctx context.Context, path string) (int64, error) {
	if m.sizeErr != nil {
		return 0, m.sizeErr
	}
	data, ok := m.files[path]
	if !ok {
		return 0, storage.ErrNotFound
	}
	return int64(len(data)), nil
}

func TestBackupResult(t *testing.T) {
	result := BackupResult{
		ID:             "backup-001",
		Timestamp:      time.Now(),
		Size:           1024,
		CompressedSize: 512,
		Duration:       5 * time.Second,
		Checksum:       "abc123",
		Error:          nil,
	}

	if result.ID != "backup-001" {
		t.Errorf("ID = %v, want backup-001", result.ID)
	}
	if result.Size != 1024 {
		t.Errorf("Size = %v, want 1024", result.Size)
	}
	if result.CompressedSize != 512 {
		t.Errorf("CompressedSize = %v, want 512", result.CompressedSize)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %v, want 3", cfg.MaxAttempts)
	}
	if cfg.InitialWait != 1*time.Second {
		t.Errorf("InitialWait = %v, want 1s", cfg.InitialWait)
	}
	if cfg.MaxWait != 30*time.Second {
		t.Errorf("MaxWait = %v, want 30s", cfg.MaxWait)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", cfg.Multiplier)
	}
}

func TestWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxAttempts: 3,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     100 * time.Millisecond,
		Multiplier:  2.0,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	callCount := 0
	result, err := WithRetry(ctx, cfg, logger, "test-op", func() (string, error) {
		callCount++
		return "success", nil
	})

	if err != nil {
		t.Errorf("WithRetry() error = %v, want nil", err)
	}
	if result != "success" {
		t.Errorf("WithRetry() result = %v, want success", result)
	}
	if callCount != 1 {
		t.Errorf("WithRetry() callCount = %v, want 1", callCount)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxAttempts: 3,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     100 * time.Millisecond,
		Multiplier:  2.0,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	callCount := 0
	result, err := WithRetry(ctx, cfg, logger, "test-op", func() (string, error) {
		callCount++
		if callCount < 3 {
			return "", errors.New("temporary error")
		}
		return "success", nil
	})

	if err != nil {
		t.Errorf("WithRetry() error = %v, want nil", err)
	}
	if result != "success" {
		t.Errorf("WithRetry() result = %v, want success", result)
	}
	if callCount != 3 {
		t.Errorf("WithRetry() callCount = %v, want 3", callCount)
	}
}

func TestWithRetry_MaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxAttempts: 3,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     100 * time.Millisecond,
		Multiplier:  2.0,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	callCount := 0
	expectedErr := errors.New("persistent error")
	_, err := WithRetry(ctx, cfg, logger, "test-op", func() (string, error) {
		callCount++
		return "", expectedErr
	})

	if err != expectedErr {
		t.Errorf("WithRetry() error = %v, want %v", err, expectedErr)
	}
	if callCount != 3 {
		t.Errorf("WithRetry() callCount = %v, want 3", callCount)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	cfg := RetryConfig{
		MaxAttempts: 3,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     100 * time.Millisecond,
		Multiplier:  2.0,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	callCount := 0
	authErr := errors.New("authentication failed")
	_, err := WithRetry(ctx, cfg, logger, "test-op", func() (string, error) {
		callCount++
		return "", authErr
	})

	if err != authErr {
		t.Errorf("WithRetry() error = %v, want %v", err, authErr)
	}
	if callCount != 1 {
		t.Errorf("WithRetry() callCount = %v, want 1 (should not retry)", callCount)
	}
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := RetryConfig{
		MaxAttempts: 3,
		InitialWait: 10 * time.Millisecond,
		MaxWait:     100 * time.Millisecond,
		Multiplier:  2.0,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := WithRetry(ctx, cfg, logger, "test-op", func() (string, error) {
		return "success", nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("WithRetry() error = %v, want context.Canceled", err)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context canceled",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "authentication failed",
			err:  errors.New("authentication failed"),
			want: false,
		},
		{
			name: "permission denied",
			err:  errors.New("permission denied"),
			want: false,
		},
		{
			name: "access denied",
			err:  errors.New("access denied"),
			want: false,
		},
		{
			name: "invalid password",
			err:  errors.New("invalid password"),
			want: false,
		},
		{
			name: "database does not exist",
			err:  errors.New("database does not exist"),
			want: false,
		},
		{
			name: "role does not exist",
			err:  errors.New("role does not exist"),
			want: false,
		},
		{
			name: "temporary network error",
			err:  errors.New("connection refused"),
			want: true,
		},
		{
			name: "timeout",
			err:  errors.New("i/o timeout"),
			want: true,
		},
		{
			name: "generic error",
			err:  errors.New("something went wrong"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestNewValidator(t *testing.T) {
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	v := NewValidator(store, logger)
	if v == nil {
		t.Error("NewValidator() returned nil")
	}
}

func TestValidationResult(t *testing.T) {
	result := ValidationResult{
		BackupID:   "backup-001",
		Valid:      true,
		FileExists: true,
		SizeMatch:  true,
		ChecksumOK: true,
		Errors:     nil,
	}

	if result.BackupID != "backup-001" {
		t.Errorf("BackupID = %v, want backup-001", result.BackupID)
	}
	if !result.Valid {
		t.Error("Valid = false, want true")
	}
}

func TestValidator_Validate_NoFiles(t *testing.T) {
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	v := NewValidator(store, logger)

	metadata := &postgres.BackupMetadata{
		ID:    "backup-001",
		Files: []string{},
	}

	result, err := v.Validate(context.Background(), metadata)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Valid {
		t.Error("Validate() Valid = true, want false for no files")
	}
	if len(result.Errors) == 0 {
		t.Error("Validate() should have errors for no files")
	}
}

func TestValidator_Validate_OnlyMetadataFile(t *testing.T) {
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	v := NewValidator(store, logger)

	metadata := &postgres.BackupMetadata{
		ID:    "backup-001",
		Files: []string{"backup-001.meta.json"},
	}

	result, err := v.Validate(context.Background(), metadata)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Valid {
		t.Error("Validate() Valid = true, want false when only metadata file exists")
	}
}

func TestValidator_Validate_FileNotExists(t *testing.T) {
	store := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	v := NewValidator(store, logger)

	metadata := &postgres.BackupMetadata{
		ID:    "backup-001",
		Files: []string{"backup-001.dump.gz", "backup-001.meta.json"},
	}

	result, err := v.Validate(context.Background(), metadata)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Valid {
		t.Error("Validate() Valid = true, want false when file doesn't exist")
	}
	if result.FileExists {
		t.Error("Validate() FileExists = true, want false")
	}
}

func TestValidator_Validate_SizeMismatch(t *testing.T) {
	store := newMockStorage()
	store.files["backup-001.dump.gz"] = []byte("small")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	v := NewValidator(store, logger)

	metadata := &postgres.BackupMetadata{
		ID:    "backup-001",
		Files: []string{"backup-001.dump.gz", "backup-001.meta.json"},
		Backup: postgres.BackupInfo{
			CompressedSize: 1000, // Different from actual size
		},
	}

	result, err := v.Validate(context.Background(), metadata)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Valid {
		t.Error("Validate() Valid = true, want false for size mismatch")
	}
	if result.SizeMatch {
		t.Error("Validate() SizeMatch = true, want false")
	}
}

func TestValidator_Validate_Success(t *testing.T) {
	store := newMockStorage()
	content := []byte("backup content")
	store.files["backup-001.dump.gz"] = content
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	v := NewValidator(store, logger)

	metadata := &postgres.BackupMetadata{
		ID:    "backup-001",
		Files: []string{"backup-001.dump.gz", "backup-001.meta.json"},
		Backup: postgres.BackupInfo{
			CompressedSize: int64(len(content)),
			Checksum:       "", // No checksum to verify
		},
	}

	result, err := v.Validate(context.Background(), metadata)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if !result.Valid {
		t.Errorf("Validate() Valid = false, want true. Errors: %v", result.Errors)
	}
	if !result.FileExists {
		t.Error("Validate() FileExists = false, want true")
	}
	if !result.SizeMatch {
		t.Error("Validate() SizeMatch = false, want true")
	}
}

func TestValidator_Validate_ExistsError(t *testing.T) {
	store := newMockStorage()
	store.existsErr = errors.New("storage error")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	v := NewValidator(store, logger)

	metadata := &postgres.BackupMetadata{
		ID:    "backup-001",
		Files: []string{"backup-001.dump.gz"},
	}

	_, err := v.Validate(context.Background(), metadata)
	if err == nil {
		t.Error("Validate() error = nil, want error")
	}
}

func TestNewScheduler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewScheduler(nil, "0 2 * * *", logger)

	if s == nil {
		t.Error("NewScheduler() returned nil")
	}
	if s.schedule != "0 2 * * *" {
		t.Errorf("schedule = %v, want 0 2 * * *", s.schedule)
	}
	if s.IsRunning() {
		t.Error("IsRunning() = true, want false for new scheduler")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewScheduler(nil, "0 2 * * *", logger)

	ctx := context.Background()
	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !s.IsRunning() {
		t.Error("IsRunning() = false after Start(), want true")
	}

	// Start again should be idempotent
	err = s.Start(ctx)
	if err != nil {
		t.Errorf("Start() second call error = %v", err)
	}

	s.Stop()
	if s.IsRunning() {
		t.Error("IsRunning() = true after Stop(), want false")
	}

	// Stop again should be idempotent
	s.Stop()
}

func TestScheduler_NextRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewScheduler(nil, "0 2 * * *", logger)

	ctx := context.Background()
	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer s.Stop()

	nextRun := s.NextRun()
	if nextRun.IsZero() {
		t.Error("NextRun() returned zero time after Start()")
	}
}

func TestScheduler_Engine(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Create a scheduler with nil engine to test the getter
	s := NewScheduler(nil, "0 2 * * *", logger)

	if s.Engine() != nil {
		t.Error("Engine() should return nil for scheduler created with nil engine")
	}
}

func TestCompressGzip(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := tmpDir + "/source.txt"
	dstPath := tmpDir + "/source.txt.gz"

	// Create source file
	content := []byte("test content for compression")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Compress
	err := compressGzip(srcPath, dstPath)
	if err != nil {
		t.Fatalf("compressGzip() error = %v", err)
	}

	// Verify compressed file exists and is smaller (for this small file it might not be)
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("Failed to stat compressed file: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Compressed file is empty")
	}
}

func TestCompressGzip_SourceNotFound(t *testing.T) {
	err := compressGzip("/nonexistent/file.txt", "/tmp/out.gz")
	if err == nil {
		t.Error("compressGzip() should error when source doesn't exist")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "WORLD", true}, // case insensitive
		{"hello", "hello", true},
		{"hello", "goodbye", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestEqualFoldSlice(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want bool
	}{
		{"hello", "hello", true},
		{"Hello", "hello", true},
		{"HELLO", "hello", true},
		{"hello", "world", false},
		{"hi", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := equalFoldSlice(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("equalFoldSlice(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

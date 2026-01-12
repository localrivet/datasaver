package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	if f == nil {
		t.Error("NewFactory() returned nil")
	}
}

func TestFactory_Create(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		backend   string
		path      string
		s3Config  *S3Config
		wantErr   bool
		errString string
	}{
		{
			name:    "local backend",
			backend: "local",
			path:    tmpDir,
			wantErr: false,
		},
		{
			name:    "s3 backend with config",
			backend: "s3",
			s3Config: &S3Config{
				Bucket:    "test-bucket",
				Endpoint:  "localhost:9000",
				AccessKey: "access",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name:      "s3 backend without config",
			backend:   "s3",
			s3Config:  nil,
			wantErr:   true,
			errString: "s3 config required",
		},
		{
			name:      "unknown backend",
			backend:   "gcs",
			wantErr:   true,
			errString: "unknown backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFactory()
			backend, err := f.Create(tt.backend, tt.path, tt.s3Config)

			if tt.wantErr {
				if err == nil {
					t.Error("Create() expected error, got nil")
				} else if tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("Create() error = %v, want error containing %q", err, tt.errString)
				}
				return
			}

			if err != nil {
				t.Errorf("Create() unexpected error: %v", err)
				return
			}

			if backend == nil {
				t.Error("Create() returned nil backend")
			}
		})
	}
}

func TestStorageError(t *testing.T) {
	err := &StorageError{
		Op:   "write",
		Path: "/test/path",
		Err:  io.EOF,
	}

	expected := "write /test/path: EOF"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}

	if err.Unwrap() != io.EOF {
		t.Errorf("Unwrap() = %v, want io.EOF", err.Unwrap())
	}
}

func TestNewLocalStorage(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		basePath string
		wantErr  bool
	}{
		{
			name:     "valid path",
			basePath: filepath.Join(tmpDir, "storage"),
			wantErr:  false,
		},
		{
			name:     "nested path",
			basePath: filepath.Join(tmpDir, "a", "b", "c", "storage"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewLocalStorage(tt.basePath)

			if tt.wantErr {
				if err == nil {
					t.Error("NewLocalStorage() expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("NewLocalStorage() error: %v", err)
				return
			}

			if storage == nil {
				t.Error("NewLocalStorage() returned nil")
			}

			// Verify directory was created
			if _, err := os.Stat(tt.basePath); os.IsNotExist(err) {
				t.Errorf("NewLocalStorage() did not create directory %s", tt.basePath)
			}
		})
	}
}

func TestLocalStorage_Write(t *testing.T) {
	tmpDir := t.TempDir()
	storage, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name    string
		path    string
		content string
		wantErr bool
	}{
		{
			name:    "simple file",
			path:    "test.txt",
			content: "hello world",
			wantErr: false,
		},
		{
			name:    "nested path",
			path:    "subdir/nested/file.txt",
			content: "nested content",
			wantErr: false,
		},
		{
			name:    "empty content",
			path:    "empty.txt",
			content: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			err := storage.Write(ctx, tt.path, reader)

			if tt.wantErr {
				if err == nil {
					t.Error("Write() expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("Write() error: %v", err)
				return
			}

			// Verify file was written
			fullPath := filepath.Join(tmpDir, tt.path)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				t.Errorf("Failed to read written file: %v", err)
				return
			}

			if string(data) != tt.content {
				t.Errorf("Written content = %q, want %q", string(data), tt.content)
			}
		})
	}
}

func TestLocalStorage_Read(t *testing.T) {
	tmpDir := t.TempDir()
	storage, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Create test file
	testContent := "test file content"
	testPath := "testfile.txt"
	os.WriteFile(filepath.Join(tmpDir, testPath), []byte(testContent), 0644)

	tests := []struct {
		name        string
		path        string
		wantContent string
		wantErr     bool
		isNotFound  bool
	}{
		{
			name:        "existing file",
			path:        testPath,
			wantContent: testContent,
			wantErr:     false,
		},
		{
			name:       "non-existent file",
			path:       "nonexistent.txt",
			wantErr:    true,
			isNotFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := storage.Read(ctx, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Error("Read() expected error")
					return
				}
				if tt.isNotFound && err != ErrNotFound {
					t.Errorf("Read() error = %v, want ErrNotFound", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Read() error: %v", err)
				return
			}
			defer reader.Close()

			data, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("Failed to read content: %v", err)
				return
			}

			if string(data) != tt.wantContent {
				t.Errorf("Read content = %q, want %q", string(data), tt.wantContent)
			}
		})
	}
}

func TestLocalStorage_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	storage, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Create test file
	testPath := "todelete.txt"
	os.WriteFile(filepath.Join(tmpDir, testPath), []byte("delete me"), 0644)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "existing file",
			path:    testPath,
			wantErr: false,
		},
		{
			name:    "non-existent file",
			path:    "nonexistent.txt",
			wantErr: false, // Delete of non-existent is not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.Delete(ctx, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Error("Delete() expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("Delete() error: %v", err)
				return
			}

			// Verify file was deleted
			fullPath := filepath.Join(tmpDir, tt.path)
			if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
				t.Errorf("Delete() file still exists: %s", fullPath)
			}
		})
	}
}

func TestLocalStorage_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	storage, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Create test file
	testPath := "exists.txt"
	os.WriteFile(filepath.Join(tmpDir, testPath), []byte("exists"), 0644)

	tests := []struct {
		name       string
		path       string
		wantExists bool
		wantErr    bool
	}{
		{
			name:       "existing file",
			path:       testPath,
			wantExists: true,
			wantErr:    false,
		},
		{
			name:       "non-existent file",
			path:       "nonexistent.txt",
			wantExists: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := storage.Exists(ctx, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Error("Exists() expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("Exists() error: %v", err)
				return
			}

			if exists != tt.wantExists {
				t.Errorf("Exists() = %v, want %v", exists, tt.wantExists)
			}
		})
	}
}

func TestLocalStorage_Size(t *testing.T) {
	tmpDir := t.TempDir()
	storage, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Create test file with known size
	testPath := "sized.txt"
	testContent := "12345678901234567890" // 20 bytes
	os.WriteFile(filepath.Join(tmpDir, testPath), []byte(testContent), 0644)

	tests := []struct {
		name       string
		path       string
		wantSize   int64
		wantErr    bool
		isNotFound bool
	}{
		{
			name:     "existing file",
			path:     testPath,
			wantSize: 20,
			wantErr:  false,
		},
		{
			name:       "non-existent file",
			path:       "nonexistent.txt",
			wantErr:    true,
			isNotFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := storage.Size(ctx, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Error("Size() expected error")
					return
				}
				if tt.isNotFound && err != ErrNotFound {
					t.Errorf("Size() error = %v, want ErrNotFound", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Size() error: %v", err)
				return
			}

			if size != tt.wantSize {
				t.Errorf("Size() = %d, want %d", size, tt.wantSize)
			}
		})
	}
}

func TestLocalStorage_List(t *testing.T) {
	tmpDir := t.TempDir()
	storage, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	// Create test files
	files := map[string]string{
		"file1.txt":           "content1",
		"file2.txt":           "content2",
		"subdir/file3.txt":    "content3",
		"subdir/file4.txt":    "content4",
		"other/file5.txt":     "content5",
		"backup-2024-01.dump": "backup1",
		"backup-2024-02.dump": "backup2",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
		// Add small delay to ensure different modification times
		time.Sleep(10 * time.Millisecond)
	}

	tests := []struct {
		name       string
		prefix     string
		wantCount  int
		wantPrefix string
	}{
		{
			name:      "no prefix lists all",
			prefix:    "",
			wantCount: len(files),
		},
		{
			name:       "prefix filter",
			prefix:     "subdir/",
			wantCount:  2,
			wantPrefix: "subdir/",
		},
		{
			name:       "backup prefix",
			prefix:     "backup-",
			wantCount:  2,
			wantPrefix: "backup-",
		},
		{
			name:      "non-matching prefix",
			prefix:    "nonexistent/",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := storage.List(ctx, tt.prefix)
			if err != nil {
				t.Errorf("List() error: %v", err)
				return
			}

			if len(result) != tt.wantCount {
				t.Errorf("List() returned %d files, want %d", len(result), tt.wantCount)
				for _, f := range result {
					t.Logf("  - %s", f.Path)
				}
			}

			// Verify all results have correct prefix
			if tt.wantPrefix != "" {
				for _, f := range result {
					if !strings.HasPrefix(f.Path, tt.wantPrefix) {
						t.Errorf("List() file %s doesn't have prefix %s", f.Path, tt.wantPrefix)
					}
				}
			}

			// Verify sorted by LastModified (newest first)
			for i := 1; i < len(result); i++ {
				if result[i].LastModified.After(result[i-1].LastModified) {
					t.Errorf("List() not sorted by LastModified descending")
					break
				}
			}
		})
	}
}

func TestLocalStorage_WriteReadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	storage, _ := NewLocalStorage(tmpDir)
	ctx := context.Background()

	testCases := []struct {
		name    string
		path    string
		content []byte
	}{
		{
			name:    "text content",
			path:    "text.txt",
			content: []byte("Hello, World!"),
		},
		{
			name:    "binary content",
			path:    "binary.bin",
			content: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		},
		{
			name:    "large content",
			path:    "large.txt",
			content: bytes.Repeat([]byte("x"), 1024*1024), // 1MB
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write
			err := storage.Write(ctx, tc.path, bytes.NewReader(tc.content))
			if err != nil {
				t.Fatalf("Write() error: %v", err)
			}

			// Read
			reader, err := storage.Read(ctx, tc.path)
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			defer reader.Close()

			got, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("ReadAll() error: %v", err)
			}

			if !bytes.Equal(got, tc.content) {
				t.Errorf("Round-trip content mismatch, got %d bytes, want %d bytes", len(got), len(tc.content))
			}
		})
	}
}

func TestMatchPrefix(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   bool
	}{
		{"backup-2024-01.dump", "backup-", true},
		{"backup-2024-01.dump", "backup-2024", true},
		{"backup-2024-01.dump", "other", false},
		{"subdir/file.txt", "subdir/", true},
		{"subdir/file.txt", "subdir/file", true},
		{"file.txt", "", true},
		{"", "prefix", false},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.prefix, func(t *testing.T) {
			got := matchPrefix(tt.path, tt.prefix)
			if got != tt.want {
				t.Errorf("matchPrefix(%q, %q) = %v, want %v", tt.path, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestNewS3Storage(t *testing.T) {
	tests := []struct {
		name    string
		cfg     S3Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: S3Config{
				Bucket:    "test-bucket",
				Endpoint:  "localhost:9000",
				Region:    "us-east-1",
				AccessKey: "access",
				SecretKey: "secret",
				UseSSL:    false,
			},
			wantErr: false,
		},
		{
			name: "empty endpoint uses default",
			cfg: S3Config{
				Bucket:    "test-bucket",
				AccessKey: "access",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "strips https prefix from endpoint",
			cfg: S3Config{
				Bucket:    "test-bucket",
				Endpoint:  "https://s3.amazonaws.com",
				AccessKey: "access",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "strips http prefix from endpoint",
			cfg: S3Config{
				Bucket:    "test-bucket",
				Endpoint:  "http://localhost:9000",
				AccessKey: "access",
				SecretKey: "secret",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewS3Storage(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("NewS3Storage() expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("NewS3Storage() error: %v", err)
				return
			}

			if storage == nil {
				t.Error("NewS3Storage() returned nil")
			}

			if storage.bucket != tt.cfg.Bucket {
				t.Errorf("bucket = %v, want %v", storage.bucket, tt.cfg.Bucket)
			}
		})
	}
}

func TestFileInfo(t *testing.T) {
	now := time.Now()
	fi := FileInfo{
		Path:         "test/file.txt",
		Size:         1024,
		LastModified: now,
		IsDir:        false,
	}

	if fi.Path != "test/file.txt" {
		t.Errorf("Path = %v, want test/file.txt", fi.Path)
	}
	if fi.Size != 1024 {
		t.Errorf("Size = %v, want 1024", fi.Size)
	}
	if !fi.LastModified.Equal(now) {
		t.Errorf("LastModified = %v, want %v", fi.LastModified, now)
	}
	if fi.IsDir {
		t.Error("IsDir = true, want false")
	}
}

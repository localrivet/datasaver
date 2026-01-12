//go:build integration

package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalStorage_Integration_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create 10MB of random data
	size := 10 * 1024 * 1024
	data := make([]byte, size)
	rand.Read(data)

	// Write large file
	start := time.Now()
	if err := store.Write(ctx, "large_backup.dump", bytes.NewReader(data)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	writeDuration := time.Since(start)

	// Verify file exists and has correct size
	fileSize, err := store.Size(ctx, "large_backup.dump")
	if err != nil {
		t.Fatalf("Size() error: %v", err)
	}
	if fileSize != int64(size) {
		t.Errorf("Size = %d, want %d", fileSize, size)
	}

	// Read and verify content
	start = time.Now()
	reader, err := store.Read(ctx, "large_backup.dump")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	readData, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	readDuration := time.Since(start)

	if !bytes.Equal(data, readData) {
		t.Error("Data mismatch after read")
	}

	t.Logf("10MB file: write=%v, read=%v", writeDuration, readDuration)
}

func TestLocalStorage_Integration_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Write multiple files concurrently
	numFiles := 10
	done := make(chan error, numFiles)

	for i := 0; i < numFiles; i++ {
		go func(idx int) {
			data := []byte(fmt.Sprintf("content for file %d", idx))
			path := fmt.Sprintf("concurrent_%d.txt", idx)
			err := store.Write(ctx, path, bytes.NewReader(data))
			done <- err
		}(i)
	}

	// Wait for all writes
	for i := 0; i < numFiles; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent write failed: %v", err)
		}
	}

	// Verify all files exist
	files, err := store.List(ctx, "concurrent_")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(files) != numFiles {
		t.Errorf("Expected %d files, got %d", numFiles, len(files))
	}
}

func TestLocalStorage_Integration_NestedPaths(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Write to nested path
	nestedPath := "2024/01/15/backup.dump"
	data := []byte("nested backup data")

	if err := store.Write(ctx, nestedPath, bytes.NewReader(data)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Verify file exists
	exists, err := store.Exists(ctx, nestedPath)
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if !exists {
		t.Error("Nested file does not exist")
	}

	// Verify we can read it
	reader, err := store.Read(ctx, nestedPath)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	defer reader.Close()

	readData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}

	if !bytes.Equal(data, readData) {
		t.Error("Data mismatch")
	}

	// Verify physical path exists
	fullPath := filepath.Join(tmpDir, nestedPath)
	if _, err := os.Stat(fullPath); err != nil {
		t.Errorf("Physical file not found: %v", err)
	}
}

func TestLocalStorage_Integration_DeleteAll(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create multiple files
	for i := 0; i < 5; i++ {
		path := fmt.Sprintf("delete_test_%d.txt", i)
		data := []byte(fmt.Sprintf("data %d", i))
		if err := store.Write(ctx, path, bytes.NewReader(data)); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}

	// Verify files exist
	files, err := store.List(ctx, "delete_test_")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("Expected 5 files, got %d", len(files))
	}

	// Delete all
	for _, f := range files {
		if err := store.Delete(ctx, f.Path); err != nil {
			t.Errorf("Delete(%s) error: %v", f.Path, err)
		}
	}

	// Verify all deleted
	files, err = store.List(ctx, "delete_test_")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 files after delete, got %d", len(files))
	}
}

func TestLocalStorage_Integration_ListWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Create files with different prefixes
	prefixes := map[string]int{
		"backup_2024_01_":   3,
		"backup_2024_02_":   2,
		"metadata_":         4,
	}

	for prefix, count := range prefixes {
		for i := 0; i < count; i++ {
			path := fmt.Sprintf("%s%d.txt", prefix, i)
			if err := store.Write(ctx, path, bytes.NewReader([]byte("data"))); err != nil {
				t.Fatalf("Write() error: %v", err)
			}
		}
	}

	// List with each prefix
	for prefix, expectedCount := range prefixes {
		files, err := store.List(ctx, prefix)
		if err != nil {
			t.Fatalf("List(%s) error: %v", prefix, err)
		}
		if len(files) != expectedCount {
			t.Errorf("List(%s) = %d files, want %d", prefix, len(files), expectedCount)
		}
	}

	// List all
	all, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("List('') error: %v", err)
	}
	expectedTotal := 0
	for _, c := range prefixes {
		expectedTotal += c
	}
	if len(all) != expectedTotal {
		t.Errorf("List all = %d, want %d", len(all), expectedTotal)
	}
}

func TestFactory_Integration_Create(t *testing.T) {
	tmpDir := t.TempDir()

	factory := NewFactory()

	// Test local storage creation
	store, err := factory.Create("local", tmpDir, nil)
	if err != nil {
		t.Fatalf("Create(local) error: %v", err)
	}

	// Verify it works
	ctx := context.Background()
	if err := store.Write(ctx, "test.txt", bytes.NewReader([]byte("test"))); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	exists, err := store.Exists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if !exists {
		t.Error("File should exist")
	}
}

func TestLocalStorage_Integration_FileInfo(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Write file
	data := []byte("test content for file info")
	if err := store.Write(ctx, "info_test.txt", bytes.NewReader(data)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Get file list
	files, err := store.List(ctx, "info_test")
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	info := files[0]

	// Verify FileInfo fields
	if info.Path != "info_test.txt" {
		t.Errorf("Path = %s, want info_test.txt", info.Path)
	}
	if info.Size != int64(len(data)) {
		t.Errorf("Size = %d, want %d", info.Size, len(data))
	}
	if info.LastModified.IsZero() {
		t.Error("LastModified is zero")
	}
	if time.Since(info.LastModified) > time.Minute {
		t.Error("LastModified seems too old")
	}
}

func TestLocalStorage_Integration_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx := context.Background()

	// Test various filename patterns
	filenames := []string{
		"backup_20240115_143045.dump",
		"backup-with-dashes.sql",
		"backup.with.dots.gz",
		"backup_v1.2.3.tar.gz",
	}

	for _, name := range filenames {
		data := []byte("content for " + name)
		if err := store.Write(ctx, name, bytes.NewReader(data)); err != nil {
			t.Errorf("Write(%s) error: %v", name, err)
			continue
		}

		exists, err := store.Exists(ctx, name)
		if err != nil {
			t.Errorf("Exists(%s) error: %v", name, err)
			continue
		}
		if !exists {
			t.Errorf("File %s should exist", name)
		}
	}
}

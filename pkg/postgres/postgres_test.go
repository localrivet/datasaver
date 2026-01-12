package postgres

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewBackupMetadata(t *testing.T) {
	meta := NewBackupMetadata("backup-001", "testdb", "localhost", "15.0")

	if meta.ID != "backup-001" {
		t.Errorf("ID = %v, want backup-001", meta.ID)
	}
	if meta.Database.Name != "testdb" {
		t.Errorf("Database.Name = %v, want testdb", meta.Database.Name)
	}
	if meta.Database.Host != "localhost" {
		t.Errorf("Database.Host = %v, want localhost", meta.Database.Host)
	}
	if meta.Database.Version != "15.0" {
		t.Errorf("Database.Version = %v, want 15.0", meta.Database.Version)
	}
	if meta.Type != "daily" {
		t.Errorf("Type = %v, want daily", meta.Type)
	}
	if meta.Backup.Method != "pg_dump" {
		t.Errorf("Backup.Method = %v, want pg_dump", meta.Backup.Method)
	}
	if meta.Backup.Format != "custom" {
		t.Errorf("Backup.Format = %v, want custom", meta.Backup.Format)
	}
	if meta.Backup.Compression != "gzip" {
		t.Errorf("Backup.Compression = %v, want gzip", meta.Backup.Compression)
	}
	if meta.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestBackupMetadata_SetBackupInfo(t *testing.T) {
	meta := NewBackupMetadata("backup-001", "testdb", "localhost", "15.0")

	duration := 5 * time.Second
	meta.SetBackupInfo(1024, 512, duration, "sha256:abc123")

	if meta.Backup.SizeBytes != 1024 {
		t.Errorf("SizeBytes = %v, want 1024", meta.Backup.SizeBytes)
	}
	if meta.Backup.CompressedSize != 512 {
		t.Errorf("CompressedSize = %v, want 512", meta.Backup.CompressedSize)
	}
	if meta.Backup.DurationSeconds != 5.0 {
		t.Errorf("DurationSeconds = %v, want 5.0", meta.Backup.DurationSeconds)
	}
	if meta.Backup.Checksum != "sha256:abc123" {
		t.Errorf("Checksum = %v, want sha256:abc123", meta.Backup.Checksum)
	}
}

func TestBackupMetadata_SetRetention(t *testing.T) {
	meta := NewBackupMetadata("backup-001", "testdb", "localhost", "15.0")

	keepUntil := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	meta.SetRetention(keepUntil, "monthly")

	if !meta.Retention.KeepUntil.Equal(keepUntil) {
		t.Errorf("KeepUntil = %v, want %v", meta.Retention.KeepUntil, keepUntil)
	}
	if meta.Retention.Policy != "monthly" {
		t.Errorf("Policy = %v, want monthly", meta.Retention.Policy)
	}
}

func TestBackupMetadata_AddFile(t *testing.T) {
	meta := NewBackupMetadata("backup-001", "testdb", "localhost", "15.0")

	if len(meta.Files) != 0 {
		t.Errorf("Files should be empty initially, got %d", len(meta.Files))
	}

	meta.AddFile("backup-001.dump")
	meta.AddFile("backup-001.meta.json")

	if len(meta.Files) != 2 {
		t.Errorf("Files length = %d, want 2", len(meta.Files))
	}
	if meta.Files[0] != "backup-001.dump" {
		t.Errorf("Files[0] = %v, want backup-001.dump", meta.Files[0])
	}
	if meta.Files[1] != "backup-001.meta.json" {
		t.Errorf("Files[1] = %v, want backup-001.meta.json", meta.Files[1])
	}
}

func TestBackupMetadata_ToJSON(t *testing.T) {
	meta := NewBackupMetadata("backup-001", "testdb", "localhost", "15.0")
	meta.AddFile("backup-001.dump")

	jsonData, err := meta.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("ToJSON() returned empty data")
	}

	// Verify it's valid JSON by parsing it back
	parsed, err := ParseMetadata(jsonData)
	if err != nil {
		t.Fatalf("ParseMetadata() error: %v", err)
	}

	if parsed.ID != meta.ID {
		t.Errorf("Parsed ID = %v, want %v", parsed.ID, meta.ID)
	}
}

func TestParseMetadata(t *testing.T) {
	jsonData := `{
		"id": "backup-001",
		"timestamp": "2024-01-15T12:00:00Z",
		"type": "daily",
		"database": {
			"name": "testdb",
			"host": "localhost",
			"version": "15.0"
		},
		"backup": {
			"method": "pg_dump",
			"format": "custom",
			"compression": "gzip",
			"size_bytes": 1024,
			"compressed_size_bytes": 512,
			"duration_seconds": 5.0,
			"checksum": "sha256:abc123"
		},
		"files": ["backup-001.dump", "backup-001.meta.json"],
		"retention": {
			"keep_until": "2024-12-31T00:00:00Z",
			"policy": "monthly"
		}
	}`

	meta, err := ParseMetadata([]byte(jsonData))
	if err != nil {
		t.Fatalf("ParseMetadata() error: %v", err)
	}

	if meta.ID != "backup-001" {
		t.Errorf("ID = %v, want backup-001", meta.ID)
	}
	if meta.Type != "daily" {
		t.Errorf("Type = %v, want daily", meta.Type)
	}
	if meta.Database.Name != "testdb" {
		t.Errorf("Database.Name = %v, want testdb", meta.Database.Name)
	}
	if meta.Backup.SizeBytes != 1024 {
		t.Errorf("Backup.SizeBytes = %v, want 1024", meta.Backup.SizeBytes)
	}
	if len(meta.Files) != 2 {
		t.Errorf("Files length = %d, want 2", len(meta.Files))
	}
	if meta.Retention.Policy != "monthly" {
		t.Errorf("Retention.Policy = %v, want monthly", meta.Retention.Policy)
	}
}

func TestParseMetadata_InvalidJSON(t *testing.T) {
	invalidJSON := `{invalid json{{{`

	_, err := ParseMetadata([]byte(invalidJSON))
	if err == nil {
		t.Error("ParseMetadata() should error with invalid JSON")
	}
}

func TestCalculateChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content for checksum")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	checksum, err := CalculateChecksum(testFile)
	if err != nil {
		t.Fatalf("CalculateChecksum() error: %v", err)
	}

	if checksum == "" {
		t.Error("CalculateChecksum() returned empty string")
	}

	// Should start with sha256: prefix
	if len(checksum) < 7 || checksum[:7] != "sha256:" {
		t.Errorf("Checksum should start with 'sha256:', got %v", checksum)
	}

	// Verify consistency - same file should produce same checksum
	checksum2, err := CalculateChecksum(testFile)
	if err != nil {
		t.Fatalf("CalculateChecksum() second call error: %v", err)
	}

	if checksum != checksum2 {
		t.Errorf("Checksum should be consistent, got %v and %v", checksum, checksum2)
	}
}

func TestCalculateChecksum_FileNotFound(t *testing.T) {
	_, err := CalculateChecksum("/nonexistent/file.txt")
	if err == nil {
		t.Error("CalculateChecksum() should error when file doesn't exist")
	}
}

func TestCalculateChecksum_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	os.WriteFile(file1, []byte("content 1"), 0644)
	os.WriteFile(file2, []byte("content 2"), 0644)

	checksum1, _ := CalculateChecksum(file1)
	checksum2, _ := CalculateChecksum(file2)

	if checksum1 == checksum2 {
		t.Error("Different content should produce different checksums")
	}
}

func TestGenerateBackupID(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

	id := GenerateBackupID(timestamp)

	expected := "backup_20240115_143045"
	if id != expected {
		t.Errorf("GenerateBackupID() = %v, want %v", id, expected)
	}
}

func TestGenerateBackupID_DifferentTimes(t *testing.T) {
	t1 := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 15, 12, 0, 1, 0, time.UTC)

	id1 := GenerateBackupID(t1)
	id2 := GenerateBackupID(t2)

	if id1 == id2 {
		t.Error("Different times should produce different IDs")
	}
}

func TestDumpOptions(t *testing.T) {
	opts := DumpOptions{
		Format:      "custom",
		Compression: "gzip",
		OutputPath:  "/tmp/backup.dump",
		Database:    "testdb",
		Host:        "localhost",
		Port:        5432,
		User:        "postgres",
		Password:    "secret",
	}

	if opts.Format != "custom" {
		t.Errorf("Format = %v, want custom", opts.Format)
	}
	if opts.Database != "testdb" {
		t.Errorf("Database = %v, want testdb", opts.Database)
	}
	if opts.Port != 5432 {
		t.Errorf("Port = %v, want 5432", opts.Port)
	}
}

func TestDatabaseMetadata(t *testing.T) {
	dbMeta := DatabaseMetadata{
		Name:    "testdb",
		Host:    "db.example.com",
		Version: "15.2",
	}

	if dbMeta.Name != "testdb" {
		t.Errorf("Name = %v, want testdb", dbMeta.Name)
	}
	if dbMeta.Host != "db.example.com" {
		t.Errorf("Host = %v, want db.example.com", dbMeta.Host)
	}
	if dbMeta.Version != "15.2" {
		t.Errorf("Version = %v, want 15.2", dbMeta.Version)
	}
}

func TestBackupInfo(t *testing.T) {
	info := BackupInfo{
		Method:          "pg_dump",
		Format:          "custom",
		Compression:     "gzip",
		SizeBytes:       1048576,
		CompressedSize:  524288,
		DurationSeconds: 10.5,
		Checksum:        "sha256:abc",
	}

	if info.Method != "pg_dump" {
		t.Errorf("Method = %v, want pg_dump", info.Method)
	}
	if info.SizeBytes != 1048576 {
		t.Errorf("SizeBytes = %v, want 1048576", info.SizeBytes)
	}
	if info.CompressedSize != 524288 {
		t.Errorf("CompressedSize = %v, want 524288", info.CompressedSize)
	}
}

func TestRetentionInfo(t *testing.T) {
	keepUntil := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	info := RetentionInfo{
		KeepUntil: keepUntil,
		Policy:    "weekly",
	}

	if !info.KeepUntil.Equal(keepUntil) {
		t.Errorf("KeepUntil = %v, want %v", info.KeepUntil, keepUntil)
	}
	if info.Policy != "weekly" {
		t.Errorf("Policy = %v, want weekly", info.Policy)
	}
}

func TestBackupMetadata_RoundTrip(t *testing.T) {
	// Create metadata with all fields populated
	original := NewBackupMetadata("backup-test", "mydb", "dbhost", "14.0")
	original.Type = "weekly"
	original.SetBackupInfo(2048, 1024, 15*time.Second, "sha256:xyz789")
	original.SetRetention(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC), "weekly")
	original.AddFile("backup-test.dump.gz")
	original.AddFile("backup-test.meta.json")

	// Serialize
	jsonData, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	// Deserialize
	parsed, err := ParseMetadata(jsonData)
	if err != nil {
		t.Fatalf("ParseMetadata() error: %v", err)
	}

	// Verify all fields
	if parsed.ID != original.ID {
		t.Errorf("ID mismatch: %v vs %v", parsed.ID, original.ID)
	}
	if parsed.Type != original.Type {
		t.Errorf("Type mismatch: %v vs %v", parsed.Type, original.Type)
	}
	if parsed.Database.Name != original.Database.Name {
		t.Errorf("Database.Name mismatch")
	}
	if parsed.Database.Host != original.Database.Host {
		t.Errorf("Database.Host mismatch")
	}
	if parsed.Backup.SizeBytes != original.Backup.SizeBytes {
		t.Errorf("Backup.SizeBytes mismatch")
	}
	if parsed.Backup.Checksum != original.Backup.Checksum {
		t.Errorf("Backup.Checksum mismatch")
	}
	if len(parsed.Files) != len(original.Files) {
		t.Errorf("Files length mismatch")
	}
	if parsed.Retention.Policy != original.Retention.Policy {
		t.Errorf("Retention.Policy mismatch")
	}
}

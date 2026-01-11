package postgres

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

type BackupMetadata struct {
	ID        string           `json:"id"`
	Timestamp time.Time        `json:"timestamp"`
	Type      string           `json:"type"`
	Database  DatabaseMetadata `json:"database"`
	Backup    BackupInfo       `json:"backup"`
	Files     []string         `json:"files"`
	Retention RetentionInfo    `json:"retention"`
}

type DatabaseMetadata struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Version string `json:"version"`
}

type BackupInfo struct {
	Method           string  `json:"method"`
	Format           string  `json:"format"`
	Compression      string  `json:"compression"`
	SizeBytes        int64   `json:"size_bytes"`
	CompressedSize   int64   `json:"compressed_size_bytes"`
	DurationSeconds  float64 `json:"duration_seconds"`
	Checksum         string  `json:"checksum"`
}

type RetentionInfo struct {
	KeepUntil time.Time `json:"keep_until"`
	Policy    string    `json:"policy"`
}

func NewBackupMetadata(id string, dbName, dbHost, dbVersion string) *BackupMetadata {
	return &BackupMetadata{
		ID:        id,
		Timestamp: time.Now().UTC(),
		Type:      "daily",
		Database: DatabaseMetadata{
			Name:    dbName,
			Host:    dbHost,
			Version: dbVersion,
		},
		Backup: BackupInfo{
			Method:      "pg_dump",
			Format:      "custom",
			Compression: "gzip",
		},
		Files: make([]string, 0),
	}
}

func (m *BackupMetadata) SetBackupInfo(sizeBytes, compressedSize int64, duration time.Duration, checksum string) {
	m.Backup.SizeBytes = sizeBytes
	m.Backup.CompressedSize = compressedSize
	m.Backup.DurationSeconds = duration.Seconds()
	m.Backup.Checksum = checksum
}

func (m *BackupMetadata) SetRetention(keepUntil time.Time, policy string) {
	m.Retention.KeepUntil = keepUntil
	m.Retention.Policy = policy
}

func (m *BackupMetadata) AddFile(filename string) {
	m.Files = append(m.Files, filename)
}

func (m *BackupMetadata) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func ParseMetadata(data []byte) (*BackupMetadata, error) {
	var meta BackupMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}
	return &meta, nil
}

func CalculateChecksum(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func GenerateBackupID(timestamp time.Time) string {
	return fmt.Sprintf("backup_%s", timestamp.Format("20060102_150405"))
}

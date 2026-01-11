package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type SQLiteDriver struct {
	path string
	db   *sql.DB
}

func NewSQLiteDriver(cfg Config) (*SQLiteDriver, error) {
	path := cfg.Path
	if path == "" {
		path = cfg.Name
	}

	if path == "" {
		return nil, fmt.Errorf("sqlite database path is required")
	}

	return &SQLiteDriver{
		path: path,
	}, nil
}

func (s *SQLiteDriver) Type() string {
	return "sqlite"
}

func (s *SQLiteDriver) Connect(ctx context.Context) error {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return fmt.Errorf("sqlite database file not found: %s", s.path)
	}

	db, err := sql.Open("sqlite", s.path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	s.db = db
	return nil
}

func (s *SQLiteDriver) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLiteDriver) Version(ctx context.Context) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("database not connected")
	}

	var version string
	err := s.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get sqlite version: %w", err)
	}

	return version, nil
}

func (s *SQLiteDriver) Dump(ctx context.Context, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "sqlite3", s.path, ".dump")
	cmd.Stdout = w
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sqlite3 dump failed: %w", err)
	}

	return nil
}

func (s *SQLiteDriver) DumpToFile(ctx context.Context, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	return s.Dump(ctx, f)
}

func (s *SQLiteDriver) CopyDatabase(ctx context.Context, destPath string) error {
	srcFile, err := os.Open(s.path)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy database: %w", err)
	}

	return destFile.Sync()
}

func (s *SQLiteDriver) Restore(ctx context.Context, r io.Reader, targetDB string) error {
	targetPath := targetDB
	if targetPath == "" {
		targetPath = s.path
	}

	tmpFile, err := os.CreateTemp("", "sqlite-restore-*.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, r); err != nil {
		return fmt.Errorf("failed to write restore data: %w", err)
	}
	tmpFile.Close()

	if _, err := os.Stat(targetPath); err == nil {
		backupPath := targetPath + ".bak"
		if err := os.Rename(targetPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing database: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "sqlite3", targetPath)
	sqlFile, err := os.Open(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open sql file: %w", err)
	}
	defer sqlFile.Close()

	cmd.Stdin = sqlFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sqlite3 restore failed: %w", err)
	}

	return nil
}

func (s *SQLiteDriver) Path() string {
	return s.path
}

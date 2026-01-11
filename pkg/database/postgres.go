package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	_ "github.com/lib/pq"
)

type PostgresDriver struct {
	cfg Config
	db  *sql.DB
}

func NewPostgresDriver(cfg Config) (*PostgresDriver, error) {
	return &PostgresDriver{
		cfg: cfg,
	}, nil
}

func (p *PostgresDriver) Type() string {
	return "postgres"
}

func (p *PostgresDriver) ConnectionString() string {
	if p.cfg.URL != "" {
		return p.cfg.URL
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		p.cfg.User, p.cfg.Password, p.cfg.Host, p.cfg.Port, p.cfg.Name)
}

func (p *PostgresDriver) Connect(ctx context.Context) error {
	db, err := sql.Open("postgres", p.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	p.db = db
	return nil
}

func (p *PostgresDriver) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *PostgresDriver) Version(ctx context.Context) (string, error) {
	if p.db == nil {
		return "", fmt.Errorf("database not connected")
	}

	var version string
	err := p.db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get postgres version: %w", err)
	}

	parts := strings.Fields(version)
	if len(parts) >= 2 {
		return parts[1], nil
	}
	return version, nil
}

func (p *PostgresDriver) Dump(ctx context.Context, w io.Writer) error {
	args := []string{
		"-h", p.cfg.Host,
		"-p", fmt.Sprintf("%d", p.cfg.Port),
		"-U", p.cfg.User,
		"-d", p.cfg.Name,
		"-F", "c",
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", p.cfg.Password))
	cmd.Stdout = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	return nil
}

func (p *PostgresDriver) DumpToFile(ctx context.Context, outputPath string) error {
	args := []string{
		"-h", p.cfg.Host,
		"-p", fmt.Sprintf("%d", p.cfg.Port),
		"-U", p.cfg.User,
		"-d", p.cfg.Name,
		"-F", "c",
		"-f", outputPath,
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", p.cfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (p *PostgresDriver) Restore(ctx context.Context, r io.Reader, targetDB string) error {
	dbName := targetDB
	if dbName == "" {
		dbName = p.cfg.Name
	}

	tmpFile, err := os.CreateTemp("", "pg-restore-*.dump")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, r); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write restore data: %w", err)
	}
	tmpFile.Close()

	args := []string{
		"-h", p.cfg.Host,
		"-p", fmt.Sprintf("%d", p.cfg.Port),
		"-U", p.cfg.User,
		"-d", dbName,
		tmpFile.Name(),
	}

	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", p.cfg.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_restore failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (p *PostgresDriver) Config() Config {
	return p.cfg
}

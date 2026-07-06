package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/url"
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
	return p.connString("")
}

// connString returns the libpq connection string, optionally overriding the
// database name. pg_dump/pg_restore receive this via -d: in URL-only setups
// the discrete Host/Port/User/Name fields are empty, so building CLI args
// from them silently targets nothing.
func (p *PostgresDriver) connString(dbName string) string {
	if p.cfg.URL != "" {
		if dbName == "" {
			return p.cfg.URL
		}
		u, err := url.Parse(p.cfg.URL)
		if err != nil {
			return p.cfg.URL
		}
		u.Path = "/" + dbName
		return u.String()
	}
	if dbName == "" {
		dbName = p.cfg.Name
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		p.cfg.User, url.QueryEscape(p.cfg.Password), p.cfg.Host, p.cfg.Port, dbName)
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
		"-d", p.connString(""),
		"-F", "c",
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Stdout = w
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w, output: %s", err, stderr.String())
	}

	return nil
}

func (p *PostgresDriver) DumpToFile(ctx context.Context, outputPath string) error {
	args := []string{
		"-d", p.connString(""),
		"-F", "c",
		"-f", outputPath,
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)

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
		"-d", p.connString(dbName),
		"--clean",          // Drop existing objects before restoring
		"--if-exists",      // Don't error if objects don't exist
		"--no-owner",       // Don't restore ownership
		"--no-privileges",  // Don't restore privileges
		tmpFile.Name(),
	}

	cmd := exec.CommandContext(ctx, "pg_restore", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_restore failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (p *PostgresDriver) Config() Config {
	return p.cfg
}

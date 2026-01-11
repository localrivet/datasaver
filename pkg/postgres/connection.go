package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"

	_ "github.com/lib/pq"
)

type Client struct {
	connString string
	db         *sql.DB
}

func NewClient(connString string) (*Client, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Client{
		connString: connString,
		db:         db,
	}, nil
}

func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *Client) GetVersion(ctx context.Context) (string, error) {
	var version string
	err := c.db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get postgres version: %w", err)
	}

	parts := strings.Fields(version)
	if len(parts) >= 2 {
		return parts[1], nil
	}
	return version, nil
}

func (c *Client) GetDatabaseSize(ctx context.Context, dbName string) (int64, error) {
	var size int64
	err := c.db.QueryRowContext(ctx,
		"SELECT pg_database_size($1)", dbName).Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("failed to get database size: %w", err)
	}
	return size, nil
}

func (c *Client) ConnectionString() string {
	return c.connString
}

type DumpOptions struct {
	Format      string
	Compression string
	OutputPath  string
	Database    string
	Host        string
	Port        int
	User        string
	Password    string
}

func Dump(ctx context.Context, opts DumpOptions) error {
	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		"-F", opts.Format,
		"-f", opts.OutputPath,
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("PGPASSWORD=%s", opts.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %w, output: %s", err, string(output))
	}

	return nil
}

func Restore(ctx context.Context, backupPath string, opts DumpOptions) error {
	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", opts.Port),
		"-U", opts.User,
		"-d", opts.Database,
		backupPath,
	}

	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("PGPASSWORD=%s", opts.Password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_restore failed: %w, output: %s", err, string(output))
	}

	return nil
}

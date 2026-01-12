//go:build integration

package database

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Integration test configuration - uses Docker PostgreSQL
var testPostgresConfig = Config{
	Type:     "postgres",
	Host:     getEnvOrDefault("TEST_PG_HOST", "localhost"),
	Port:     getEnvPortOrDefault("TEST_PG_PORT", 5434),
	Name:     getEnvOrDefault("TEST_PG_DB", "testdb"),
	User:     getEnvOrDefault("TEST_PG_USER", "testuser"),
	Password: getEnvOrDefault("TEST_PG_PASS", "testpass"),
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvPortOrDefault(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var port int
		fmt.Sscanf(val, "%d", &port)
		if port > 0 {
			return port
		}
	}
	return defaultVal
}

func waitForPostgres(t *testing.T, cfg Config, timeout time.Duration) error {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = db.PingContext(ctx)
		cancel()
		db.Close()

		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("postgres not ready after %v: %w", timeout, lastErr)
}

func TestPostgresDriver_Integration_Connect(t *testing.T) {
	if err := waitForPostgres(t, testPostgresConfig, 30*time.Second); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	driver, err := NewPostgresDriver(testPostgresConfig)
	if err != nil {
		t.Fatalf("NewPostgresDriver() error: %v", err)
	}

	ctx := context.Background()
	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	// Verify we can query
	version, err := driver.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}

	if version == "" {
		t.Error("Version() returned empty string")
	}
	t.Logf("Connected to PostgreSQL version: %s", version)
}

func TestPostgresDriver_Integration_DumpAndRestore(t *testing.T) {
	if err := waitForPostgres(t, testPostgresConfig, 30*time.Second); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	// Check pg_dump is available
	if _, err := os.Stat("/usr/bin/pg_dump"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/pg_dump"); os.IsNotExist(err) {
			if _, err := os.Stat("/opt/homebrew/bin/pg_dump"); os.IsNotExist(err) {
				t.Skip("pg_dump not found in common paths")
			}
		}
	}

	ctx := context.Background()

	// Setup: Create test data
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		testPostgresConfig.User, testPostgresConfig.Password,
		testPostgresConfig.Host, testPostgresConfig.Port, testPostgresConfig.Name)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create test table and insert data
	_, err = db.ExecContext(ctx, `
		DROP TABLE IF EXISTS backup_test;
		CREATE TABLE backup_test (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			value INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO backup_test (name, value) VALUES
			('test1', 100),
			('test2', 200),
			('test3', 300);
	`)
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Verify initial data
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM backup_test").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 3 {
		t.Fatalf("Expected 3 rows, got %d", count)
	}

	// Create driver and dump
	driver, err := NewPostgresDriver(testPostgresConfig)
	if err != nil {
		t.Fatalf("NewPostgresDriver() error: %v", err)
	}

	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	// Dump to buffer
	var dumpBuffer bytes.Buffer
	if err := driver.Dump(ctx, &dumpBuffer); err != nil {
		t.Fatalf("Dump() error: %v", err)
	}

	dumpSize := dumpBuffer.Len()
	if dumpSize == 0 {
		t.Fatal("Dump() produced empty output")
	}
	t.Logf("Dump size: %d bytes", dumpSize)

	// Verify dump contains our data (pg_dump custom format is binary)
	if dumpSize < 100 {
		t.Errorf("Dump seems too small: %d bytes", dumpSize)
	}

	// Delete the test data
	_, err = db.ExecContext(ctx, "DELETE FROM backup_test")
	if err != nil {
		t.Fatalf("Failed to delete test data: %v", err)
	}

	// Verify data is gone
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM backup_test").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 rows after delete, got %d", count)
	}

	// Restore from dump
	if err := driver.Restore(ctx, bytes.NewReader(dumpBuffer.Bytes()), ""); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}

	// Verify data is restored
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM backup_test").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows after restore: %v", err)
	}
	if count != 3 {
		t.Fatalf("Expected 3 rows after restore, got %d", count)
	}

	// Verify specific values
	var name string
	var value int
	err = db.QueryRowContext(ctx, "SELECT name, value FROM backup_test WHERE name = 'test2'").Scan(&name, &value)
	if err != nil {
		t.Fatalf("Failed to query restored data: %v", err)
	}
	if value != 200 {
		t.Errorf("Expected value 200 for test2, got %d", value)
	}

	t.Log("Dump and restore completed successfully with data integrity verified")
}

func TestPostgresDriver_Integration_DumpToFile(t *testing.T) {
	if err := waitForPostgres(t, testPostgresConfig, 30*time.Second); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	ctx := context.Background()

	driver, err := NewPostgresDriver(testPostgresConfig)
	if err != nil {
		t.Fatalf("NewPostgresDriver() error: %v", err)
	}

	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	// Dump to file
	tmpDir := t.TempDir()
	dumpPath := tmpDir + "/test.dump"

	if err := driver.DumpToFile(ctx, dumpPath); err != nil {
		t.Fatalf("DumpToFile() error: %v", err)
	}

	// Verify file exists and has content
	info, err := os.Stat(dumpPath)
	if err != nil {
		t.Fatalf("Dump file not created: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Dump file is empty")
	}

	t.Logf("Dump file created: %s (%d bytes)", dumpPath, info.Size())
}

func TestPostgresDriver_Integration_LargeDataset(t *testing.T) {
	if err := waitForPostgres(t, testPostgresConfig, 30*time.Second); err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}

	ctx := context.Background()

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		testPostgresConfig.User, testPostgresConfig.Password,
		testPostgresConfig.Host, testPostgresConfig.Port, testPostgresConfig.Name)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create table with larger dataset
	_, err = db.ExecContext(ctx, `
		DROP TABLE IF EXISTS large_backup_test;
		CREATE TABLE large_backup_test (
			id SERIAL PRIMARY KEY,
			data TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert 1000 rows
	for i := 0; i < 1000; i++ {
		_, err = db.ExecContext(ctx, "INSERT INTO large_backup_test (data) VALUES ($1)",
			fmt.Sprintf("test data row %d with some extra content to make it larger", i))
		if err != nil {
			t.Fatalf("Failed to insert row %d: %v", i, err)
		}
	}

	driver, err := NewPostgresDriver(testPostgresConfig)
	if err != nil {
		t.Fatalf("NewPostgresDriver() error: %v", err)
	}

	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	// Time the dump
	start := time.Now()
	var dumpBuffer bytes.Buffer
	if err := driver.Dump(ctx, &dumpBuffer); err != nil {
		t.Fatalf("Dump() error: %v", err)
	}
	dumpDuration := time.Since(start)

	t.Logf("Dumped 1000 rows in %v (%d bytes)", dumpDuration, dumpBuffer.Len())

	// Clear and restore
	_, err = db.ExecContext(ctx, "TRUNCATE large_backup_test")
	if err != nil {
		t.Fatalf("Failed to truncate: %v", err)
	}

	start = time.Now()
	if err := driver.Restore(ctx, bytes.NewReader(dumpBuffer.Bytes()), ""); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	restoreDuration := time.Since(start)

	t.Logf("Restored in %v", restoreDuration)

	// Verify count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM large_backup_test").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 1000 {
		t.Errorf("Expected 1000 rows, got %d", count)
	}
}

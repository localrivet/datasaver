//go:build integration

package database

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func createTestSQLiteDB(t *testing.T, path string) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("Failed to create SQLite database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			age INTEGER
		);
		CREATE TABLE IF NOT EXISTS orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			product TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			price REAL NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
		INSERT INTO users (name, email, age) VALUES
			('Alice', 'alice@example.com', 30),
			('Bob', 'bob@example.com', 25),
			('Charlie', 'charlie@example.com', 35);
		INSERT INTO orders (user_id, product, quantity, price) VALUES
			(1, 'Widget', 5, 9.99),
			(1, 'Gadget', 2, 24.99),
			(2, 'Widget', 10, 9.99),
			(3, 'Gizmo', 1, 149.99);
	`)
	if err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
}

func TestSQLiteDriver_Integration_Connect(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	createTestSQLiteDB(t, dbPath)

	driver, err := NewSQLiteDriver(Config{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		t.Fatalf("NewSQLiteDriver() error: %v", err)
	}

	ctx := context.Background()
	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	version, err := driver.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}

	if version == "" {
		t.Error("Version() returned empty string")
	}
	t.Logf("Connected to SQLite version: %s", version)
}

func TestSQLiteDriver_Integration_DumpAndRestore(t *testing.T) {
	// Check if sqlite3 CLI is available
	if _, err := os.Stat("/usr/bin/sqlite3"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/sqlite3"); os.IsNotExist(err) {
			if _, err := os.Stat("/opt/homebrew/bin/sqlite3"); os.IsNotExist(err) {
				t.Skip("sqlite3 CLI not found in common paths")
			}
		}
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "source.db")

	createTestSQLiteDB(t, dbPath)

	driver, err := NewSQLiteDriver(Config{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		t.Fatalf("NewSQLiteDriver() error: %v", err)
	}

	ctx := context.Background()
	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	// Dump to buffer
	var dumpBuffer bytes.Buffer
	if err := driver.Dump(ctx, &dumpBuffer); err != nil {
		t.Fatalf("Dump() error: %v", err)
	}

	dumpContent := dumpBuffer.String()
	if dumpContent == "" {
		t.Fatal("Dump() produced empty output")
	}

	t.Logf("Dump size: %d bytes", len(dumpContent))

	// Verify dump contains expected SQL
	if !bytes.Contains(dumpBuffer.Bytes(), []byte("CREATE TABLE")) {
		t.Error("Dump missing CREATE TABLE statements")
	}
	if !bytes.Contains(dumpBuffer.Bytes(), []byte("INSERT INTO")) {
		t.Error("Dump missing INSERT statements")
	}
	if !bytes.Contains(dumpBuffer.Bytes(), []byte("alice@example.com")) {
		t.Error("Dump missing test data")
	}

	// Create a new database and restore
	restoredPath := filepath.Join(tmpDir, "restored.db")

	if err := driver.Restore(ctx, bytes.NewReader(dumpBuffer.Bytes()), restoredPath); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}

	// Verify restored database
	restoredDB, err := sql.Open("sqlite", restoredPath)
	if err != nil {
		t.Fatalf("Failed to open restored database: %v", err)
	}
	defer restoredDB.Close()

	// Check user count
	var userCount int
	err = restoredDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	if userCount != 3 {
		t.Errorf("Expected 3 users, got %d", userCount)
	}

	// Check order count
	var orderCount int
	err = restoredDB.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orderCount)
	if err != nil {
		t.Fatalf("Failed to count orders: %v", err)
	}
	if orderCount != 4 {
		t.Errorf("Expected 4 orders, got %d", orderCount)
	}

	// Verify specific data
	var email string
	var age int
	err = restoredDB.QueryRow("SELECT email, age FROM users WHERE name = 'Bob'").Scan(&email, &age)
	if err != nil {
		t.Fatalf("Failed to query restored data: %v", err)
	}
	if email != "bob@example.com" {
		t.Errorf("Expected bob@example.com, got %s", email)
	}
	if age != 25 {
		t.Errorf("Expected age 25, got %d", age)
	}

	t.Log("SQLite dump and restore completed with data integrity verified")
}

func TestSQLiteDriver_Integration_CopyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.db")

	createTestSQLiteDB(t, srcPath)

	driver, err := NewSQLiteDriver(Config{
		Type: "sqlite",
		Path: srcPath,
	})
	if err != nil {
		t.Fatalf("NewSQLiteDriver() error: %v", err)
	}

	ctx := context.Background()
	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	// Copy database
	destPath := filepath.Join(tmpDir, "copy.db")
	if err := driver.CopyDatabase(ctx, destPath); err != nil {
		t.Fatalf("CopyDatabase() error: %v", err)
	}

	// Verify copy exists
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Copy not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Copy is empty")
	}

	// Verify copy is readable
	copyDB, err := sql.Open("sqlite", destPath)
	if err != nil {
		t.Fatalf("Failed to open copy: %v", err)
	}
	defer copyDB.Close()

	var count int
	err = copyDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query copy: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 users in copy, got %d", count)
	}

	t.Logf("Database copied successfully: %d bytes", info.Size())
}

func TestSQLiteDriver_Integration_DumpToFile(t *testing.T) {
	// Check if sqlite3 CLI is available
	if _, err := os.Stat("/usr/bin/sqlite3"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/sqlite3"); os.IsNotExist(err) {
			if _, err := os.Stat("/opt/homebrew/bin/sqlite3"); os.IsNotExist(err) {
				t.Skip("sqlite3 CLI not found in common paths")
			}
		}
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	createTestSQLiteDB(t, dbPath)

	driver, err := NewSQLiteDriver(Config{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		t.Fatalf("NewSQLiteDriver() error: %v", err)
	}

	ctx := context.Background()
	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	// Dump to file
	dumpPath := filepath.Join(tmpDir, "dump.sql")
	if err := driver.DumpToFile(ctx, dumpPath); err != nil {
		t.Fatalf("DumpToFile() error: %v", err)
	}

	// Verify file
	content, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatalf("Failed to read dump file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Dump file is empty")
	}

	if !bytes.Contains(content, []byte("CREATE TABLE")) {
		t.Error("Dump file missing CREATE TABLE")
	}

	t.Logf("Dump file created: %d bytes", len(content))
}

func TestSQLiteDriver_Integration_LargeDataset(t *testing.T) {
	// Check if sqlite3 CLI is available
	if _, err := os.Stat("/usr/bin/sqlite3"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/sqlite3"); os.IsNotExist(err) {
			if _, err := os.Stat("/opt/homebrew/bin/sqlite3"); os.IsNotExist(err) {
				t.Skip("sqlite3 CLI not found in common paths")
			}
		}
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "large.db")

	// Create database with many rows
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE large_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert 5000 rows
	tx, _ := db.Begin()
	stmt, _ := tx.Prepare("INSERT INTO large_data (data) VALUES (?)")
	for i := 0; i < 5000; i++ {
		stmt.Exec("This is test data row with some content to make it reasonably sized for testing purposes")
	}
	stmt.Close()
	tx.Commit()
	db.Close()

	driver, err := NewSQLiteDriver(Config{
		Type: "sqlite",
		Path: dbPath,
	})
	if err != nil {
		t.Fatalf("NewSQLiteDriver() error: %v", err)
	}

	ctx := context.Background()
	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer driver.Close()

	// Dump
	var dumpBuffer bytes.Buffer
	if err := driver.Dump(ctx, &dumpBuffer); err != nil {
		t.Fatalf("Dump() error: %v", err)
	}

	t.Logf("Dumped 5000 rows: %d bytes", dumpBuffer.Len())

	// Restore to new database
	restoredPath := filepath.Join(tmpDir, "restored_large.db")
	if err := driver.Restore(ctx, bytes.NewReader(dumpBuffer.Bytes()), restoredPath); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}

	// Verify count
	restoredDB, err := sql.Open("sqlite", restoredPath)
	if err != nil {
		t.Fatalf("Failed to open restored: %v", err)
	}
	defer restoredDB.Close()

	var count int
	err = restoredDB.QueryRow("SELECT COUNT(*) FROM large_data").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 5000 {
		t.Errorf("Expected 5000 rows, got %d", count)
	}

	t.Log("Large dataset backup/restore verified")
}

package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDriver(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		wantType    string
		wantErr     bool
		errContains string
	}{
		{
			name: "postgres explicit type",
			cfg: Config{
				Type:     "postgres",
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "user",
				Password: "pass",
			},
			wantType: "postgres",
			wantErr:  false,
		},
		{
			name: "postgresql alias",
			cfg: Config{
				Type: "postgresql",
				Host: "localhost",
				Port: 5432,
				Name: "testdb",
			},
			wantType: "postgres",
			wantErr:  false,
		},
		{
			name: "pg alias",
			cfg: Config{
				Type: "pg",
				Host: "localhost",
				Port: 5432,
				Name: "testdb",
			},
			wantType: "postgres",
			wantErr:  false,
		},
		{
			name: "empty type defaults to postgres",
			cfg: Config{
				Type: "",
				Host: "localhost",
				Port: 5432,
				Name: "testdb",
			},
			wantType: "postgres",
			wantErr:  false,
		},
		{
			name: "sqlite type",
			cfg: Config{
				Type: "sqlite",
				Path: "/tmp/test.db",
			},
			wantType: "sqlite",
			wantErr:  false,
		},
		{
			name: "sqlite3 alias",
			cfg: Config{
				Type: "sqlite3",
				Path: "/tmp/test.db",
			},
			wantType: "sqlite",
			wantErr:  false,
		},
		{
			name: "sqlite with name instead of path",
			cfg: Config{
				Type: "sqlite",
				Name: "/tmp/test.db",
			},
			wantType: "sqlite",
			wantErr:  false,
		},
		{
			name: "sqlite missing path",
			cfg: Config{
				Type: "sqlite",
			},
			wantErr:     true,
			errContains: "path is required",
		},
		{
			name: "unsupported database type",
			cfg: Config{
				Type: "mysql",
			},
			wantErr:     true,
			errContains: "unsupported database type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := NewDriver(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewDriver() expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewDriver() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NewDriver() unexpected error: %v", err)
				return
			}

			if driver.Type() != tt.wantType {
				t.Errorf("NewDriver().Type() = %v, want %v", driver.Type(), tt.wantType)
			}
		})
	}
}

func TestPostgresDriver_Type(t *testing.T) {
	driver, err := NewPostgresDriver(Config{
		Host: "localhost",
		Port: 5432,
		Name: "testdb",
	})
	if err != nil {
		t.Fatalf("NewPostgresDriver() error: %v", err)
	}

	if driver.Type() != "postgres" {
		t.Errorf("Type() = %v, want postgres", driver.Type())
	}
}

func TestPostgresDriver_ConnectionString(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "from components",
			cfg: Config{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "user",
				Password: "pass",
			},
			want: "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
		},
		{
			name: "from URL takes precedence",
			cfg: Config{
				URL:      "postgres://custom:url@host:1234/db",
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "user",
				Password: "pass",
			},
			want: "postgres://custom:url@host:1234/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, _ := NewPostgresDriver(tt.cfg)
			got := driver.ConnectionString()
			if got != tt.want {
				t.Errorf("ConnectionString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPostgresDriver_Config(t *testing.T) {
	cfg := Config{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "user",
		Password: "pass",
	}
	driver, _ := NewPostgresDriver(cfg)

	got := driver.Config()
	if got.Host != cfg.Host || got.Port != cfg.Port || got.Name != cfg.Name {
		t.Errorf("Config() = %v, want %v", got, cfg)
	}
}

func TestPostgresDriver_Close_NilDB(t *testing.T) {
	driver, _ := NewPostgresDriver(Config{Host: "localhost"})
	err := driver.Close()
	if err != nil {
		t.Errorf("Close() with nil db should not error, got: %v", err)
	}
}

func TestPostgresDriver_Version_NotConnected(t *testing.T) {
	driver, _ := NewPostgresDriver(Config{Host: "localhost"})
	_, err := driver.Version(context.Background())
	if err == nil {
		t.Error("Version() should error when not connected")
	}
	if !contains(err.Error(), "not connected") {
		t.Errorf("Version() error = %v, want error containing 'not connected'", err)
	}
}

func TestSQLiteDriver_Type(t *testing.T) {
	driver, err := NewSQLiteDriver(Config{Path: "/tmp/test.db"})
	if err != nil {
		t.Fatalf("NewSQLiteDriver() error: %v", err)
	}

	if driver.Type() != "sqlite" {
		t.Errorf("Type() = %v, want sqlite", driver.Type())
	}
}

func TestSQLiteDriver_Path(t *testing.T) {
	path := "/tmp/test.db"
	driver, _ := NewSQLiteDriver(Config{Path: path})

	if driver.Path() != path {
		t.Errorf("Path() = %v, want %v", driver.Path(), path)
	}
}

func TestSQLiteDriver_PathFromName(t *testing.T) {
	path := "/tmp/test.db"
	driver, _ := NewSQLiteDriver(Config{Name: path})

	if driver.Path() != path {
		t.Errorf("Path() = %v, want %v", driver.Path(), path)
	}
}

func TestSQLiteDriver_Connect_FileNotFound(t *testing.T) {
	driver, _ := NewSQLiteDriver(Config{Path: "/nonexistent/path/to/db.sqlite"})
	err := driver.Connect(context.Background())
	if err == nil {
		t.Error("Connect() should error when file doesn't exist")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Connect() error = %v, want error containing 'not found'", err)
	}
}

func TestSQLiteDriver_Close_NilDB(t *testing.T) {
	driver, _ := NewSQLiteDriver(Config{Path: "/tmp/test.db"})
	err := driver.Close()
	if err != nil {
		t.Errorf("Close() with nil db should not error, got: %v", err)
	}
}

func TestSQLiteDriver_Version_NotConnected(t *testing.T) {
	driver, _ := NewSQLiteDriver(Config{Path: "/tmp/test.db"})
	_, err := driver.Version(context.Background())
	if err == nil {
		t.Error("Version() should error when not connected")
	}
	if !contains(err.Error(), "not connected") {
		t.Errorf("Version() error = %v, want error containing 'not connected'", err)
	}
}

func TestSQLiteDriver_ConnectAndVersion(t *testing.T) {
	// Create a temporary SQLite database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create the database file with a table
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test db: %v", err)
	}
	f.Close()

	// Open with sqlite to initialize
	driver, err := NewSQLiteDriver(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("NewSQLiteDriver() error: %v", err)
	}
	defer driver.Close()

	ctx := context.Background()
	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	version, err := driver.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}

	if version == "" {
		t.Error("Version() returned empty string")
	}
}

func TestSQLiteDriver_CopyDatabase(t *testing.T) {
	// Create a temporary SQLite database with some content
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.db")
	destPath := filepath.Join(tmpDir, "subdir", "dest.db")

	// Create source file
	if err := os.WriteFile(srcPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	driver, err := NewSQLiteDriver(Config{Path: srcPath})
	if err != nil {
		t.Fatalf("NewSQLiteDriver() error: %v", err)
	}

	ctx := context.Background()
	if err := driver.CopyDatabase(ctx, destPath); err != nil {
		t.Fatalf("CopyDatabase() error: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("CopyDatabase() did not create destination file")
	}

	// Verify content matches
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}
	if string(destContent) != "test content" {
		t.Errorf("CopyDatabase() content mismatch, got %q", destContent)
	}
}

func TestSQLiteDriver_CopyDatabase_SourceNotFound(t *testing.T) {
	driver, _ := NewSQLiteDriver(Config{Path: "/nonexistent/source.db"})
	err := driver.CopyDatabase(context.Background(), "/tmp/dest.db")
	if err == nil {
		t.Error("CopyDatabase() should error when source doesn't exist")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

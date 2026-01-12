package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear environment
	clearEnv()
	defer clearEnv()

	// Set minimum required config
	os.Setenv("DATASAVER_DB_NAME", "testdb")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Check defaults
	if cfg.Database.Type != "postgres" {
		t.Errorf("Database.Type = %v, want postgres", cfg.Database.Type)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %v, want localhost", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %v, want 5432", cfg.Database.Port)
	}
	if cfg.Schedule != "0 2 * * *" {
		t.Errorf("Schedule = %v, want 0 2 * * *", cfg.Schedule)
	}
	if cfg.Compression != "gzip" {
		t.Errorf("Compression = %v, want gzip", cfg.Compression)
	}
	if cfg.Storage.Backend != "local" {
		t.Errorf("Storage.Backend = %v, want local", cfg.Storage.Backend)
	}
	if cfg.Storage.Path != "/backups" {
		t.Errorf("Storage.Path = %v, want /backups", cfg.Storage.Path)
	}
	if cfg.Retention.Daily != 7 {
		t.Errorf("Retention.Daily = %v, want 7", cfg.Retention.Daily)
	}
	if cfg.Retention.Weekly != 4 {
		t.Errorf("Retention.Weekly = %v, want 4", cfg.Retention.Weekly)
	}
	if cfg.Retention.Monthly != 6 {
		t.Errorf("Retention.Monthly = %v, want 6", cfg.Retention.Monthly)
	}
	if cfg.Retention.MaxAgeDays != 90 {
		t.Errorf("Retention.MaxAgeDays = %v, want 90", cfg.Retention.MaxAgeDays)
	}
	if cfg.Monitoring.MetricsPort != 9090 {
		t.Errorf("Monitoring.MetricsPort = %v, want 9090", cfg.Monitoring.MetricsPort)
	}
	if cfg.Monitoring.HealthPort != 8080 {
		t.Errorf("Monitoring.HealthPort = %v, want 8080", cfg.Monitoring.HealthPort)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	clearEnv()
	defer clearEnv()

	// Set all environment variables
	os.Setenv("DATASAVER_DB_TYPE", "postgres")
	os.Setenv("DATASAVER_DB_HOST", "db.example.com")
	os.Setenv("DATASAVER_DB_PORT", "5433")
	os.Setenv("DATASAVER_DB_NAME", "proddb")
	os.Setenv("DATASAVER_DB_USER", "admin")
	os.Setenv("DATASAVER_DB_PASSWORD", "secret123")
	os.Setenv("DATASAVER_SCHEDULE", "0 3 * * *")
	os.Setenv("DATASAVER_STORAGE_BACKEND", "local")
	os.Setenv("DATASAVER_STORAGE_PATH", "/data/backups")
	os.Setenv("DATASAVER_KEEP_DAILY", "14")
	os.Setenv("DATASAVER_KEEP_WEEKLY", "8")
	os.Setenv("DATASAVER_KEEP_MONTHLY", "12")
	os.Setenv("DATASAVER_MAX_AGE_DAYS", "180")
	os.Setenv("DATASAVER_COMPRESSION", "none")
	os.Setenv("DATASAVER_METRICS_PORT", "9191")
	os.Setenv("DATASAVER_HEALTH_PORT", "8181")
	os.Setenv("DATASAVER_WEBHOOK_URL", "https://hooks.example.com")
	os.Setenv("DATASAVER_ALERT_AFTER_HOURS", "48")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.Type != "postgres" {
		t.Errorf("Database.Type = %v, want postgres", cfg.Database.Type)
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %v, want db.example.com", cfg.Database.Host)
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("Database.Port = %v, want 5433", cfg.Database.Port)
	}
	if cfg.Database.Name != "proddb" {
		t.Errorf("Database.Name = %v, want proddb", cfg.Database.Name)
	}
	if cfg.Database.User != "admin" {
		t.Errorf("Database.User = %v, want admin", cfg.Database.User)
	}
	if cfg.Database.Password != "secret123" {
		t.Errorf("Database.Password = %v, want secret123", cfg.Database.Password)
	}
	if cfg.Schedule != "0 3 * * *" {
		t.Errorf("Schedule = %v, want 0 3 * * *", cfg.Schedule)
	}
	if cfg.Storage.Path != "/data/backups" {
		t.Errorf("Storage.Path = %v, want /data/backups", cfg.Storage.Path)
	}
	if cfg.Retention.Daily != 14 {
		t.Errorf("Retention.Daily = %v, want 14", cfg.Retention.Daily)
	}
	if cfg.Retention.Weekly != 8 {
		t.Errorf("Retention.Weekly = %v, want 8", cfg.Retention.Weekly)
	}
	if cfg.Retention.Monthly != 12 {
		t.Errorf("Retention.Monthly = %v, want 12", cfg.Retention.Monthly)
	}
	if cfg.Retention.MaxAgeDays != 180 {
		t.Errorf("Retention.MaxAgeDays = %v, want 180", cfg.Retention.MaxAgeDays)
	}
	if cfg.Compression != "none" {
		t.Errorf("Compression = %v, want none", cfg.Compression)
	}
	if cfg.Monitoring.MetricsPort != 9191 {
		t.Errorf("Monitoring.MetricsPort = %v, want 9191", cfg.Monitoring.MetricsPort)
	}
	if cfg.Monitoring.HealthPort != 8181 {
		t.Errorf("Monitoring.HealthPort = %v, want 8181", cfg.Monitoring.HealthPort)
	}
	if cfg.Monitoring.WebhookURL != "https://hooks.example.com" {
		t.Errorf("Monitoring.WebhookURL = %v, want https://hooks.example.com", cfg.Monitoring.WebhookURL)
	}
	if cfg.Monitoring.AlertAfterHours != 48 {
		t.Errorf("Monitoring.AlertAfterHours = %v, want 48", cfg.Monitoring.AlertAfterHours)
	}
}

func TestLoad_FromFile(t *testing.T) {
	clearEnv()
	defer clearEnv()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
database:
  type: postgres
  host: filedb.example.com
  port: 5434
  name: filedb
  user: fileuser
  password: filepass

schedule: "0 4 * * *"
compression: gzip

storage:
  backend: local
  path: /file/backups

retention:
  daily: 10
  weekly: 5
  monthly: 3
  max_age_days: 60
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.Host != "filedb.example.com" {
		t.Errorf("Database.Host = %v, want filedb.example.com", cfg.Database.Host)
	}
	if cfg.Database.Port != 5434 {
		t.Errorf("Database.Port = %v, want 5434", cfg.Database.Port)
	}
	if cfg.Database.Name != "filedb" {
		t.Errorf("Database.Name = %v, want filedb", cfg.Database.Name)
	}
	if cfg.Schedule != "0 4 * * *" {
		t.Errorf("Schedule = %v, want 0 4 * * *", cfg.Schedule)
	}
	if cfg.Storage.Path != "/file/backups" {
		t.Errorf("Storage.Path = %v, want /file/backups", cfg.Storage.Path)
	}
	if cfg.Retention.Daily != 10 {
		t.Errorf("Retention.Daily = %v, want 10", cfg.Retention.Daily)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	clearEnv()
	defer clearEnv()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
database:
  type: postgres
  host: filehost
  name: filedb
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Environment should override file
	os.Setenv("DATASAVER_DB_HOST", "envhost")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.Host != "envhost" {
		t.Errorf("Database.Host = %v, want envhost (env should override file)", cfg.Database.Host)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	clearEnv()
	defer clearEnv()

	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() should error when file doesn't exist")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	clearEnv()
	defer clearEnv()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	invalidContent := `
database:
  type: [invalid yaml{{{
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() should error with invalid YAML")
	}
}

func TestLoad_Validation_PostgresNoDB(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_TYPE", "postgres")
	// No database name or URL set

	_, err := Load("")
	if err == nil {
		t.Error("Load() should error when postgres has no database name")
	}
}

func TestLoad_Validation_SQLiteNoPath(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_TYPE", "sqlite")
	// No path set

	_, err := Load("")
	if err == nil {
		t.Error("Load() should error when sqlite has no path")
	}
}

func TestLoad_Validation_UnsupportedDBType(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_TYPE", "mysql")
	os.Setenv("DATASAVER_DB_NAME", "testdb")

	_, err := Load("")
	if err == nil {
		t.Error("Load() should error for unsupported database type")
	}
}

func TestLoad_Validation_InvalidStorageBackend(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_NAME", "testdb")
	os.Setenv("DATASAVER_STORAGE_BACKEND", "gcs")

	_, err := Load("")
	if err == nil {
		t.Error("Load() should error for invalid storage backend")
	}
}

func TestLoad_Validation_S3MissingBucket(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_NAME", "testdb")
	os.Setenv("DATASAVER_STORAGE_BACKEND", "s3")
	os.Setenv("DATASAVER_S3_ACCESS_KEY", "access")
	os.Setenv("DATASAVER_S3_SECRET_KEY", "secret")
	// No bucket set

	_, err := Load("")
	if err == nil {
		t.Error("Load() should error when S3 bucket is missing")
	}
}

func TestLoad_Validation_S3MissingCredentials(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_NAME", "testdb")
	os.Setenv("DATASAVER_STORAGE_BACKEND", "s3")
	os.Setenv("DATASAVER_S3_BUCKET", "my-bucket")
	// No credentials set

	_, err := Load("")
	if err == nil {
		t.Error("Load() should error when S3 credentials are missing")
	}
}

func TestLoad_Validation_InvalidCompression(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_NAME", "testdb")
	os.Setenv("DATASAVER_COMPRESSION", "lz4")

	_, err := Load("")
	if err == nil {
		t.Error("Load() should error for invalid compression type")
	}
}

func TestLoad_S3Config(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_NAME", "testdb")
	os.Setenv("DATASAVER_STORAGE_BACKEND", "s3")
	os.Setenv("DATASAVER_S3_BUCKET", "my-bucket")
	os.Setenv("DATASAVER_S3_ENDPOINT", "s3.example.com")
	os.Setenv("DATASAVER_S3_REGION", "us-west-2")
	os.Setenv("DATASAVER_S3_ACCESS_KEY", "AKIAIOSFODNN7EXAMPLE")
	os.Setenv("DATASAVER_S3_SECRET_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	os.Setenv("DATASAVER_S3_USE_SSL", "true")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Storage.S3.Bucket != "my-bucket" {
		t.Errorf("S3.Bucket = %v, want my-bucket", cfg.Storage.S3.Bucket)
	}
	if cfg.Storage.S3.Endpoint != "s3.example.com" {
		t.Errorf("S3.Endpoint = %v, want s3.example.com", cfg.Storage.S3.Endpoint)
	}
	if cfg.Storage.S3.Region != "us-west-2" {
		t.Errorf("S3.Region = %v, want us-west-2", cfg.Storage.S3.Region)
	}
	if cfg.Storage.S3.AccessKey != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("S3.AccessKey mismatch")
	}
	if !cfg.Storage.S3.UseSSL {
		t.Error("S3.UseSSL = false, want true")
	}
}

func TestLoad_SQLiteConfig(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DB_TYPE", "sqlite")
	os.Setenv("DATASAVER_DB_PATH", "/data/mydb.sqlite")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.Type != "sqlite" {
		t.Errorf("Database.Type = %v, want sqlite", cfg.Database.Type)
	}
	if cfg.Database.Path != "/data/mydb.sqlite" {
		t.Errorf("Database.Path = %v, want /data/mydb.sqlite", cfg.Database.Path)
	}
}

func TestLoad_DatabaseURL(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("DATASAVER_DATABASE_URL", "postgres://user:pass@host:5432/dbname")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.URL != "postgres://user:pass@host:5432/dbname" {
		t.Errorf("Database.URL mismatch")
	}
}

func TestDatabaseConfig_ConnectionString(t *testing.T) {
	tests := []struct {
		name string
		cfg  DatabaseConfig
		want string
	}{
		{
			name: "from URL",
			cfg: DatabaseConfig{
				URL: "postgres://custom:url@customhost:1234/customdb",
			},
			want: "postgres://custom:url@customhost:1234/customdb",
		},
		{
			name: "from components",
			cfg: DatabaseConfig{
				User:     "user",
				Password: "pass",
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
			},
			want: "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ConnectionString()
			if got != tt.want {
				t.Errorf("ConnectionString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_AlertDuration(t *testing.T) {
	cfg := &Config{
		Monitoring: MonitoringConfig{
			AlertAfterHours: 24,
		},
	}

	expected := 24 * time.Hour
	got := cfg.AlertDuration()

	if got != expected {
		t.Errorf("AlertDuration() = %v, want %v", got, expected)
	}
}

func TestConfig_IsSQLite(t *testing.T) {
	tests := []struct {
		dbType string
		want   bool
	}{
		{"sqlite", true},
		{"sqlite3", true},
		{"SQLITE", true},
		{"SQLite3", true},
		{"postgres", false},
		{"postgresql", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			cfg := &Config{Database: DatabaseConfig{Type: tt.dbType}}
			got := cfg.IsSQLite()
			if got != tt.want {
				t.Errorf("IsSQLite() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_IsPostgres(t *testing.T) {
	tests := []struct {
		dbType string
		want   bool
	}{
		{"postgres", true},
		{"postgresql", true},
		{"pg", true},
		{"POSTGRES", true},
		{"PostgreSQL", true},
		{"", true}, // Empty defaults to postgres
		{"sqlite", false},
		{"mysql", false},
	}

	for _, tt := range tests {
		t.Run(tt.dbType, func(t *testing.T) {
			cfg := &Config{Database: DatabaseConfig{Type: tt.dbType}}
			got := cfg.IsPostgres()
			if got != tt.want {
				t.Errorf("IsPostgres() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad_EnvExpansionInFile(t *testing.T) {
	clearEnv()
	defer clearEnv()

	os.Setenv("MY_DB_PASSWORD", "secret-from-env")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
database:
  type: postgres
  host: localhost
  name: testdb
  password: ${MY_DB_PASSWORD}
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Database.Password != "secret-from-env" {
		t.Errorf("Database.Password = %v, want secret-from-env (from env expansion)", cfg.Database.Password)
	}
}

func clearEnv() {
	envVars := []string{
		"DATASAVER_DB_TYPE",
		"DATASAVER_DATABASE_URL",
		"DATASAVER_DB_HOST",
		"DATASAVER_DB_PORT",
		"DATASAVER_DB_NAME",
		"DATASAVER_DB_USER",
		"DATASAVER_DB_PASSWORD",
		"DATASAVER_DB_PATH",
		"DATASAVER_SCHEDULE",
		"DATASAVER_STORAGE_BACKEND",
		"DATASAVER_STORAGE_PATH",
		"DATASAVER_S3_BUCKET",
		"DATASAVER_S3_ENDPOINT",
		"DATASAVER_S3_REGION",
		"DATASAVER_S3_ACCESS_KEY",
		"DATASAVER_S3_SECRET_KEY",
		"DATASAVER_S3_USE_SSL",
		"DATASAVER_KEEP_DAILY",
		"DATASAVER_KEEP_WEEKLY",
		"DATASAVER_KEEP_MONTHLY",
		"DATASAVER_MAX_AGE_DAYS",
		"DATASAVER_COMPRESSION",
		"DATASAVER_METRICS_PORT",
		"DATASAVER_HEALTH_PORT",
		"DATASAVER_WEBHOOK_URL",
		"DATASAVER_ALERT_AFTER_HOURS",
		"MY_DB_PASSWORD",
	}

	for _, v := range envVars {
		os.Unsetenv(v)
	}
}

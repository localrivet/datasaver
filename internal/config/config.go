package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database    DatabaseConfig    `yaml:"database"`
	Schedule    string            `yaml:"schedule"`
	Storage     StorageConfig     `yaml:"storage"`
	Retention   RetentionConfig   `yaml:"retention"`
	Compression string            `yaml:"compression"`
	Monitoring  MonitoringConfig  `yaml:"monitoring"`
	Backup      BackupConfig      `yaml:"backup"`
}

type BackupConfig struct {
	VerifyAfterBackup bool `yaml:"verify_after_backup"` // Restore to temp DB to verify backup integrity
	VerifyChecksum    bool `yaml:"verify_checksum"`     // Verify checksum on restore
}

type DatabaseConfig struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	URL      string `yaml:"url"`
	Path     string `yaml:"path"`
}

func (d *DatabaseConfig) ConnectionString() string {
	if d.URL != "" {
		return d.URL
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		d.User, d.Password, d.Host, d.Port, d.Name)
}

type StorageConfig struct {
	Backend string       `yaml:"backend"`
	Path    string       `yaml:"path"`
	S3      S3Config     `yaml:"s3"`
}

type S3Config struct {
	Bucket    string `yaml:"bucket"`
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	UseSSL    bool   `yaml:"use_ssl"`
}

type RetentionConfig struct {
	Daily      int `yaml:"daily"`
	Weekly     int `yaml:"weekly"`
	Monthly    int `yaml:"monthly"`
	MaxAgeDays int `yaml:"max_age_days"`
}

type MonitoringConfig struct {
	MetricsPort     int           `yaml:"metrics_port"`
	WebhookURL      string        `yaml:"webhook_url"`
	AlertAfterHours int           `yaml:"alert_after_hours"`
	HealthPort      int           `yaml:"health_port"`
}

func Load(configPath string) (*Config, error) {
	cfg := &Config{
		Database: DatabaseConfig{
			Type: "postgres",
			Host: "localhost",
			Port: 5432,
		},
		Schedule:    "0 2 * * *",
		Compression: "gzip",
		Storage: StorageConfig{
			Backend: "local",
			Path:    "/backups",
		},
		Retention: RetentionConfig{
			Daily:      7,
			Weekly:     4,
			Monthly:    6,
			MaxAgeDays: 90,
		},
		Monitoring: MonitoringConfig{
			MetricsPort:     9090,
			HealthPort:      8080,
			AlertAfterHours: 26,
		},
	}

	if configPath != "" {
		if err := cfg.loadFromFile(configPath); err != nil {
			return nil, err
		}
	}

	cfg.loadFromEnv()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	if err := yaml.Unmarshal([]byte(expanded), c); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

func (c *Config) loadFromEnv() {
	if v := os.Getenv("DATASAVER_DB_TYPE"); v != "" {
		c.Database.Type = v
	}
	if v := os.Getenv("DATASAVER_DATABASE_URL"); v != "" {
		c.Database.URL = v
	}
	if v := os.Getenv("DATASAVER_DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("DATASAVER_DB_PATH"); v != "" {
		c.Database.Path = v
	}
	if v := os.Getenv("DATASAVER_DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Database.Port = port
		}
	}
	if v := os.Getenv("DATASAVER_DB_NAME"); v != "" {
		c.Database.Name = v
	}
	if v := os.Getenv("DATASAVER_DB_USER"); v != "" {
		c.Database.User = v
	}
	if v := os.Getenv("DATASAVER_DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}

	if v := os.Getenv("DATASAVER_SCHEDULE"); v != "" {
		c.Schedule = v
	}

	if v := os.Getenv("DATASAVER_STORAGE_BACKEND"); v != "" {
		c.Storage.Backend = v
	}
	if v := os.Getenv("DATASAVER_STORAGE_PATH"); v != "" {
		c.Storage.Path = v
	}

	if v := os.Getenv("DATASAVER_S3_BUCKET"); v != "" {
		c.Storage.S3.Bucket = v
	}
	if v := os.Getenv("DATASAVER_S3_ENDPOINT"); v != "" {
		c.Storage.S3.Endpoint = v
	}
	if v := os.Getenv("DATASAVER_S3_REGION"); v != "" {
		c.Storage.S3.Region = v
	}
	if v := os.Getenv("DATASAVER_S3_ACCESS_KEY"); v != "" {
		c.Storage.S3.AccessKey = v
	}
	if v := os.Getenv("DATASAVER_S3_SECRET_KEY"); v != "" {
		c.Storage.S3.SecretKey = v
	}
	if v := os.Getenv("DATASAVER_S3_USE_SSL"); v != "" {
		c.Storage.S3.UseSSL = strings.ToLower(v) == "true"
	}

	if v := os.Getenv("DATASAVER_KEEP_DAILY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Retention.Daily = n
		}
	}
	if v := os.Getenv("DATASAVER_KEEP_WEEKLY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Retention.Weekly = n
		}
	}
	if v := os.Getenv("DATASAVER_KEEP_MONTHLY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Retention.Monthly = n
		}
	}
	if v := os.Getenv("DATASAVER_MAX_AGE_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Retention.MaxAgeDays = n
		}
	}

	if v := os.Getenv("DATASAVER_COMPRESSION"); v != "" {
		c.Compression = v
	}

	if v := os.Getenv("DATASAVER_METRICS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Monitoring.MetricsPort = port
		}
	}
	if v := os.Getenv("DATASAVER_HEALTH_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Monitoring.HealthPort = port
		}
	}
	if v := os.Getenv("DATASAVER_WEBHOOK_URL"); v != "" {
		c.Monitoring.WebhookURL = v
	}
	if v := os.Getenv("DATASAVER_ALERT_AFTER_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Monitoring.AlertAfterHours = n
		}
	}

	if v := os.Getenv("DATASAVER_VERIFY_BACKUP"); v != "" {
		c.Backup.VerifyAfterBackup = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("DATASAVER_VERIFY_CHECKSUM"); v != "" {
		c.Backup.VerifyChecksum = strings.ToLower(v) == "true"
	}
}

func (c *Config) validate() error {
	dbType := strings.ToLower(c.Database.Type)
	if dbType == "" {
		dbType = "postgres"
	}

	switch dbType {
	case "postgres", "postgresql", "pg":
		if c.Database.URL == "" && c.Database.Name == "" {
			return fmt.Errorf("database name or URL is required for PostgreSQL")
		}
	case "sqlite", "sqlite3":
		if c.Database.Path == "" && c.Database.Name == "" {
			return fmt.Errorf("database path is required for SQLite")
		}
	default:
		return fmt.Errorf("unsupported database type: %s (supported: postgres, sqlite)", c.Database.Type)
	}

	if c.Storage.Backend != "local" && c.Storage.Backend != "s3" {
		return fmt.Errorf("storage backend must be 'local' or 's3'")
	}

	if c.Storage.Backend == "s3" {
		if c.Storage.S3.Bucket == "" {
			return fmt.Errorf("S3 bucket is required when using S3 storage")
		}
		if c.Storage.S3.AccessKey == "" || c.Storage.S3.SecretKey == "" {
			return fmt.Errorf("S3 access key and secret key are required")
		}
	}

	if c.Compression != "gzip" && c.Compression != "zstd" && c.Compression != "none" {
		return fmt.Errorf("compression must be 'gzip', 'zstd', or 'none'")
	}

	return nil
}

func (c *Config) AlertDuration() time.Duration {
	return time.Duration(c.Monitoring.AlertAfterHours) * time.Hour
}

func (c *Config) IsSQLite() bool {
	t := strings.ToLower(c.Database.Type)
	return t == "sqlite" || t == "sqlite3"
}

func (c *Config) IsPostgres() bool {
	t := strings.ToLower(c.Database.Type)
	return t == "" || t == "postgres" || t == "postgresql" || t == "pg"
}

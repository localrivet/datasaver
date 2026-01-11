package database

import "fmt"

func NewDriver(cfg Config) (Driver, error) {
	switch cfg.Type {
	case "postgres", "postgresql", "pg", "":
		if cfg.Type == "" {
			cfg.Type = "postgres"
		}
		return NewPostgresDriver(cfg)
	case "sqlite", "sqlite3":
		return NewSQLiteDriver(cfg)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}

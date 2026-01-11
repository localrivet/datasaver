package database

import (
	"context"
	"io"
)

type Driver interface {
	Type() string
	Connect(ctx context.Context) error
	Close() error
	Version(ctx context.Context) (string, error)
	Dump(ctx context.Context, w io.Writer) error
	Restore(ctx context.Context, r io.Reader, targetDB string) error
}

type Config struct {
	Type     string
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	URL      string
	Path     string // For SQLite file path
}

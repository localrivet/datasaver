package storage

import (
	"context"
	"io"
	"time"
)

type Backend interface {
	Write(ctx context.Context, path string, reader io.Reader) error
	Read(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, prefix string) ([]FileInfo, error)
	Exists(ctx context.Context, path string) (bool, error)
	Size(ctx context.Context, path string) (int64, error)
}

type FileInfo struct {
	Path         string
	Size         int64
	LastModified time.Time
	IsDir        bool
}

type Factory struct{}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) Create(backend, path string, s3Config *S3Config) (Backend, error) {
	switch backend {
	case "local":
		return NewLocalStorage(path)
	case "s3":
		if s3Config == nil {
			return nil, ErrS3ConfigRequired
		}
		return NewS3Storage(*s3Config)
	default:
		return nil, ErrUnknownBackend
	}
}

type S3Config struct {
	Bucket    string
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

type StorageError struct {
	Op   string
	Path string
	Err  error
}

func (e *StorageError) Error() string {
	return e.Op + " " + e.Path + ": " + e.Err.Error()
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

var (
	ErrNotFound         = &StorageError{Op: "storage", Err: io.EOF}
	ErrS3ConfigRequired = &StorageError{Op: "storage", Err: io.EOF}
	ErrUnknownBackend   = &StorageError{Op: "storage", Err: io.EOF}
)

func init() {
	ErrNotFound = &StorageError{Op: "not found", Err: io.EOF}
	ErrS3ConfigRequired = &StorageError{Op: "s3 config required", Err: io.EOF}
	ErrUnknownBackend = &StorageError{Op: "unknown backend", Err: io.EOF}
}

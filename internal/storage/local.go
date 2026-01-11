package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		basePath: basePath,
	}, nil
}

func (l *LocalStorage) fullPath(path string) string {
	return filepath.Join(l.basePath, path)
}

func (l *LocalStorage) Write(ctx context.Context, path string, reader io.Reader) error {
	fullPath := l.fullPath(path)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &StorageError{Op: "write", Path: path, Err: err}
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return &StorageError{Op: "write", Path: path, Err: err}
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return &StorageError{Op: "write", Path: path, Err: err}
	}

	return nil
}

func (l *LocalStorage) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := l.fullPath(path)

	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, &StorageError{Op: "read", Path: path, Err: err}
	}

	return f, nil
}

func (l *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := l.fullPath(path)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return &StorageError{Op: "delete", Path: path, Err: err}
	}

	return nil
}

func (l *LocalStorage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	searchPath := l.fullPath(prefix)

	var files []FileInfo

	err := filepath.Walk(l.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(l.basePath, path)
		if err != nil {
			return err
		}

		if prefix != "" && !matchPrefix(relPath, prefix) {
			return nil
		}

		files = append(files, FileInfo{
			Path:         relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			IsDir:        info.IsDir(),
		})

		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, &StorageError{Op: "list", Path: searchPath, Err: err}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].LastModified.After(files[j].LastModified)
	})

	return files, nil
}

func (l *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := l.fullPath(path)

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, &StorageError{Op: "exists", Path: path, Err: err}
	}

	return true, nil
}

func (l *LocalStorage) Size(ctx context.Context, path string) (int64, error) {
	fullPath := l.fullPath(path)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrNotFound
		}
		return 0, &StorageError{Op: "size", Path: path, Err: err}
	}

	return info.Size(), nil
}

func matchPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

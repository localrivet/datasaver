package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Storage struct {
	client *minio.Client
	bucket string
}

func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	return &S3Storage{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (s *S3Storage) Write(ctx context.Context, path string, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return &StorageError{Op: "write", Path: path, Err: err}
	}

	_, err = s.client.PutObject(ctx, s.bucket, path, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if err != nil {
		return &StorageError{Op: "write", Path: path, Err: err}
	}

	return nil
}

func (s *S3Storage) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, &StorageError{Op: "read", Path: path, Err: err}
	}

	_, err = obj.Stat()
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, &StorageError{Op: "read", Path: path, Err: err}
	}

	return obj, nil
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	err := s.client.RemoveObject(ctx, s.bucket, path, minio.RemoveObjectOptions{})
	if err != nil {
		return &StorageError{Op: "delete", Path: path, Err: err}
	}

	return nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	var files []FileInfo

	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, &StorageError{Op: "list", Path: prefix, Err: object.Err}
		}

		files = append(files, FileInfo{
			Path:         object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
			IsDir:        false,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].LastModified.After(files[j].LastModified)
	})

	return files, nil
}

func (s *S3Storage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, &StorageError{Op: "exists", Path: path, Err: err}
	}

	return true, nil
}

func (s *S3Storage) Size(ctx context.Context, path string) (int64, error) {
	info, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return 0, ErrNotFound
		}
		return 0, &StorageError{Op: "size", Path: path, Err: err}
	}

	return info.Size, nil
}

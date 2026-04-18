package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOProvider — реализация Provider поверх MinIO/S3.
type MinIOProvider struct {
	client *minio.Client
	bucket string
}

// NewMinIO создаёт MinIO-клиент и убеждается, что бакет существует.
func NewMinIO(ctx context.Context, endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOProvider, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
	}

	return &MinIOProvider{client: client, bucket: bucket}, nil
}

func (m *MinIOProvider) Upload(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) (string, error) {
	_, err := m.client.PutObject(ctx, m.bucket, objectKey, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio put: %w", err)
	}
	return objectKey, nil
}

func (m *MinIOProvider) Download(ctx context.Context, storagePath string) (io.ReadCloser, int64, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, storagePath, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("minio get: %w", err)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, fmt.Errorf("minio stat: %w", err)
	}

	return obj, info.Size, nil
}

func (m *MinIOProvider) Delete(ctx context.Context, storagePath string) error {
	return m.client.RemoveObject(ctx, m.bucket, storagePath, minio.RemoveObjectOptions{})
}

package storage

import (
	"context"
	"io"
)

// Provider — абстракция над файловым хранилищем (MinIO/S3).
// Изолирует систему от vendor lock-in (архитектурный риск «Зависимость от MinIO»).
type Provider interface {
	// Upload сохраняет зашифрованный файл и возвращает путь в хранилище
	Upload(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) (storagePath string, err error)

	// Download возвращает reader для зашифрованного файла
	Download(ctx context.Context, storagePath string) (io.ReadCloser, int64, error)

	// Delete безвозвратно удаляет файл из хранилища (FR-16)
	// Операция идемпотентна: повторный вызов не возвращает ошибку, если файл уже удалён.
	Delete(ctx context.Context, storagePath string) error
}

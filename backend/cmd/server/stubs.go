// stubs.go — временные заглушки для Storage и Notifier.
// Storage заменяется MinIO-реализацией, Notifier — email/push провайдером.
package main

import (
	"context"
	"io"
	"strings"

	"github.com/borrowtime/server/internal/usecase"
)

// --- Stub: Storage (MinIO/S3) ---

type stubStorage struct{}

func (s *stubStorage) Upload(_ context.Context, key string, _ io.Reader, _ int64, _ string) (string, error) {
	return key, nil
}

func (s *stubStorage) Download(_ context.Context, _ string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader("")), 0, nil
}

func (s *stubStorage) Delete(_ context.Context, _ string) error { return nil }

// --- Stub: Notifier ---

type stubNotifier struct{}

func (s *stubNotifier) NotifyOwnerFileDeleted(_ context.Context, _, _, _ string) error { return nil }

// Интерфейсы соблюдены — убедиться при компиляции
var _ usecase.Notifier = (*stubNotifier)(nil)

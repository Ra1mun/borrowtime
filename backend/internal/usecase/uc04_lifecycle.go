// UC-04: Автоматическое управление жизненным циклом передач
// Приоритет: Высокий
// Актер: Система (фоновый процесс)
// FR: 13, 14, 16, 17
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
	"github.com/borrowtime/server/internal/storage"
)

// LifecycleUseCase — UC-04: планировщик, запускается каждые 5 минут
type LifecycleUseCase struct {
	transfers repository.TransferRepository
	audit     repository.AuditRepository
	store     storage.Provider
	notifier  Notifier // интерфейс уведомлений (email/push)
	logger    *slog.Logger
}

// Notifier — интерфейс отправки уведомлений отправителю (шаг 3.4)
type Notifier interface {
	NotifyOwnerFileDeleted(ctx context.Context, ownerID, transferID, fileName string) error
}

func NewLifecycle(
	transfers repository.TransferRepository,
	audit repository.AuditRepository,
	store storage.Provider,
	notifier Notifier,
	logger *slog.Logger,
) *LifecycleUseCase {
	return &LifecycleUseCase{
		transfers: transfers,
		audit:     audit,
		store:     store,
		notifier:  notifier,
		logger:    logger,
	}
}

// Run запускает цикл планировщика. Блокирует горутину до отмены ctx.
// Вызывать в отдельной горутине: go lifecycle.Run(ctx)
func (uc *LifecycleUseCase) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	uc.logger.Info("lifecycle scheduler started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			uc.logger.Info("lifecycle scheduler stopped")
			return
		case <-ticker.C:
			uc.runOnce(ctx)
		}
	}
}

// runOnce выполняет одну итерацию проверки (шаги 1-5 основного потока UC-04)
func (uc *LifecycleUseCase) runOnce(ctx context.Context) {
	// Шаг 2: запрашиваем ACTIVE-передачи, которые истекли или исчерпали лимит
	expired, err := uc.transfers.ListExpiredOrLimitReached(ctx)
	if err != nil {
		uc.logger.Error("lifecycle: query failed", "error", err)
		return
	}

	// 2а: нет передач для обработки
	if len(expired) == 0 {
		uc.logger.Debug("lifecycle: no active transfers to process")
		return
	}

	uc.logger.Info("lifecycle: processing transfers", "count", len(expired))

	for _, t := range expired {
		uc.process(ctx, t)
	}

	// Шаг 4: фиксируем выполнение задачи
	uc.logger.Info("lifecycle: run complete", "processed", len(expired))
}

// process обрабатывает одну истёкшую/исчерпанную передачу
func (uc *LifecycleUseCase) process(ctx context.Context, t *domain.Transfer) {
	// Шаг 3.1: определяем новый статус (FR-13, FR-14)
	newStatus := domain.StatusExpired
	if t.IsDownloadLimitReached() {
		newStatus = domain.StatusDownloaded
	}

	if err := uc.transfers.UpdateStatus(ctx, t.ID, newStatus); err != nil {
		uc.logger.Error("lifecycle: update status failed", "transferID", t.ID, "error", err)
		return
	}

	// Шаг 3.2: безвозвратное удаление файла (FR-16)
	// Идемпотентно: повторный вызов Delete не приведёт к ошибке (архитектурный риск «Идемпотентность»)
	if err := uc.deleteWithRetry(ctx, t); err != nil {
		// 3.2а: ошибка удаления — логируем, администратор уведомляется
		uc.logger.Error("lifecycle: delete failed after retries",
			"transferID", t.ID,
			"storagePath", t.StoragePath,
			"error", err,
		)
		_ = uc.audit.Append(ctx, &domain.AuditLog{
			ID:         uuid.NewString(),
			TransferID: t.ID,
			OwnerID:    t.OwnerID,
			EventType:  domain.EventAutoDeleted,
			ActorID:    "system",
			Success:    false,
			Details:    fmt.Sprintf("delete failed: %v", err),
			CreatedAt:  time.Now().UTC(),
		})
		return
	}

	// Шаг 3.3: событие AUTO_DELETED в аудит (FR-17)
	_ = uc.audit.Append(ctx, &domain.AuditLog{
		ID:         uuid.NewString(),
		TransferID: t.ID,
		OwnerID:    t.OwnerID,
		EventType:  domain.EventAutoDeleted,
		ActorID:    "system",
		Success:    true,
		Details:    string(newStatus),
		CreatedAt:  time.Now().UTC(),
	})

	// Шаг 3.4: уведомление отправителя (если включено)
	if uc.notifier != nil {
		if err := uc.notifier.NotifyOwnerFileDeleted(ctx, t.OwnerID, t.ID, t.FileName); err != nil {
			uc.logger.Warn("lifecycle: notification failed", "ownerID", t.OwnerID, "error", err)
		}
	}
}

// deleteWithRetry удаляет файл с экспоненциальной задержкой (3 попытки)
func (uc *LifecycleUseCase) deleteWithRetry(ctx context.Context, t *domain.Transfer) error {
	var err error
	backoff := 2 * time.Second

	for attempt := 1; attempt <= 3; attempt++ {
		err = uc.store.Delete(ctx, t.StoragePath)
		if err == nil {
			return nil
		}
		uc.logger.Warn("lifecycle: delete attempt failed",
			"attempt", attempt,
			"transferID", t.ID,
			"error", err,
		)
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}
	return err
}

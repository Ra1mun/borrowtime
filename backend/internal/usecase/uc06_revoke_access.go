// UC-06: Отзыв доступа к файлу
// Приоритет: Средний
// Актер: Отправитель
// FR: 15, 16, 17
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

// RevokeAccessInput — входные данные для UC-06
type RevokeAccessInput struct {
	TransferID  string
	RequesterID string // ID пользователя, инициирующего отзыв
	IPAddress   string
}

// RevokeAccessUseCase — UC-06
type RevokeAccessUseCase struct {
	transfers repository.TransferRepository
	audit     repository.AuditRepository
	store     storage.Provider
	logger    *slog.Logger
}

func NewRevokeAccess(
	transfers repository.TransferRepository,
	audit repository.AuditRepository,
	store storage.Provider,
	logger *slog.Logger,
) *RevokeAccessUseCase {
	return &RevokeAccessUseCase{
		transfers: transfers,
		audit:     audit,
		store:     store,
		logger:    logger,
	}
}

// Execute выполняет сценарий отзыва доступа (шаги 3-12)
func (uc *RevokeAccessUseCase) Execute(ctx context.Context, in RevokeAccessInput) error {
	// Шаг 3: получить передачу (FR-24)
	transfer, err := uc.transfers.GetByID(ctx, in.TransferID)
	if err != nil {
		return domain.ErrTransferNotFound
	}

	// Шаг 4: проверить владельца (FR-32 — ролевая модель)
	if transfer.OwnerID != in.RequesterID {
		return domain.ErrNotOwner
	}

	// Проверить, что передача активна (FR-15)
	if transfer.Status != domain.StatusActive {
		return domain.ErrTransferNotActive
	}

	// Шаг 8: изменить статус на REVOKED (FR-15)
	if err := uc.transfers.UpdateStatus(ctx, in.TransferID, domain.StatusRevoked); err != nil {
		return fmt.Errorf("update status to REVOKED: %w", err)
	}

	// Шаг 9: удаление файла из хранилища (FR-16)
	// При ошибке — статус уже REVOKED (доступ заблокирован), логируем для ручного разбора
	if err := uc.store.Delete(ctx, transfer.StoragePath); err != nil {
		// 9а: ошибка удаления (альтернативный поток)
		uc.logger.Error("revoke: file delete failed",
			"transferID", in.TransferID,
			"storagePath", transfer.StoragePath,
			"error", err,
		)
		_ = uc.audit.Append(ctx, &domain.AuditLog{
			ID:         uuid.NewString(),
			TransferID: in.TransferID,
			OwnerID:    in.RequesterID,
			EventType:  domain.EventRevoked,
			ActorID:    in.RequesterID,
			IPAddress:  in.IPAddress,
			Success:    false,
			Details:    fmt.Sprintf("revoked but delete failed: %v", err),
			CreatedAt:  time.Now().UTC(),
		})
		return nil // статус уже REVOKED — ответ клиенту успешный
	}

	// Шаг 10: запись MANUALLY_REVOKED в аудит (FR-17)
	_ = uc.audit.Append(ctx, &domain.AuditLog{
		ID:         uuid.NewString(),
		TransferID: in.TransferID,
		OwnerID:    transfer.OwnerID,
		EventType:  domain.EventRevoked,
		ActorID:    in.RequesterID,
		IPAddress:  in.IPAddress,
		Success:    true,
		CreatedAt:  time.Now().UTC(),
	})

	return nil
}

// GetTransferDetails возвращает детали передачи для отображения (шаг 4, FR-27)
func (uc *RevokeAccessUseCase) GetTransferDetails(ctx context.Context, transferID, requesterID string) (*domain.Transfer, error) {
	transfer, err := uc.transfers.GetByID(ctx, transferID)
	if err != nil {
		return nil, domain.ErrTransferNotFound
	}
	if transfer.OwnerID != requesterID {
		return nil, domain.ErrNotOwner
	}
	return transfer, nil
}

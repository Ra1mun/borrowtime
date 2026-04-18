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

// RevokeAccessInput — входные данные
type RevokeAccessInput struct {
	TransferID  string
	RequesterID string // ID пользователя, инициирующего отзыв
	IPAddress   string
}

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

// Execute выполняет сценарий отзыва доступа
func (uc *RevokeAccessUseCase) Execute(ctx context.Context, in RevokeAccessInput) error {
	transfer, err := uc.transfers.GetByID(ctx, in.TransferID)
	if err != nil {
		return domain.ErrTransferNotFound
	}

	if transfer.OwnerID != in.RequesterID {
		return domain.ErrNotOwner
	}

	if transfer.Status != domain.StatusActive {
		return domain.ErrTransferNotActive
	}

	if err := uc.transfers.UpdateStatus(ctx, in.TransferID, domain.StatusRevoked); err != nil {
		return fmt.Errorf("update status to REVOKED: %w", err)
	}

	if err := uc.store.Delete(ctx, transfer.StoragePath); err != nil {
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
		return nil
	}

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

// GetTransferDetails возвращает детали передачи для отображения
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

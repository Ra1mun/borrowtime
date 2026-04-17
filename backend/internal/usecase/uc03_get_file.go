// UC-03: Получение файла по ссылке
// Приоритет: Высокий
// Актер: Получатель (может быть гостем)
// FR: 11, 12, 13, 14, 17, 26, 30
package usecase

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
	"github.com/borrowtime/server/internal/storage"
)

// GetFileInput — входные данные для UC-03
type GetFileInput struct {
	Token          string
	RecipientID    string // пустой = гость
	RecipientEmail string
	IPAddress      string
	UserAgent      string
}

// GetFileOutput — результат UC-03
type GetFileOutput struct {
	FileName      string
	FileSizeBytes int64
	Encryption    domain.EncryptionMeta // метаданные для расшифровки на клиенте
	Content       io.ReadCloser         // поток зашифрованных байт
}

// GetFileUseCase — UC-03
type GetFileUseCase struct {
	transfers repository.TransferRepository
	audit     repository.AuditRepository
	store     storage.Provider
}

func NewGetFile(
	transfers repository.TransferRepository,
	audit repository.AuditRepository,
	store storage.Provider,
) *GetFileUseCase {
	return &GetFileUseCase{transfers: transfers, audit: audit, store: store}
}

// Execute выполняет сценарий получения файла по ссылке
func (uc *GetFileUseCase) Execute(ctx context.Context, in GetFileInput) (*GetFileOutput, error) {
	transfer, err := uc.transfers.GetByToken(ctx, in.Token)
	if err != nil {
		uc.logUnauthorized(ctx, "", in)
		return nil, domain.ErrTransferNotFound
	}

	if transfer.Status == domain.StatusRevoked {
		uc.logUnauthorized(ctx, transfer.ID, in)
		return nil, domain.ErrTransferRevoked
	}

	if transfer.IsExpired() {
		_ = uc.transfers.UpdateStatus(ctx, transfer.ID, domain.StatusExpired)
		uc.appendAudit(ctx, transfer, domain.EventExpired, "system", in.IPAddress, in.UserAgent, true)
		return nil, domain.ErrTransferExpired
	}

	if transfer.IsDownloadLimitReached() {
		return nil, domain.ErrDownloadLimitReached
	}

	if transfer.Policy.RequireAuth && in.RecipientID == "" {
		return nil, domain.ErrAuthRequired
	}

	uc.appendAudit(ctx, transfer, domain.EventViewed, actorID(in.RecipientID), in.IPAddress, in.UserAgent, true)

	content, _, err := uc.store.Download(ctx, transfer.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("download from storage: %w", err)
	}

	if err := uc.transfers.IncrementDownloads(ctx, transfer.ID); err != nil {
		_ = content.Close()
		return nil, fmt.Errorf("increment downloads: %w", err)
	}

	uc.appendAudit(ctx, transfer, domain.EventDownloaded, actorID(in.RecipientID), in.IPAddress, in.UserAgent, true)

	if transfer.Policy.MaxDownloads > 0 && transfer.DownloadCount+1 >= transfer.Policy.MaxDownloads {
		_ = uc.transfers.UpdateStatus(ctx, transfer.ID, domain.StatusDownloaded)
	}

	return &GetFileOutput{
		FileName:      transfer.FileName,
		FileSizeBytes: transfer.FileSizeBytes,
		Encryption:    transfer.Encryption,
		Content:       content,
	}, nil
}

func (uc *GetFileUseCase) logUnauthorized(ctx context.Context, transferID string, in GetFileInput) {
	_ = uc.audit.Append(ctx, &domain.AuditLog{
		ID:         uuid.NewString(),
		TransferID: transferID,
		EventType:  domain.EventUnauthorized,
		ActorID:    actorID(in.RecipientID),
		IPAddress:  in.IPAddress,
		UserAgent:  in.UserAgent,
		Success:    false,
		Details:    "invalid or revoked token",
		CreatedAt:  time.Now().UTC(),
	})
}

func (uc *GetFileUseCase) appendAudit(ctx context.Context, t *domain.Transfer, event domain.AuditEventType, actor, ip, ua string, ok bool) {
	_ = uc.audit.Append(ctx, &domain.AuditLog{
		ID:         uuid.NewString(),
		TransferID: t.ID,
		OwnerID:    t.OwnerID,
		EventType:  event,
		ActorID:    actor,
		IPAddress:  ip,
		UserAgent:  ua,
		Success:    ok,
		CreatedAt:  time.Now().UTC(),
	})
}

func isEmailAllowed(email string, allowed []string) bool {
	for _, e := range allowed {
		if e == email {
			return true
		}
	}
	return false
}

func actorID(recipientID string) string {
	if recipientID == "" {
		return "guest"
	}
	return recipientID
}

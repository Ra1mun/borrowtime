package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
	"github.com/borrowtime/server/internal/storage"
)

// CreateTransferInput — входные данные для UC-02
type CreateTransferInput struct {
	OwnerID       string
	FileName      string
	FileSizeBytes int64
	FileContent   io.Reader
	Encryption    domain.EncryptionMeta
	Policy        domain.AccessPolicy
}

// CreateTransferOutput — результат UC-02
type CreateTransferOutput struct {
	TransferID  string
	AccessToken string
	ShareURL    string
}

// CreateTransferUseCase — UC-02
type CreateTransferUseCase struct {
	transfers repository.TransferRepository
	audit     repository.AuditRepository
	store     storage.Provider
	settings  repository.SettingsRepository
	baseURL   string
}

func NewCreateTransfer(
	transfers repository.TransferRepository,
	audit repository.AuditRepository,
	store storage.Provider,
	settings repository.SettingsRepository,
	baseURL string,
) *CreateTransferUseCase {
	return &CreateTransferUseCase{
		transfers: transfers,
		audit:     audit,
		store:     store,
		settings:  settings,
		baseURL:   baseURL,
	}
}

// Execute выполняет сценарий создания безопасной передачи
func (uc *CreateTransferUseCase) Execute(ctx context.Context, in CreateTransferInput) (*CreateTransferOutput, error) {
	if err := uc.validate(ctx, in); err != nil {
		return nil, err
	}

	objectKey := fmt.Sprintf("transfers/%s/%s", uuid.NewString(), in.FileName)
	storagePath, err := uc.store.Upload(ctx, objectKey, in.FileContent, in.FileSizeBytes, "application/octet-stream")
	if err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	token, err := generateSecureToken(32) // 256 бит
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	now := time.Now().UTC()
	transfer := &domain.Transfer{
		ID:            uuid.NewString(),
		OwnerID:       in.OwnerID,
		FileName:      in.FileName,
		FileSizeBytes: in.FileSizeBytes,
		StoragePath:   storagePath,
		AccessToken:   token,
		Policy:        in.Policy,
		Encryption:    in.Encryption,
		Status:        domain.StatusActive,
		DownloadCount: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := uc.transfers.Create(ctx, transfer); err != nil {
		_ = uc.store.Delete(ctx, storagePath)
		return nil, fmt.Errorf("save transfer: %w", err)
	}

	uc.appendAudit(ctx, transfer.ID, in.OwnerID, domain.EventCreated, in.OwnerID, "", "")

	shareURL := fmt.Sprintf("%s/s/%s", uc.baseURL, token)

	return &CreateTransferOutput{
		TransferID:  transfer.ID,
		AccessToken: token,
		ShareURL:    shareURL,
	}, nil
}

// validate проверка входных данных
func (uc *CreateTransferUseCase) validate(ctx context.Context, in CreateTransferInput) error {
	if in.FileContent == nil {
		return domain.ErrNoFile
	}

	settings, err := uc.settings.Get(ctx)
	if err != nil {
		return fmt.Errorf("get settings: %w", err)
	}

	if in.FileSizeBytes > settings.MaxFileSizeBytes {
		return domain.ErrFileTooLarge
	}

	if !in.Policy.ExpiresAt.IsZero() && in.Policy.ExpiresAt.Before(time.Now()) {
		return domain.ErrInvalidPolicy
	}

	return nil
}

func (uc *CreateTransferUseCase) appendAudit(ctx context.Context, transferID, ownerID string, event domain.AuditEventType, actorID, ip, ua string) {
	_ = uc.audit.Append(ctx, &domain.AuditLog{
		ID:         uuid.NewString(),
		TransferID: transferID,
		OwnerID:    ownerID,
		EventType:  event,
		ActorID:    actorID,
		IPAddress:  ip,
		UserAgent:  ua,
		Success:    true,
		CreatedAt:  time.Now().UTC(),
	})
}

// generateSecureToken генерирует URL-safe токен с энтропией n байт
func generateSecureToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

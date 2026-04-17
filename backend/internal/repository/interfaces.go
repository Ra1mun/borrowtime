package repository

import (
	"context"
	"time"

	"github.com/borrowtime/server/internal/domain"
)

// UserRepository — интерфейс доступа к данным пользователей (UC-01)
type UserRepository interface {
	// Create создаёт нового пользователя
	Create(ctx context.Context, u *domain.User) error

	// FindByEmail ищет пользователя по email
	FindByEmail(ctx context.Context, email string) (*domain.User, error)

	// FindByID ищет пользователя по ID
	FindByID(ctx context.Context, id string) (*domain.User, error)

	// UpdateFailedAttempts обновляет счётчик неудачных попыток
	UpdateFailedAttempts(ctx context.Context, id string, count int) error

	// LockUntil устанавливает блокировку аккаунта до указанного времени
	LockUntil(ctx context.Context, id string, until time.Time) error

	// ResetLock сбрасывает блокировку и счётчик попыток
	ResetLock(ctx context.Context, id string) error

	// UpdateTOTPSecret сохраняет TOTP-секрет (временно, до подтверждения)
	UpdateTOTPSecret(ctx context.Context, id string, secret string) error

	// SetTOTPEnabled включает/выключает 2FA
	SetTOTPEnabled(ctx context.Context, id string, enabled bool) error

	// ListAll возвращает всех пользователей (для админ-панели)
	ListAll(ctx context.Context) ([]*domain.User, error)

	// Search ищет пользователей по email (prefix match)
	Search(ctx context.Context, query string, limit int) ([]*domain.User, error)

	// Delete удаляет пользователя
	Delete(ctx context.Context, id string) error

	// UpdateRole обновляет роль пользователя
	UpdateRole(ctx context.Context, id string, role domain.UserRole) error

	// SaveRefreshToken сохраняет хеш refresh-токена
	SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error

	// FindRefreshToken ищет refresh-токен по хешу
	FindRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)

	// DeleteRefreshToken удаляет refresh-токен (logout)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error

	// DeleteExpiredRefreshTokens удаляет просроченные токены пользователя
	DeleteExpiredRefreshTokens(ctx context.Context, userID string) error
}

// TransferRepository — интерфейс доступа к данным передач
type TransferRepository interface {
	// Create сохраняет новую передачу
	Create(ctx context.Context, t *domain.Transfer) error

	// GetByToken возвращает передачу по токену доступа
	GetByToken(ctx context.Context, token string) (*domain.Transfer, error)

	// GetByID возвращает передачу по ID
	GetByID(ctx context.Context, id string) (*domain.Transfer, error)

	// ListByOwner возвращает все передачи пользователя
	ListByOwner(ctx context.Context, ownerID string) ([]*domain.Transfer, error)

	// ListByRecipient возвращает передачи, где email получателя в policy_allowed_emails
	ListByRecipient(ctx context.Context, email string) ([]*domain.Transfer, error)

	// UpdateStatus меняет статус и время обновления
	UpdateStatus(ctx context.Context, id string, status domain.TransferStatus) error

	// IncrementDownloads атомарно увеличивает счётчик скачиваний
	IncrementDownloads(ctx context.Context, id string) error

	// ListExpiredOrLimitReached возвращает ACTIVE-передачи, у которых истёк срок или лимит (UC-04)
	ListExpiredOrLimitReached(ctx context.Context) ([]*domain.Transfer, error)
}

// AuditRepository — интерфейс доступа к журналу аудита
type AuditRepository interface {
	// Append добавляет запись в журнал
	Append(ctx context.Context, log *domain.AuditLog) error

	// List возвращает записи по фильтру
	List(ctx context.Context, filter domain.AuditFilter) ([]*domain.AuditLog, error)
}

// SettingsRepository — интерфейс хранения глобальных настроек (UC-08)
type SettingsRepository interface {
	// Get возвращает текущие настройки
	Get(ctx context.Context) (*domain.GlobalSettings, error)

	// Save сохраняет обновлённые настройки
	Save(ctx context.Context, s *domain.GlobalSettings) error
}

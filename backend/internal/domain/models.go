package domain

import (
	"time"
)

// UserRole — роль пользователя в системе (FR-32)
type UserRole string

const (
	RoleGuest UserRole = "guest"
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

// User — зарегистрированный пользователь системы (UC-01, FR-1)
type User struct {
	ID             string
	Email          string
	PasswordHash   string // Argon2id (FR-28)
	Role           UserRole
	TOTPSecret     string     // пусто пока 2FA не настроена
	TOTPEnabled    bool       // FR-2
	FailedAttempts int        // счётчик неудачных попыток (5а)
	LockedUntil    *time.Time // nil = не заблокирован
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsLocked возвращает true, если аккаунт заблокирован прямо сейчас
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && time.Now().Before(*u.LockedUntil)
}

// RefreshToken — хранимый refresh-токен (хеш SHA-256)
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// TransferStatus — статус жизненного цикла передачи (NFR-7)
type TransferStatus string

const (
	StatusActive     TransferStatus = "ACTIVE"
	StatusExpired    TransferStatus = "EXPIRED"
	StatusRevoked    TransferStatus = "REVOKED"
	StatusDownloaded TransferStatus = "DOWNLOADED"
)

// AuditEventType — тип события в журнале аудита (FR-17)
type AuditEventType string

const (
	EventCreated        AuditEventType = "CREATED"
	EventViewed         AuditEventType = "VIEWED"
	EventDownloaded     AuditEventType = "DOWNLOADED"
	EventExpired        AuditEventType = "EXPIRED"
	EventRevoked        AuditEventType = "MANUALLY_REVOKED"
	EventAutoDeleted    AuditEventType = "AUTO_DELETED"
	EventUnauthorized   AuditEventType = "UNAUTHORIZED_ACCESS"
	EventUserLogin      AuditEventType = "USER_LOGIN"
	EventUserRegistered AuditEventType = "USER_REGISTERED"
)

// AccessPolicy — политика доступа к передаче (FR-9)
type AccessPolicy struct {
	ExpiresAt     time.Time // срок действия ссылки
	MaxDownloads  int       // 0 = без ограничений
	RequireAuth   bool      // требует ли получатель аутентификации
	AllowedEmails []string  // список разрешённых email (опционально)
}

// EncryptionMeta — метаданные клиентского шифрования (FR-8)
type EncryptionMeta struct {
	Algorithm string // "AES-GCM"
	IV        string // base64
	Tag       string // base64
	KeyHint   string // публичная часть (не сам ключ — ключ не покидает браузер, NFR-5)
}

// Transfer — основная сущность «безопасная передача» (Секрет)
type Transfer struct {
	ID            string
	OwnerID       string // ID пользователя-отправителя
	FileName      string
	FileSizeBytes int64
	StoragePath   string // путь в хранилище (MinIO/S3)
	AccessToken   string // криптографически стойкий токен (NFR-6, ≥128 бит)
	Policy        AccessPolicy
	Encryption    EncryptionMeta
	Status        TransferStatus
	DownloadCount int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// IsExpired возвращает true, если срок действия истёк
func (t *Transfer) IsExpired() bool {
	return !t.Policy.ExpiresAt.IsZero() && time.Now().After(t.Policy.ExpiresAt)
}

// IsDownloadLimitReached возвращает true, если лимит скачиваний исчерпан
func (t *Transfer) IsDownloadLimitReached() bool {
	return t.Policy.MaxDownloads > 0 && t.DownloadCount >= t.Policy.MaxDownloads
}

// AuditLog — запись журнала аудита (FR-17)
type AuditLog struct {
	ID         string
	TransferID string
	OwnerID    string
	EventType  AuditEventType
	ActorID    string // пользователь, гость или "system"
	IPAddress  string
	UserAgent  string
	Success    bool
	Details    string
	CreatedAt  time.Time
}

// GlobalSettings — глобальные настройки системы (FR-22)
type GlobalSettings struct {
	MaxFileSizeBytes    int64         // максимальный размер файла
	MaxRetentionPeriod  time.Duration // максимальный срок хранения
	DefaultRetention    time.Duration // срок по умолчанию
	DefaultMaxDownloads int
	UpdatedAt           time.Time
	UpdatedBy           string
}

// AuditFilter — параметры фильтрации журнала аудита (UC-05)
type AuditFilter struct {
	TransferID string
	OwnerID    string         // пустой = все (только для админа)
	EventType  AuditEventType // пустой = все типы
	From       time.Time
	To         time.Time
	Limit      int
	Offset     int
}

// UC-01: Регистрация и вход с 2FA
// FR-1, FR-2, FR-5, FR-6, FR-17, FR-28, FR-34
package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
)

const (
	maxFailedAttempts = 5
	lockDuration      = 15 * time.Minute
	totpIssuer        = "BorrowTime"
)

// TokenPair — пара access + refresh токенов
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RegisterInput — входные данные для регистрации
type RegisterInput struct {
	Email    string
	Password string
}

// LoginInput — входные данные для входа
type LoginInput struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
}

// LoginResult — результат входа (могут потребоваться токены или 2FA)
type LoginResult struct {
	Tokens      *TokenPair // nil если требуется 2FA
	PartialJWT  string     // заполнен если требуется 2FA
	TwoFANeeded bool
}

// Verify2FAInput — входные данные для проверки 2FA-кода
type Verify2FAInput struct {
	PartialJWT string
	TOTPCode   string
	IPAddress  string
	UserAgent  string
}

// TOTPSetupResult — данные для настройки 2FA
type TOTPSetupResult struct {
	Secret       string // base32-секрет для ввода вручную
	ProvisionURL string // otpauth:// URL для QR-кода
}

// AuthClaims — поля JWT
type AuthClaims struct {
	UserID  string `json:"uid"`
	Role    string `json:"role"`
	Partial bool   `json:"partial,omitempty"` // true = partial_jwt для 2FA
	jwt.RegisteredClaims
}

// AuthUseCase — UC-01
type AuthUseCase struct {
	users      repository.UserRepository
	auditRepo  repository.AuditRepository
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	partialTTL time.Duration
	logger     *slog.Logger
}

func NewAuthUseCase(
	users repository.UserRepository,
	auditRepo repository.AuditRepository,
	jwtSecret string,
	accessTTL, refreshTTL, partialTTL time.Duration,
	logger *slog.Logger,
) *AuthUseCase {
	return &AuthUseCase{
		users:      users,
		auditRepo:  auditRepo,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		partialTTL: partialTTL,
		logger:     logger,
	}
}

// GetUser возвращает пользователя по ID (для /auth/me)
func (uc *AuthUseCase) GetUser(ctx context.Context, id string) (*domain.User, error) {
	return uc.users.FindByID(ctx, id)
}

// Register регистрация нового пользователя
func (uc *AuthUseCase) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	if err := validateEmail(in.Email); err != nil {
		return nil, err
	}
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := argon2id.CreateHash(in.Password, argon2id.DefaultParams)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	user := &domain.User{
		ID:           uuid.NewString(),
		Email:        in.Email,
		PasswordHash: hash,
		Role:         domain.RoleUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := uc.users.Create(ctx, user); err != nil {
		return nil, err
	}

	uc.logger.Info("user registered", "user_id", user.ID, "email", user.Email)
	return user, nil
}

// Login авторизация пользователя
func (uc *AuthUseCase) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	user, err := uc.users.FindByEmail(ctx, in.Email)
	if err != nil {
		// не раскрываем деталь (FR-6)
		return nil, domain.ErrInvalidCredentials
	}

	if user.IsLocked() {
		_ = uc.appendAudit(ctx, "", user.ID, domain.EventUnauthorized, in.IPAddress, in.UserAgent, false, "account locked")
		return nil, domain.ErrAccountLocked
	}

	match, err := argon2id.ComparePasswordAndHash(in.Password, user.PasswordHash)
	if err != nil || !match {
		return nil, uc.handleFailedLogin(ctx, user, in.IPAddress, in.UserAgent)
	}

	if user.FailedAttempts > 0 {
		_ = uc.users.ResetLock(ctx, user.ID)
	}

	if user.TOTPEnabled {
		partialJWT, err := uc.issuePartialJWT(user)
		if err != nil {
			return nil, fmt.Errorf("issue partial jwt: %w", err)
		}
		return &LoginResult{TwoFANeeded: true, PartialJWT: partialJWT}, nil
	}

	tokens, err := uc.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	_ = uc.appendAudit(ctx, "", user.ID, domain.EventUserLogin, in.IPAddress, in.UserAgent, true, "")
	return &LoginResult{Tokens: tokens}, nil
}

// Verify2FA подтверждение входа через 2FA авторизацию
func (uc *AuthUseCase) Verify2FA(ctx context.Context, in Verify2FAInput) (*TokenPair, error) {
	claims, err := uc.parsePartialJWT(in.PartialJWT)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	user, err := uc.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if !totp.Validate(in.TOTPCode, user.TOTPSecret) {
		_ = uc.appendAudit(ctx, "", user.ID, domain.EventUnauthorized, in.IPAddress, in.UserAgent, false, "invalid 2FA code")
		return nil, domain.ErrInvalidTOTP
	}

	tokens, err := uc.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	_ = uc.appendAudit(ctx, "", user.ID, domain.EventUserLogin, in.IPAddress, in.UserAgent, true, "2fa verified")
	return tokens, nil
}

// RefreshTokens — обновление пары токенов по refresh_jwt
func (uc *AuthUseCase) RefreshTokens(ctx context.Context, refreshToken string) (*TokenPair, error) {
	tokenHash := hashToken(refreshToken)

	rt, err := uc.users.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	if time.Now().After(rt.ExpiresAt) {
		_ = uc.users.DeleteRefreshToken(ctx, tokenHash)
		return nil, domain.ErrInvalidRefreshToken
	}

	user, err := uc.users.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, domain.ErrInvalidRefreshToken
	}

	// Ротация: удаляем старый, выдаём новый
	_ = uc.users.DeleteRefreshToken(ctx, tokenHash)

	tokens, err := uc.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

// Logout — удаление refresh-токена
func (uc *AuthUseCase) Logout(ctx context.Context, refreshToken string) error {
	return uc.users.DeleteRefreshToken(ctx, hashToken(refreshToken))
}

// SetupTOTP — генерирует новый TOTP-секрет, сохраняет его (не включает 2FA до подтверждения)
func (uc *AuthUseCase) SetupTOTP(ctx context.Context, userID string) (*TOTPSetupResult, error) {
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: user.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("generate totp key: %w", err)
	}

	if err := uc.users.UpdateTOTPSecret(ctx, userID, key.Secret()); err != nil {
		return nil, fmt.Errorf("save totp secret: %w", err)
	}

	return &TOTPSetupResult{
		Secret:       key.Secret(),
		ProvisionURL: key.URL(),
	}, nil
}

// ConfirmTOTP — подтверждает настройку 2FA кодом и включает её
func (uc *AuthUseCase) ConfirmTOTP(ctx context.Context, userID, code string) error {
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.TOTPEnabled {
		return domain.ErrTOTPAlreadyEnabled
	}
	if !totp.Validate(code, user.TOTPSecret) {
		return domain.ErrInvalidTOTP
	}
	return uc.users.SetTOTPEnabled(ctx, userID, true)
}

// DisableTOTP — отключает 2FA (требует валидного кода)
func (uc *AuthUseCase) DisableTOTP(ctx context.Context, userID, code string) error {
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.TOTPEnabled {
		return domain.ErrTOTPNotEnabled
	}
	if !totp.Validate(code, user.TOTPSecret) {
		return domain.ErrInvalidTOTP
	}
	if err := uc.users.SetTOTPEnabled(ctx, userID, false); err != nil {
		return err
	}
	return uc.users.UpdateTOTPSecret(ctx, userID, "")
}

// ValidateAccessJWT парсит и проверяет access_jwt; возвращает claims
func (uc *AuthUseCase) ValidateAccessJWT(tokenStr string) (*AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AuthClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return uc.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrInvalidCredentials
	}
	claims, ok := token.Claims.(*AuthClaims)
	if !ok || claims.Partial {
		return nil, domain.ErrInvalidCredentials
	}
	return claims, nil
}

func (uc *AuthUseCase) handleFailedLogin(ctx context.Context, user *domain.User, ip, ua string) error {
	newCount := user.FailedAttempts + 1
	_ = uc.users.UpdateFailedAttempts(ctx, user.ID, newCount)

	if newCount >= maxFailedAttempts {
		until := time.Now().Add(lockDuration)
		_ = uc.users.LockUntil(ctx, user.ID, until)
		_ = uc.appendAudit(ctx, "", user.ID, domain.EventUnauthorized, ip, ua, false, "account locked after failed attempts")
		return domain.ErrAccountLocked
	}

	_ = uc.appendAudit(ctx, "", user.ID, domain.EventUnauthorized, ip, ua, false, "invalid password")
	return domain.ErrInvalidCredentials
}

func (uc *AuthUseCase) issueTokenPair(ctx context.Context, user *domain.User) (*TokenPair, error) {
	accessToken, err := uc.signJWT(user.ID, string(user.Role), false, uc.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("sign access jwt: %w", err)
	}

	rawRefresh, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := time.Now().UTC()
	rt := &domain.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: hashToken(rawRefresh),
		ExpiresAt: now.Add(uc.refreshTTL),
		CreatedAt: now,
	}
	if err := uc.users.SaveRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	go func() {
		_ = uc.users.DeleteExpiredRefreshTokens(context.Background(), user.ID)
	}()

	return &TokenPair{AccessToken: accessToken, RefreshToken: rawRefresh}, nil
}

func (uc *AuthUseCase) issuePartialJWT(user *domain.User) (string, error) {
	return uc.signJWT(user.ID, string(user.Role), true, uc.partialTTL)
}

func (uc *AuthUseCase) signJWT(userID, role string, partial bool, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := &AuthClaims{
		UserID:  userID,
		Role:    role,
		Partial: partial,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(uc.jwtSecret)
}

func (uc *AuthUseCase) parsePartialJWT(tokenStr string) (*AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AuthClaims{}, func(t *jwt.Token) (any, error) {
		return uc.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid partial jwt")
	}
	claims, ok := token.Claims.(*AuthClaims)
	if !ok || !claims.Partial {
		return nil, errors.New("not a partial jwt")
	}
	return claims, nil
}

func (uc *AuthUseCase) appendAudit(ctx context.Context, transferID, ownerID string, event domain.AuditEventType, ip, ua string, success bool, details string) error {
	return uc.auditRepo.Append(ctx, &domain.AuditLog{
		ID:         uuid.NewString(),
		TransferID: transferID,
		OwnerID:    ownerID,
		EventType:  event,
		ActorID:    ownerID,
		IPAddress:  ip,
		UserAgent:  ua,
		Success:    success,
		Details:    details,
		CreatedAt:  time.Now().UTC(),
	})
}

func validateEmail(email string) error {
	if len(email) < 3 || len(email) > 254 {
		return fmt.Errorf("invalid email format")
	}
	for _, c := range email {
		if c == '@' {
			return nil
		}
	}
	return fmt.Errorf("invalid email format: missing @")
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

func generateToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken возвращает hex SHA-256 от токена (для хранения refresh-токенов)
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

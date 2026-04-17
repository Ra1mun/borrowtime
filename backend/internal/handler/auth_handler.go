// HTTP-хендлеры для UC-01 (регистрация, вход, 2FA, обновление токенов, выход)
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/usecase"
)

// AuthHandler — хендлер аутентификации и регистрации
type AuthHandler struct {
	authUC *usecase.AuthUseCase
}

func NewAuthHandler(authUC *usecase.AuthUseCase) *AuthHandler {
	return &AuthHandler{authUC: authUC}
}

// RegisterRoutes регистрирует публичные и защищённые маршруты авторизации
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	// Публичные (без токена)
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
	r.Post("/auth/2fa/verify", h.Verify2FA)
	r.Post("/auth/refresh", h.Refresh)
	r.Post("/auth/logout", h.Logout)

	// Защищённые (требуют access_jwt)
	r.Get("/auth/me", h.Me)
	r.Post("/auth/2fa/setup", h.Setup2FA)
	r.Post("/auth/2fa/confirm", h.Confirm2FA)
	r.Post("/auth/2fa/disable", h.Disable2FA)
}

// Me — GET /auth/me
//
// @Summary      Информация о текущем пользователе
// @Description  Возвращает данные авторизованного пользователя по access_token.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]string  "id, email, role"
// @Failure      401  {object}  map[string]string
// @Security     BearerAuth
// @Router       /auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.authUC.GetUser(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"role":         string(user.Role),
		"totp_enabled": user.TOTPEnabled,
	})
}

// Register — POST /auth/register
//
// @Summary      Регистрация нового пользователя
// @Description  Создаёт аккаунт с email и паролем. Пароль хешируется Argon2id (FR-28). Минимальная длина пароля — 8 символов.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "email, password"
// @Success      201  {object}  map[string]string  "id, email"
// @Failure      400  {object}  map[string]string
// @Failure      409  {object}  map[string]string  "email already taken"
// @Failure      500  {object}  map[string]string
// @Router       /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	user, err := h.authUC.Register(r.Context(), usecase.RegisterInput{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		if errors.Is(err, domain.ErrEmailTaken) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"id":    user.ID,
		"email": user.Email,
	})
}

// Login — POST /auth/login
//
// @Summary      Вход в систему
// @Description  Аутентифицирует пользователя по email и паролю. Если 2FA включена — возвращает partial_jwt и статус 2fa_required (FR-2). При ≥5 неверных попытках аккаунт блокируется на 15 минут.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "email, password"
// @Success      200  {object}  map[string]any  "access_token + refresh_token ИЛИ partial_jwt + status=2fa_required"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      423  {object}  map[string]string  "account locked"
// @Failure      500  {object}  map[string]string
// @Router       /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	result, err := h.authUC.Login(r.Context(), usecase.LoginInput{
		Email:     body.Email,
		Password:  body.Password,
		IPAddress: realIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrAccountLocked):
			respondError(w, http.StatusLocked, err.Error())
		default:
			respondError(w, http.StatusUnauthorized, "invalid email or password")
		}
		return
	}

	if result.TwoFANeeded {
		respondJSON(w, http.StatusOK, map[string]any{
			"status":      "2fa_required",
			"partial_jwt": result.PartialJWT,
		})
		return
	}

	respondJSON(w, http.StatusOK, result.Tokens)
}

// Verify2FA — POST /auth/2fa/verify
//
// @Summary      Верификация кода 2FA
// @Description  Принимает partial_jwt и TOTP-код, возвращает финальную пару токенов (FR-2, шаги 6.3–6.5 UC-01). Допускается 3 попытки на один partial_jwt.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "partial_jwt, code"
// @Success      200  {object}  usecase.TokenPair
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string  "invalid code or expired partial_jwt"
// @Failure      500  {object}  map[string]string
// @Router       /auth/2fa/verify [post]
func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PartialJWT string `json:"partial_jwt"`
		Code       string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	tokens, err := h.authUC.Verify2FA(r.Context(), usecase.Verify2FAInput{
		PartialJWT: body.PartialJWT,
		TOTPCode:   body.Code,
		IPAddress:  realIP(r),
		UserAgent:  r.UserAgent(),
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidTOTP) {
			respondError(w, http.StatusUnauthorized, err.Error())
			return
		}
		respondError(w, http.StatusUnauthorized, "authentication failed")
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

// Refresh — POST /auth/refresh
//
// @Summary      Обновление токенов
// @Description  Принимает refresh_token, возвращает новую пару access + refresh. Ротация: старый токен удаляется.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "refresh_token"
// @Success      200  {object}  usecase.TokenPair
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	tokens, err := h.authUC.RefreshTokens(r.Context(), body.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

// Logout — POST /auth/logout
//
// @Summary      Выход из системы
// @Description  Инвалидирует refresh_token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "refresh_token"
// @Success      204  "no content"
// @Failure      400  {object}  map[string]string
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	_ = h.authUC.Logout(r.Context(), body.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

// Setup2FA — POST /auth/2fa/setup
//
// @Summary      Начало настройки 2FA
// @Description  Генерирует TOTP-секрет и возвращает URI для QR-кода. 2FA включается только после подтверждения кодом (POST /auth/2fa/confirm). Требует аутентификации.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]string  "secret, provision_url"
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /auth/2fa/setup [post]
func (h *AuthHandler) Setup2FA(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	result, err := h.authUC.SetupTOTP(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"secret":       result.Secret,
		"provision_url": result.ProvisionURL,
	})
}

// Confirm2FA — POST /auth/2fa/confirm
//
// @Summary      Подтверждение и включение 2FA
// @Description  Принимает TOTP-код, проверяет его и активирует двухфакторную аутентификацию для аккаунта. Требует аутентификации.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "code"
// @Success      200  {object}  map[string]string  "status: enabled"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      409  {object}  map[string]string  "2fa already enabled"
// @Security     BearerAuth
// @Router       /auth/2fa/confirm [post]
func (h *AuthHandler) Confirm2FA(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := h.authUC.ConfirmTOTP(r.Context(), userID, body.Code); err != nil {
		switch {
		case errors.Is(err, domain.ErrTOTPAlreadyEnabled):
			respondError(w, http.StatusConflict, err.Error())
		case errors.Is(err, domain.ErrInvalidTOTP):
			respondError(w, http.StatusBadRequest, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

// Disable2FA — POST /auth/2fa/disable
//
// @Summary      Отключение 2FA
// @Description  Принимает TOTP-код и отключает двухфакторную аутентификацию. Требует аутентификации.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "code"
// @Success      200  {object}  map[string]string  "status: disabled"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      409  {object}  map[string]string  "2fa not enabled"
// @Security     BearerAuth
// @Router       /auth/2fa/disable [post]
func (h *AuthHandler) Disable2FA(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := h.authUC.DisableTOTP(r.Context(), userID, body.Code); err != nil {
		switch {
		case errors.Is(err, domain.ErrTOTPNotEnabled):
			respondError(w, http.StatusConflict, err.Error())
		case errors.Is(err, domain.ErrInvalidTOTP):
			respondError(w, http.StatusBadRequest, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

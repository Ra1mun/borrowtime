// HTTP-хендлеры для UC-02, UC-03, UC-06
package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
	"github.com/borrowtime/server/internal/usecase"
)

// TransferHandler — хендлер передач файлов
type TransferHandler struct {
	createUC *usecase.CreateTransferUseCase
	getUC    *usecase.GetFileUseCase
	revokeUC *usecase.RevokeAccessUseCase
	userRepo repository.UserRepository
}

func NewTransferHandler(
	create *usecase.CreateTransferUseCase,
	get *usecase.GetFileUseCase,
	revoke *usecase.RevokeAccessUseCase,
	userRepo repository.UserRepository,
) *TransferHandler {
	return &TransferHandler{createUC: create, getUC: get, revokeUC: revoke, userRepo: userRepo}
}

// RegisterRoutes регистрирует маршруты в chi-роутере
func (h *TransferHandler) RegisterRoutes(r chi.Router) {
	// UC-02: создание передачи (требует аутентификации)
	r.Post("/transfers", h.Create)

	// UC-03: получение файла по токену (публичный endpoint)
	r.Get("/s/{token}", h.Download)

	// UC-06: отзыв доступа (требует аутентификации)
	r.Delete("/transfers/{id}", h.Revoke)
	r.Get("/transfers/{id}", h.GetDetails)
}

// Create — UC-02: POST /transfers
//
// @Summary      Создать передачу файла
// @Description  Загружает зашифрованный файл и создаёт ссылку для получателя. Требует аутентификации.
// @Tags         transfers
// @Accept       multipart/form-data
// @Produce      json
// @Param        file                    formData  file    true   "Зашифрованный файл"
// @Param        policy_expires_at       formData  string  false  "Срок действия ссылки (RFC3339)"
// @Param        policy_max_downloads    formData  int     false  "Максимальное количество скачиваний (0 — без ограничений)"
// @Param        policy_require_auth     formData  bool    false  "Требовать аутентификацию получателя"
// @Param        policy_allowed_emails   formData  string  false  "Разрешённые email-адреса (можно передать несколько раз)"
// @Param        encryption_alg         formData  string  false  "Алгоритм шифрования (AES-256-GCM)"
// @Param        encryption_iv          formData  string  false  "Initialization Vector (base64)"
// @Param        encryption_tag         formData  string  false  "Auth tag (base64)"
// @Success      201  {object}  map[string]string  "transfer_id, share_url, access_token"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      413  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /transfers [post]
func (h *TransferHandler) Create(w http.ResponseWriter, r *http.Request) {
	ownerID := userIDFromCtx(r.Context())
	if ownerID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Ограничение размера: 200 МБ для multipart-памяти, остальное — на диск
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Разбор политики доступа (FR-9)
	policy, err := parsePolicyFromForm(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Метаданные шифрования (FR-8) — заполнены клиентом
	encryption := domain.EncryptionMeta{
		Algorithm: r.FormValue("encryption_alg"),
		IV:        r.FormValue("encryption_iv"),
		Tag:       r.FormValue("encryption_tag"),
	}

	out, err := h.createUC.Execute(r.Context(), usecase.CreateTransferInput{
		OwnerID:       ownerID,
		FileName:      header.Filename,
		FileSizeBytes: header.Size,
		FileContent:   file,
		Encryption:    encryption,
		Policy:        policy,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrFileTooLarge):
			respondError(w, http.StatusRequestEntityTooLarge, err.Error())
		case errors.Is(err, domain.ErrInvalidPolicy), errors.Is(err, domain.ErrNoFile):
			respondError(w, http.StatusBadRequest, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"transfer_id":  out.TransferID,
		"share_url":    out.ShareURL,
		"access_token": out.AccessToken,
	})
}

// Download — UC-03: GET /s/{token}
//
// @Summary      Скачать файл по токену
// @Description  Публичный endpoint для получения зашифрованного файла по одноразовому токену.
// @Tags         transfers
// @Produce      application/octet-stream
// @Param        token  path    string  true  "Одноразовый токен доступа"
// @Param        email  query   string  false "Email получателя (если требуется политикой)"
// @Success      200  {file}  binary  "Зашифрованный файл"
// @Header       200  {string}  X-Encryption-Alg  "Алгоритм шифрования"
// @Header       200  {string}  X-Encryption-IV   "Initialization Vector (base64)"
// @Header       200  {string}  X-Encryption-Tag  "Auth tag (base64)"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      410  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /s/{token} [get]
func (h *TransferHandler) Download(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	recipientID := userIDFromCtx(r.Context()) // может быть пустым для гостей

	// Определяем email получателя: из query или из профиля аутентифицированного пользователя
	recipientEmail := r.URL.Query().Get("email")
	if recipientEmail == "" && recipientID != "" {
		if u, err := h.userRepo.FindByID(r.Context(), recipientID); err == nil {
			recipientEmail = u.Email
		}
	}

	out, err := h.getUC.Execute(r.Context(), usecase.GetFileInput{
		Token:          token,
		RecipientID:    recipientID,
		RecipientEmail: recipientEmail,
		IPAddress:      realIP(r),
		UserAgent:      r.UserAgent(),
	})

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTransferNotFound):
			respondError(w, http.StatusNotFound, "transfer not found or access denied")
		case errors.Is(err, domain.ErrTransferExpired):
			respondError(w, http.StatusGone, "transfer has expired")
		case errors.Is(err, domain.ErrTransferRevoked):
			respondError(w, http.StatusForbidden, "access has been revoked")
		case errors.Is(err, domain.ErrDownloadLimitReached):
			respondError(w, http.StatusForbidden, "download limit reached")
		case errors.Is(err, domain.ErrAuthRequired):
			respondError(w, http.StatusUnauthorized, "authentication required")
		case errors.Is(err, domain.ErrEmailNotAllowed):
			respondError(w, http.StatusForbidden, "your email is not allowed")
		default:
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	defer out.Content.Close()

	// Передаём метаданные шифрования в заголовках (клиент использует для расшифровки)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, out.FileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(out.FileSizeBytes, 10))
	w.Header().Set("X-Encryption-Alg", out.Encryption.Algorithm)
	w.Header().Set("X-Encryption-IV", out.Encryption.IV)
	w.Header().Set("X-Encryption-Tag", out.Encryption.Tag)
	w.WriteHeader(http.StatusOK)

	_, _ = io.Copy(w, out.Content)
}

// Revoke — UC-06: DELETE /transfers/{id}
//
// @Summary      Отозвать доступ к передаче
// @Description  Досрочно блокирует ссылку. Доступно только владельцу передачи.
// @Tags         transfers
// @Produce      json
// @Param        id  path  string  true  "UUID передачи"
// @Success      200  {object}  map[string]string  "status: revoked"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /transfers/{id} [delete]
func (h *TransferHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	requesterID := userIDFromCtx(r.Context())
	if requesterID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	transferID := chi.URLParam(r, "id")

	err := h.revokeUC.Execute(r.Context(), usecase.RevokeAccessInput{
		TransferID:  transferID,
		RequesterID: requesterID,
		IPAddress:   realIP(r),
	})

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTransferNotFound):
			respondError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrNotOwner):
			respondError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, domain.ErrTransferNotActive):
			respondError(w, http.StatusConflict, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// GetDetails — UC-06 шаг 4: GET /transfers/{id} — детали перед отзывом (FR-27)
//
// @Summary      Получить детали передачи
// @Description  Возвращает метаданные передачи. Доступно только владельцу.
// @Tags         transfers
// @Produce      json
// @Param        id  path  string  true  "UUID передачи"
// @Success      200  {object}  map[string]any
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /transfers/{id} [get]
func (h *TransferHandler) GetDetails(w http.ResponseWriter, r *http.Request) {
	requesterID := userIDFromCtx(r.Context())
	if requesterID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	transferID := chi.URLParam(r, "id")
	transfer, err := h.revokeUC.GetTransferDetails(r.Context(), transferID, requesterID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTransferNotFound):
			respondError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrNotOwner):
			respondError(w, http.StatusForbidden, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"id":             transfer.ID,
		"file_name":      transfer.FileName,
		"file_size":      transfer.FileSizeBytes,
		"status":         transfer.Status,
		"download_count": transfer.DownloadCount,
		"policy": map[string]any{
			"expires_at":     transfer.Policy.ExpiresAt,
			"max_downloads":  transfer.Policy.MaxDownloads,
			"require_auth":   transfer.Policy.RequireAuth,
			"allowed_emails": transfer.Policy.AllowedEmails,
		},
		"created_at": transfer.CreatedAt,
	})
}

// parsePolicyFromForm разбирает политику доступа из form-данных (FR-9)
func parsePolicyFromForm(r *http.Request) (domain.AccessPolicy, error) {
	policy := domain.AccessPolicy{}

	if v := r.FormValue("policy_expires_at"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return policy, fmt.Errorf("invalid policy_expires_at: use RFC3339 format")
		}
		policy.ExpiresAt = t
	}

	if v := r.FormValue("policy_max_downloads"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return policy, fmt.Errorf("invalid policy_max_downloads: must be non-negative integer")
		}
		policy.MaxDownloads = n
	}

	policy.RequireAuth = r.FormValue("policy_require_auth") == "true"

	if emails := r.Form["policy_allowed_emails"]; len(emails) > 0 {
		policy.AllowedEmails = emails
	}

	return policy, nil
}

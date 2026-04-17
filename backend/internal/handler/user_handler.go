// HTTP-хендлеры для управления пользователями (админ-панель)
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
	"github.com/borrowtime/server/internal/usecase"
)

// UserHandler — хендлер управления пользователями
type UserHandler struct {
	userRepo     repository.UserRepository
	transferRepo repository.TransferRepository
}

func NewUserHandler(userRepo repository.UserRepository, transferRepo repository.TransferRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo, transferRepo: transferRepo}
}

// RegisterRoutes регистрирует маршруты управления пользователями
func (h *UserHandler) RegisterRoutes(r chi.Router) {
	r.Get("/users", h.List)
	r.Get("/users/search", h.Search)
	r.Delete("/users/{id}", h.Delete)
	r.Put("/users/{id}/role", h.UpdateRole)
	r.Get("/transfers", h.ListTransfers)
	r.Get("/transfers/incoming", h.ListIncomingTransfers)
}

// List — GET /users (admin only)
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	users, err := h.userRepo.ListAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	result := make([]map[string]any, 0, len(users))
	for _, u := range users {
		result = append(result, map[string]any{
			"id":         u.ID,
			"email":      u.Email,
			"role":       string(u.Role),
			"created_at": u.CreatedAt,
		})
	}

	respondJSON(w, http.StatusOK, result)
}

// Search — GET /users/search?q=... (authenticated)
func (h *UserHandler) Search(w http.ResponseWriter, r *http.Request) {
	requesterID := userIDFromCtx(r.Context())
	if requesterID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	q := r.URL.Query().Get("q")
	users, err := h.userRepo.Search(r.Context(), q, 10)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	result := make([]map[string]any, 0, len(users))
	for _, u := range users {
		if u.ID == requesterID {
			continue
		}
		result = append(result, map[string]any{
			"id":    u.ID,
			"email": u.Email,
			"role":  string(u.Role),
		})
	}

	respondJSON(w, http.StatusOK, result)
}

// Delete — DELETE /users/{id} (admin only)
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.userRepo.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateRole — PUT /users/{id}/role (admin only)
func (h *UserHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id := chi.URLParam(r, "id")
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	role := domain.UserRole(body.Role)
	if role != domain.RoleUser && role != domain.RoleAdmin {
		respondError(w, http.StatusBadRequest, "invalid role: must be 'user' or 'admin'")
		return
	}

	if err := h.userRepo.UpdateRole(r.Context(), id, role); err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ListTransfers — GET /transfers (authenticated, lists own transfers)
func (h *UserHandler) ListTransfers(w http.ResponseWriter, r *http.Request) {
	ownerID := userIDFromCtx(r.Context())
	if ownerID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	role := usecase.UserRole(userRoleFromCtx(r.Context()))

	var transfers []*domain.Transfer
	var err error

	// Админ видит все передачи, пользователь — только свои
	if role == usecase.RoleAdmin {
		// For admin: still show own transfers (no ListAll for transfers)
		transfers, err = h.transferRepo.ListByOwner(r.Context(), ownerID)
	} else {
		transfers, err = h.transferRepo.ListByOwner(r.Context(), ownerID)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	result := make([]map[string]any, 0, len(transfers))
	for _, t := range transfers {
		result = append(result, map[string]any{
			"id":             t.ID,
			"file_name":      t.FileName,
			"file_size":      t.FileSizeBytes,
			"status":         string(t.Status),
			"access_token":   t.AccessToken,
			"download_count": t.DownloadCount,
			"expires_at":     t.Policy.ExpiresAt,
			"created_at":     t.CreatedAt,
		})
	}

	respondJSON(w, http.StatusOK, result)
}

// ListIncomingTransfers — GET /transfers/incoming (файлы, где пользователь — получатель)
func (h *UserHandler) ListIncomingTransfers(w http.ResponseWriter, r *http.Request) {
	ownerID := userIDFromCtx(r.Context())
	if ownerID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.userRepo.FindByID(r.Context(), ownerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	transfers, err := h.transferRepo.ListByRecipient(r.Context(), user.Email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	incoming := make([]map[string]any, 0, len(transfers))
	for _, t := range transfers {
		incoming = append(incoming, map[string]any{
			"id":             t.ID,
			"file_name":      t.FileName,
			"file_size":      t.FileSizeBytes,
			"status":         string(t.Status),
			"access_token":   t.AccessToken,
			"download_count": t.DownloadCount,
			"expires_at":     t.Policy.ExpiresAt,
			"created_at":     t.CreatedAt,
		})
	}

	respondJSON(w, http.StatusOK, incoming)
}
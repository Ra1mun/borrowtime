// HTTP-хендлеры для UC-08 (управление настройками и статистика)
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/usecase"
)

// AdminHandler — хендлер администрирования
type AdminHandler struct {
	settingsUC *usecase.GlobalSettingsUseCase
}

func NewAdminHandler(settingsUC *usecase.GlobalSettingsUseCase) *AdminHandler {
	return &AdminHandler{settingsUC: settingsUC}
}

// RegisterRoutes регистрирует маршруты администратора
func (h *AdminHandler) RegisterRoutes(r chi.Router) {
	r.Get("/admin/settings", h.GetSettings)
	r.Put("/admin/settings", h.UpdateSettings)
	r.Get("/admin/stats", h.GetStats)
}

// GetSettings — GET /admin/settings
//
// @Summary      Получить глобальные настройки
// @Description  Возвращает текущие системные настройки. Только для администраторов.
// @Tags         admin
// @Produce      json
// @Success      200  {object}  map[string]any
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/settings [get]
func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	s, err := h.settingsUC.GetSettings(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, settingsToJSON(s))
}

// UpdateSettings — PUT /admin/settings
//
// @Summary      Обновить глобальные настройки
// @Description  Изменяет системные ограничения. Только для администраторов.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "Настройки: max_file_size_mb, max_retention_days, default_retention_hours, default_max_downloads"
// @Success      200  {object}  map[string]any
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/settings [put]
func (h *AdminHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var body struct {
		MaxFileSizeMB       int64 `json:"max_file_size_mb"`
		MaxRetentionDays    int   `json:"max_retention_days"`
		DefaultRetentionH   int   `json:"default_retention_hours"`
		DefaultMaxDownloads int   `json:"default_max_downloads"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	updated, err := h.settingsUC.UpdateSettings(r.Context(), usecase.UpdateSettingsInput{
		AdminID:             userIDFromCtx(r.Context()),
		MaxFileSizeBytes:    body.MaxFileSizeMB * 1024 * 1024,
		MaxRetentionPeriod:  time.Duration(body.MaxRetentionDays) * 24 * time.Hour,
		DefaultRetention:    time.Duration(body.DefaultRetentionH) * time.Hour,
		DefaultMaxDownloads: body.DefaultMaxDownloads,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidSettings) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, settingsToJSON(updated))
}

// GetStats — GET /admin/stats (FR-23)
//
// @Summary      Статистика системы
// @Description  Возвращает агрегированные показатели: активные передачи, объём хранилища, инциденты безопасности. Только для администраторов.
// @Tags         admin
// @Produce      json
// @Success      200  {object}  map[string]any  "active_transfers, total_storage_bytes, security_incidents_today"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/stats [get]
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	stats, err := h.settingsUC.GetStats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"active_transfers":         stats.ActiveTransfers,
		"total_storage_bytes":      stats.TotalStorageBytes,
		"security_incidents_today": stats.SecurityIncidentsToday,
	})
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if userIDFromCtx(r.Context()) == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	if userRoleFromCtx(r.Context()) != string(usecase.RoleAdmin) {
		respondError(w, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

func settingsToJSON(s *domain.GlobalSettings) map[string]any {
	return map[string]any{
		"max_file_size_mb":      s.MaxFileSizeBytes / (1024 * 1024),
		"max_retention_days":    int(s.MaxRetentionPeriod.Hours() / 24),
		"default_retention_h":   int(s.DefaultRetention.Hours()),
		"default_max_downloads": s.DefaultMaxDownloads,
		"updated_at":            s.UpdatedAt,
		"updated_by":            s.UpdatedBy,
	}
}

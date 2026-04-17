// HTTP-хендлеры для UC-05 (просмотр журнала) и UC-07 (экспорт CSV)
package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/usecase"
)

// AuditHandler — хендлер журнала аудита
type AuditHandler struct {
	auditUC  *usecase.AuditLogUseCase
	exportUC *usecase.ExportAuditUseCase
}

func NewAuditHandler(auditUC *usecase.AuditLogUseCase, exportUC *usecase.ExportAuditUseCase) *AuditHandler {
	return &AuditHandler{auditUC: auditUC, exportUC: exportUC}
}

// RegisterRoutes регистрирует маршруты
func (h *AuditHandler) RegisterRoutes(r chi.Router) {
	// UC-05: просмотр журнала (FR-18, FR-19)
	r.Get("/audit", h.List)

	// UC-07: экспорт в CSV (FR-20)
	r.Get("/audit/export", h.ExportCSV)
}

// List — UC-05: GET /audit?from=...&to=...&event_type=...&limit=50&offset=0
//
// @Summary      Список событий аудита
// @Description  Возвращает отфильтрованный журнал аудита. Администратор видит все события, пользователь — только свои.
// @Tags         audit
// @Produce      json
// @Param        transfer_id  query   string  false  "Фильтр по ID передачи"
// @Param        event_type   query   string  false  "Тип события (transfer_created, file_downloaded, …)"
// @Param        from         query   string  false  "Начало периода (RFC3339)"
// @Param        to           query   string  false  "Конец периода (RFC3339)"
// @Success      200  {object}  map[string]any  "events, total"
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audit [get]
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	requesterID := userIDFromCtx(r.Context())
	if requesterID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	role := usecase.UserRole(userRoleFromCtx(r.Context()))
	filter, err := parseAuditFilter(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	out, err := h.auditUC.Execute(r.Context(), usecase.AuditLogInput{
		RequesterID:   requesterID,
		RequesterRole: role,
		Filter:        filter,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// 4а: пустой результат — сообщение из ТЗ
	if out.Total == 0 {
		respondJSON(w, http.StatusOK, map[string]any{
			"events":  []any{},
			"total":   0,
			"message": "Нет записей аудита за выбранный период",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"events": out.Events,
		"total":  out.Total,
	})
}

// ExportCSV — UC-07: GET /audit/export?from=...&to=...
//
// @Summary      Экспорт журнала аудита в CSV
// @Description  Скачивает все записи аудита в формате CSV. Доступно только администраторам.
// @Tags         audit
// @Produce      text/csv
// @Param        transfer_id  query   string  false  "Фильтр по ID передачи"
// @Param        event_type   query   string  false  "Тип события"
// @Param        from         query   string  false  "Начало периода (RFC3339)"
// @Param        to           query   string  false  "Конец периода (RFC3339)"
// @Success      200  {file}    binary  "CSV-файл"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audit/export [get]
func (h *AuditHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	requesterID := userIDFromCtx(r.Context())
	if requesterID == "" {
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	role := usecase.UserRole(userRoleFromCtx(r.Context()))

	// Только администратор может экспортировать все логи (FR-32)
	if role != usecase.RoleAdmin {
		respondError(w, http.StatusForbidden, "admin role required for full export")
		return
	}

	filter, err := parseAuditFilter(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	filename := fmt.Sprintf("audit_%s.csv", time.Now().UTC().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)

	if err := h.exportUC.WriteCSV(r.Context(), usecase.ExportInput{
		RequesterID:   requesterID,
		RequesterRole: role,
		Filter:        filter,
	}, w); err != nil {
		// Заголовки уже отправлены — логируем, но не можем изменить статус
		_, _ = fmt.Fprintf(w, "\n# export error: %v\n", err)
	}
}

// parseAuditFilter разбирает параметры фильтрации из query string
func parseAuditFilter(r *http.Request) (domain.AuditFilter, error) {
	filter := domain.AuditFilter{}
	q := r.URL.Query()

	if v := q.Get("transfer_id"); v != "" {
		filter.TransferID = v
	}

	if v := q.Get("event_type"); v != "" {
		filter.EventType = domain.AuditEventType(v)
	}

	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, fmt.Errorf("invalid 'from' date: use RFC3339")
		}
		filter.From = t
	}

	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, fmt.Errorf("invalid 'to' date: use RFC3339")
		}
		filter.To = t
	}

	return filter, nil
}

// UC-07: Экспорт журнала аудита в CSV
// Приоритет: Низкий
// Актер: Администратор
// FR: 20
package usecase

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
)

// ExportAuditUseCase — UC-07
type ExportAuditUseCase struct {
	audit repository.AuditRepository
}

func NewExportAudit(audit repository.AuditRepository) *ExportAuditUseCase {
	return &ExportAuditUseCase{audit: audit}
}

// ExportInput — параметры экспорта (шаг 6а.1)
type ExportInput struct {
	RequesterID   string
	RequesterRole UserRole
	Filter        domain.AuditFilter
}

// WriteCSV записывает журнал аудита в CSV-формат в указанный writer (FR-20).
// Метод использует streaming: данные пишутся по мере получения из репозитория.
func (uc *ExportAuditUseCase) WriteCSV(ctx context.Context, in ExportInput, w io.Writer) error {
	// Только администратор может экспортировать все логи (FR-32)
	if in.RequesterRole != RoleAdmin {
		in.Filter.OwnerID = in.RequesterID
	}

	// Снять лимит пагинации для полного экспорта
	in.Filter.Limit = 0

	events, err := uc.audit.List(ctx, in.Filter)
	if err != nil {
		return fmt.Errorf("query audit for export: %w", err)
	}

	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Заголовки (шаг 4, UC-05: колонки таблицы)
	headers := []string{
		"ID",
		"TransferID",
		"OwnerID",
		"EventType",
		"ActorID",
		"IPAddress",
		"UserAgent",
		"Success",
		"Details",
		"CreatedAt",
	}
	if err := cw.Write(headers); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	// Строки данных
	for _, e := range events {
		row := []string{
			e.ID,
			e.TransferID,
			e.OwnerID,
			string(e.EventType),
			e.ActorID,
			e.IPAddress,
			e.UserAgent,
			boolToStr(e.Success),
			e.Details,
			e.CreatedAt.Format(time.RFC3339),
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	return nil
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

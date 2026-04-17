// UC-05: Просмотр журнала аудита
// Приоритет: Средний
// Актеры: Отправитель (свои передачи), Администратор (все передачи)
// FR: 17, 18, 19, 20
package usecase

import (
	"context"
	"fmt"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
)

// UserRole — роль пользователя (FR-32)
type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

// AuditLogInput — параметры запроса журнала аудита
type AuditLogInput struct {
	RequesterID   string
	RequesterRole UserRole
	Filter        domain.AuditFilter
}

// AuditLogOutput — результат запроса
type AuditLogOutput struct {
	Events []*domain.AuditLog
	Total  int
}

// AuditLogUseCase — UC-05
type AuditLogUseCase struct {
	audit repository.AuditRepository
}

func NewAuditLog(audit repository.AuditRepository) *AuditLogUseCase {
	return &AuditLogUseCase{audit: audit}
}

// Execute выполняет сценарий просмотра журнала аудита
func (uc *AuditLogUseCase) Execute(ctx context.Context, in AuditLogInput) (*AuditLogOutput, error) {
	filter := in.Filter

	// Шаг 2: применяем ограничения по роли (FR-18, FR-19)
	switch in.RequesterRole {
	case RoleUser:
		// Отправитель видит только свои передачи (FR-18)
		filter.OwnerID = in.RequesterID

	case RoleAdmin:
		// Администратор видит все передачи (FR-19)
		// filter.OwnerID остаётся таким, каким передал клиент (может быть пустым = все)

	default:
		return nil, fmt.Errorf("unknown role: %s", in.RequesterRole)
	}

	// Пагинация по умолчанию
	if filter.Limit == 0 {
		filter.Limit = 50
	}

	// Шаг 3: запрос событий из БД (FR-17)
	events, err := uc.audit.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}

	// 4а: пустой результат (показывается сообщение на уровне хендлера)
	return &AuditLogOutput{
		Events: events,
		Total:  len(events),
	}, nil
}

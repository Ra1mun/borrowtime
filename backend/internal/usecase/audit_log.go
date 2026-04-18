package usecase

import (
	"context"
	"fmt"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
)

// UserRole — роль пользователя
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

type AuditLogUseCase struct {
	audit repository.AuditRepository
}

func NewAuditLog(audit repository.AuditRepository) *AuditLogUseCase {
	return &AuditLogUseCase{audit: audit}
}

// Execute выполняет сценарий просмотра журнала аудита
func (uc *AuditLogUseCase) Execute(ctx context.Context, in AuditLogInput) (*AuditLogOutput, error) {
	filter := in.Filter

	switch in.RequesterRole {
	case RoleUser:
		// Отправитель видит только свои передачи (FR-18)
		filter.OwnerID = in.RequesterID

	case RoleAdmin:

	default:
		return nil, fmt.Errorf("unknown role: %s", in.RequesterRole)
	}

	if filter.Limit == 0 {
		filter.Limit = 50
	}

	events, err := uc.audit.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}

	return &AuditLogOutput{
		Events: events,
		Total:  len(events),
	}, nil
}

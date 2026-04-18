package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/borrowtime/server/internal/domain"
)

// AuditRepo — реализация repository.AuditRepository на PostgreSQL
type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

// Append добавляет запись в журнал аудита
func (r *AuditRepo) Append(ctx context.Context, log *domain.AuditLog) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (
			id, transfer_id, owner_id, event_type,
			actor_id, ip_address, user_agent, success, details, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		log.ID,
		nullStr(log.TransferID),
		nullStr(log.OwnerID),
		string(log.EventType),
		log.ActorID,
		log.IPAddress,
		log.UserAgent,
		log.Success,
		log.Details,
		log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// List возвращает записи журнала по фильтру
func (r *AuditRepo) List(ctx context.Context, filter domain.AuditFilter) ([]*domain.AuditLog, error) {
	conditions := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if filter.OwnerID != "" {
		conditions = append(conditions, fmt.Sprintf("owner_id = $%d", argIdx))
		args = append(args, filter.OwnerID)
		argIdx++
	}

	if filter.TransferID != "" {
		conditions = append(conditions, fmt.Sprintf("transfer_id = $%d", argIdx))
		args = append(args, filter.TransferID)
		argIdx++
	}

	if filter.EventType != "" {
		conditions = append(conditions, fmt.Sprintf("event_type = $%d", argIdx))
		args = append(args, string(filter.EventType))
		argIdx++
	}

	if !filter.From.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, filter.From)
		argIdx++
	}

	if !filter.To.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, filter.To)
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT id, COALESCE(transfer_id::text, ''), COALESCE(owner_id::text, ''),
		       event_type, actor_id, ip_address, user_agent, success, details, created_at
		FROM audit_logs
		WHERE %s
		ORDER BY created_at DESC`,
		strings.Join(conditions, " AND "),
	)

	// Пагинация
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var result []*domain.AuditLog
	for rows.Next() {
		var log domain.AuditLog
		var eventType string
		err := rows.Scan(
			&log.ID, &log.TransferID, &log.OwnerID,
			&eventType, &log.ActorID, &log.IPAddress, &log.UserAgent,
			&log.Success, &log.Details, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan audit row: %w", err)
		}
		log.EventType = domain.AuditEventType(eventType)
		result = append(result, &log)
	}

	return result, rows.Err()
}

// nullStr возвращает nil для пустой строки (для UUID-полей, которые могут быть NULL)
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

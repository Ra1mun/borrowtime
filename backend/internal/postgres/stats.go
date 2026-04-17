package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// StatsProvider — реализация usecase.SystemStatsProvider на PostgreSQL (UC-08, FR-23)
type StatsProvider struct {
	pool *pgxpool.Pool
}

func NewStatsProvider(pool *pgxpool.Pool) *StatsProvider {
	return &StatsProvider{pool: pool}
}

// ActiveTransfersCount возвращает число передач со статусом ACTIVE
func (s *StatsProvider) ActiveTransfersCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transfers WHERE status = 'ACTIVE'`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("active transfers count: %w", err)
	}
	return count, nil
}

// TotalStorageBytes возвращает суммарный объём файлов по активным и скачанным передачам
func (s *StatsProvider) TotalStorageBytes(ctx context.Context) (int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(file_size_bytes), 0) FROM transfers WHERE status = 'ACTIVE'`,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("total storage bytes: %w", err)
	}
	return total, nil
}

// SecurityIncidentsCount возвращает число несанкционированных попыток за период (FR-30)
func (s *StatsProvider) SecurityIncidentsCount(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_logs
		 WHERE event_type = 'UNAUTHORIZED_ACCESS' AND created_at >= $1`,
		since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("security incidents count: %w", err)
	}
	return count, nil
}

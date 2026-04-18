package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/borrowtime/server/internal/domain"
)

// SettingsRepo — реализация repository.SettingsRepository на PostgreSQL
type SettingsRepo struct {
	pool *pgxpool.Pool
}

func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo {
	return &SettingsRepo{pool: pool}
}

// Get возвращает текущие глобальные настройки
func (r *SettingsRepo) Get(ctx context.Context) (*domain.GlobalSettings, error) {
	var s domain.GlobalSettings
	var maxRetentionSecs, defaultRetentionSecs int64

	err := r.pool.QueryRow(ctx, `
		SELECT max_file_size_bytes, max_retention_period_secs, default_retention_secs,
		       default_max_downloads, updated_at, updated_by
		FROM global_settings
		WHERE id = 1`,
	).Scan(
		&s.MaxFileSizeBytes,
		&maxRetentionSecs,
		&defaultRetentionSecs,
		&s.DefaultMaxDownloads,
		&s.UpdatedAt,
		&s.UpdatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	s.MaxRetentionPeriod = time.Duration(maxRetentionSecs) * time.Second
	s.DefaultRetention = time.Duration(defaultRetentionSecs) * time.Second
	return &s, nil
}

// Save сохраняет глобальные настройки
func (r *SettingsRepo) Save(ctx context.Context, s *domain.GlobalSettings) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO global_settings (
			id, max_file_size_bytes, max_retention_period_secs, default_retention_secs,
			default_max_downloads, updated_at, updated_by
		) VALUES (1, $1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			max_file_size_bytes       = EXCLUDED.max_file_size_bytes,
			max_retention_period_secs = EXCLUDED.max_retention_period_secs,
			default_retention_secs    = EXCLUDED.default_retention_secs,
			default_max_downloads     = EXCLUDED.default_max_downloads,
			updated_at                = EXCLUDED.updated_at,
			updated_by                = EXCLUDED.updated_by`,
		s.MaxFileSizeBytes,
		int64(s.MaxRetentionPeriod.Seconds()),
		int64(s.DefaultRetention.Seconds()),
		s.DefaultMaxDownloads,
		s.UpdatedAt,
		s.UpdatedBy,
	)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

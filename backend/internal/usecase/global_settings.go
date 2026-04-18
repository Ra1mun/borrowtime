package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/borrowtime/server/internal/domain"
	"github.com/borrowtime/server/internal/repository"
)

// SystemStatsProvider — интерфейс сбора статистики
type SystemStatsProvider interface {
	// ActiveTransfersCount возвращает число активных передач
	ActiveTransfersCount(ctx context.Context) (int64, error)

	// TotalStorageBytes возвращает суммарный объём данных в хранилище
	TotalStorageBytes(ctx context.Context) (int64, error)

	// SecurityIncidentsCount возвращает число несанкционированных попыток за период
	SecurityIncidentsCount(ctx context.Context, since time.Time) (int64, error)
}

type GlobalSettingsUseCase struct {
	settings repository.SettingsRepository
	stats    SystemStatsProvider
}

func NewGlobalSettings(
	settings repository.SettingsRepository,
	stats SystemStatsProvider,
) *GlobalSettingsUseCase {
	return &GlobalSettingsUseCase{settings: settings, stats: stats}
}

// UpdateSettingsInput — новые значения настроек (FR-22)
type UpdateSettingsInput struct {
	AdminID             string
	MaxFileSizeBytes    int64
	MaxRetentionPeriod  time.Duration
	DefaultRetention    time.Duration
	DefaultMaxDownloads int
}

// SystemStats — статистика системы (FR-23)
type SystemStats struct {
	ActiveTransfers        int64
	TotalStorageBytes      int64
	SecurityIncidentsToday int64
}

// GetSettings возвращает текущие глобальные настройки
func (uc *GlobalSettingsUseCase) GetSettings(ctx context.Context) (*domain.GlobalSettings, error) {
	s, err := uc.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	return s, nil
}

// UpdateSettings сохраняет обновлённые настройки (FR-22)
func (uc *GlobalSettingsUseCase) UpdateSettings(ctx context.Context, in UpdateSettingsInput) (*domain.GlobalSettings, error) {
	if in.MaxFileSizeBytes <= 0 || in.MaxRetentionPeriod <= 0 {
		return nil, domain.ErrInvalidSettings
	}

	s := &domain.GlobalSettings{
		MaxFileSizeBytes:    in.MaxFileSizeBytes,
		MaxRetentionPeriod:  in.MaxRetentionPeriod,
		DefaultRetention:    in.DefaultRetention,
		DefaultMaxDownloads: in.DefaultMaxDownloads,
		UpdatedAt:           time.Now().UTC(),
		UpdatedBy:           in.AdminID,
	}

	if err := uc.settings.Save(ctx, s); err != nil {
		return nil, fmt.Errorf("save settings: %w", err)
	}

	return s, nil
}

// GetStats возвращает текущую статистику системы
func (uc *GlobalSettingsUseCase) GetStats(ctx context.Context) (*SystemStats, error) {
	active, err := uc.stats.ActiveTransfersCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("active transfers count: %w", err)
	}

	storageBytes, err := uc.stats.TotalStorageBytes(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage size: %w", err)
	}

	since := time.Now().UTC().Truncate(24 * time.Hour) // с начала суток
	incidents, err := uc.stats.SecurityIncidentsCount(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("incidents count: %w", err)
	}

	return &SystemStats{
		ActiveTransfers:        active,
		TotalStorageBytes:      storageBytes,
		SecurityIncidentsToday: incidents,
	}, nil
}

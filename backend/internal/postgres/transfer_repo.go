package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/borrowtime/server/internal/domain"
)

// TransferRepo — реализация repository.TransferRepository на PostgreSQL
type TransferRepo struct {
	pool *pgxpool.Pool
}

func NewTransferRepo(pool *pgxpool.Pool) *TransferRepo {
	return &TransferRepo{pool: pool}
}

// Create сохраняет новую передачу
func (r *TransferRepo) Create(ctx context.Context, t *domain.Transfer) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO transfers (
			id, owner_id, file_name, file_size_bytes, storage_path, access_token,
			policy_expires_at, policy_max_downloads, policy_require_auth, policy_allowed_emails,
			encryption_alg, encryption_iv, encryption_tag,
			status, download_count, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13,
			$14, $15, $16, $17
		)`,
		t.ID, t.OwnerID, t.FileName, t.FileSizeBytes, t.StoragePath, t.AccessToken,
		nullTime(t.Policy.ExpiresAt), t.Policy.MaxDownloads, t.Policy.RequireAuth, t.Policy.AllowedEmails,
		t.Encryption.Algorithm, t.Encryption.IV, t.Encryption.Tag,
		string(t.Status), t.DownloadCount, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert transfer: %w", err)
	}
	return nil
}

// GetByToken возвращает передачу по токену доступа
func (r *TransferRepo) GetByToken(ctx context.Context, token string) (*domain.Transfer, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, owner_id, file_name, file_size_bytes, storage_path, access_token,
		       policy_expires_at, policy_max_downloads, policy_require_auth, policy_allowed_emails,
		       encryption_alg, encryption_iv, encryption_tag,
		       status, download_count, created_at, updated_at
		FROM transfers
		WHERE access_token = $1`, token)

	t, err := scanTransfer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransferNotFound
		}
		return nil, fmt.Errorf("get by token: %w", err)
	}
	return t, nil
}

// GetByID возвращает передачу по ID
func (r *TransferRepo) GetByID(ctx context.Context, id string) (*domain.Transfer, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, owner_id, file_name, file_size_bytes, storage_path, access_token,
		       policy_expires_at, policy_max_downloads, policy_require_auth, policy_allowed_emails,
		       encryption_alg, encryption_iv, encryption_tag,
		       status, download_count, created_at, updated_at
		FROM transfers
		WHERE id = $1`, id)

	t, err := scanTransfer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransferNotFound
		}
		return nil, fmt.Errorf("get by id: %w", err)
	}
	return t, nil
}

// ListByOwner возвращает все передачи пользователя
func (r *TransferRepo) ListByOwner(ctx context.Context, ownerID string) ([]*domain.Transfer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_id, file_name, file_size_bytes, storage_path, access_token,
		       policy_expires_at, policy_max_downloads, policy_require_auth, policy_allowed_emails,
		       encryption_alg, encryption_iv, encryption_tag,
		       status, download_count, created_at, updated_at
		FROM transfers
		WHERE owner_id = $1
		ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list by owner: %w", err)
	}
	defer rows.Close()

	return collectTransfers(rows)
}

// ListByRecipient возвращает передачи, где email получателя в списке policy_allowed_emails
func (r *TransferRepo) ListByRecipient(ctx context.Context, email string) ([]*domain.Transfer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_id, file_name, file_size_bytes, storage_path, access_token,
		       policy_expires_at, policy_max_downloads, policy_require_auth, policy_allowed_emails,
		       encryption_alg, encryption_iv, encryption_tag,
		       status, download_count, created_at, updated_at
		FROM transfers
		WHERE $1 = ANY(policy_allowed_emails)
		ORDER BY created_at DESC`, email)
	if err != nil {
		return nil, fmt.Errorf("list by recipient: %w", err)
	}
	defer rows.Close()

	return collectTransfers(rows)
}

// UpdateStatus меняет статус передачи
func (r *TransferRepo) UpdateStatus(ctx context.Context, id string, status domain.TransferStatus) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE transfers SET status = $1, updated_at = $2 WHERE id = $3`,
		string(status), time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTransferNotFound
	}
	return nil
}

// IncrementDownloads атомарно увеличивает счётчик скачиваний
func (r *TransferRepo) IncrementDownloads(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE transfers SET download_count = download_count + 1, updated_at = $1 WHERE id = $2`,
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("increment downloads: %w", err)
	}
	return nil
}

// ListExpiredOrLimitReached возвращает ACTIVE-передачи
func (r *TransferRepo) ListExpiredOrLimitReached(ctx context.Context) ([]*domain.Transfer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_id, file_name, file_size_bytes, storage_path, access_token,
		       policy_expires_at, policy_max_downloads, policy_require_auth, policy_allowed_emails,
		       encryption_alg, encryption_iv, encryption_tag,
		       status, download_count, created_at, updated_at
		FROM transfers
		WHERE status = 'ACTIVE'
		  AND (
		      (policy_expires_at IS NOT NULL AND policy_expires_at <= NOW())
		   OR (policy_max_downloads > 0 AND download_count >= policy_max_downloads)
		  )`)
	if err != nil {
		return nil, fmt.Errorf("list expired: %w", err)
	}
	defer rows.Close()

	return collectTransfers(rows)
}

func scanTransfer(row pgx.Row) (*domain.Transfer, error) {
	var t domain.Transfer
	var status string
	var expiresAt *time.Time

	err := row.Scan(
		&t.ID, &t.OwnerID, &t.FileName, &t.FileSizeBytes, &t.StoragePath, &t.AccessToken,
		&expiresAt, &t.Policy.MaxDownloads, &t.Policy.RequireAuth, &t.Policy.AllowedEmails,
		&t.Encryption.Algorithm, &t.Encryption.IV, &t.Encryption.Tag,
		&status, &t.DownloadCount, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Status = domain.TransferStatus(status)
	if expiresAt != nil {
		t.Policy.ExpiresAt = *expiresAt
	}
	return &t, nil
}

func collectTransfers(rows pgx.Rows) ([]*domain.Transfer, error) {
	var result []*domain.Transfer
	for rows.Next() {
		t, err := scanTransfer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan transfer row: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// nullTime возвращает nil для нулевого значения времени
func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

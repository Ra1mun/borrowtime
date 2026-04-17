// Package postgres — реализация UserRepository (UC-01)
package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/borrowtime/server/internal/domain"
)

// UserRepo — postgres-реализация repository.UserRepository
type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, totp_secret, totp_enabled,
		                   failed_attempts, locked_until, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		u.ID, u.Email, u.PasswordHash, string(u.Role),
		u.TOTPSecret, u.TOTPEnabled, u.FailedAttempts, u.LockedUntil,
		u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return fmt.Errorf("%w", domain.ErrEmailTaken)
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.scanUser(ctx, `
		SELECT id, email, password_hash, role, totp_secret, totp_enabled,
		       failed_attempts, locked_until, created_at, updated_at
		FROM users WHERE email = $1`, email)
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return r.scanUser(ctx, `
		SELECT id, email, password_hash, role, totp_secret, totp_enabled,
		       failed_attempts, locked_until, created_at, updated_at
		FROM users WHERE id = $1`, id)
}

func (r *UserRepo) scanUser(ctx context.Context, query string, arg any) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, query, arg)
	u := &domain.User{}
	var role string
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &role,
		&u.TOTPSecret, &u.TOTPEnabled,
		&u.FailedAttempts, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.Role = domain.UserRole(role)
	return u, nil
}

func (r *UserRepo) UpdateFailedAttempts(ctx context.Context, id string, count int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET failed_attempts=$1, updated_at=NOW() WHERE id=$2`, count, id)
	return err
}

func (r *UserRepo) LockUntil(ctx context.Context, id string, until time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET locked_until=$1, updated_at=NOW() WHERE id=$2`, until, id)
	return err
}

func (r *UserRepo) ResetLock(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET failed_attempts=0, locked_until=NULL, updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *UserRepo) UpdateTOTPSecret(ctx context.Context, id string, secret string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET totp_secret=$1, updated_at=NOW() WHERE id=$2`, secret, id)
	return err
}

func (r *UserRepo) SetTOTPEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET totp_enabled=$1, updated_at=NOW() WHERE id=$2`, enabled, id)
	return err
}

func (r *UserRepo) ListAll(ctx context.Context) ([]*domain.User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, password_hash, role, totp_secret, totp_enabled,
		       failed_attempts, locked_until, created_at, updated_at
		FROM users ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list all users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		var role string
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &role,
			&u.TOTPSecret, &u.TOTPEnabled, &u.FailedAttempts, &u.LockedUntil,
			&u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user row: %w", err)
		}
		u.Role = domain.UserRole(role)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) Search(ctx context.Context, query string, limit int) ([]*domain.User, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, password_hash, role, totp_secret, totp_enabled,
		       failed_attempts, locked_until, created_at, updated_at
		FROM users WHERE email ILIKE $1
		ORDER BY email LIMIT $2`, query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		var role string
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &role,
			&u.TOTPSecret, &u.TOTPEnabled, &u.FailedAttempts, &u.LockedUntil,
			&u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user row: %w", err)
		}
		u.Role = domain.UserRole(role)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}

func (r *UserRepo) UpdateRole(ctx context.Context, id string, role domain.UserRole) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET role=$1, updated_at=NOW() WHERE id=$2`, string(role), id)
	return err
}

// --- Refresh tokens ---

func (r *UserRepo) SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		rt.ID, rt.UserID, rt.TokenHash, rt.ExpiresAt, rt.CreatedAt,
	)
	return err
}

func (r *UserRepo) FindRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	rt := &domain.RefreshToken{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM refresh_tokens WHERE token_hash = $1`, tokenHash).
		Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	return rt, nil
}

func (r *UserRepo) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE token_hash=$1`, tokenHash)
	return err
}

func (r *UserRepo) DeleteExpiredRefreshTokens(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE user_id=$1 AND expires_at < NOW()`, userID)
	return err
}

// HashToken возвращает hex-строку SHA-256 от токена (используется для хранения)
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

package postgres

import (
	"context"
	"errors"
	"fmt"

	"oriva/backend-go/models/user"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepo is the pgx-backed user store.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo constructs a UserRepo.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

const userColumns = `id, org_id, email, password_hash, role, created_at, updated_at`

func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	if err := row.Scan(&u.ID, &u.OrgID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// FindByEmail looks a user up by email (case-insensitive via the citext column).
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	q := `SELECT ` + userColumns + ` FROM users WHERE email = $1`
	return scanUser(r.pool.QueryRow(ctx, q, email))
}

// FindByID looks a user up by id.
func (r *UserRepo) FindByID(ctx context.Context, id string) (*user.User, error) {
	q := `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	return scanUser(r.pool.QueryRow(ctx, q, id))
}

// Upsert inserts or updates a user by email and returns the stored row.
// Used by cmd/seed.
func (r *UserRepo) Upsert(ctx context.Context, orgID, email, passwordHash, role string) (*user.User, error) {
	q := `
		INSERT INTO users (org_id, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE
		  SET password_hash = EXCLUDED.password_hash,
		      role          = EXCLUDED.role,
		      org_id        = EXCLUDED.org_id,
		      updated_at    = now()
		RETURNING ` + userColumns
	u, err := scanUser(r.pool.QueryRow(ctx, q, orgID, email, passwordHash, role))
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return u, nil
}

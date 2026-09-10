package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OrgRepo is the pgx-backed organizations store. Slice 1 only needs an upsert
// for seeding; full CRUD lands in a later slice.
type OrgRepo struct {
	pool *pgxpool.Pool
}

// NewOrgRepo constructs an OrgRepo.
func NewOrgRepo(pool *pgxpool.Pool) *OrgRepo { return &OrgRepo{pool: pool} }

// UpsertByName returns the id of the org with the given name, creating it if absent.
func (r *OrgRepo) UpsertByName(ctx context.Context, name string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `SELECT id FROM organizations WHERE name = $1`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	err = r.pool.QueryRow(ctx,
		`INSERT INTO organizations (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert org: %w", err)
	}
	return id, nil
}

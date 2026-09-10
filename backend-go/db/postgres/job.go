package postgres

import (
	"context"
	"errors"

	"oriva/backend-go/models/job"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobRepo is the pgx-backed jobs store. Every query is scoped by org_id.
type JobRepo struct{ pool *pgxpool.Pool }

// NewJobRepo constructs a JobRepo.
func NewJobRepo(pool *pgxpool.Pool) *JobRepo { return &JobRepo{pool: pool} }

const jobColumns = `id, org_id, title, description, created_at, updated_at`

func scanJob(row pgx.Row) (*job.Job, error) {
	var j job.Job
	if err := row.Scan(&j.ID, &j.OrgID, &j.Title, &j.Description, &j.CreatedAt, &j.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &j, nil
}

// Create inserts a job and returns it.
func (r *JobRepo) Create(ctx context.Context, orgID, title, description string) (*job.Job, error) {
	q := `INSERT INTO jobs (org_id, title, description) VALUES ($1, $2, $3) RETURNING ` + jobColumns
	return scanJob(r.pool.QueryRow(ctx, q, orgID, title, description))
}

// ListByOrg returns the org's jobs, newest first.
func (r *JobRepo) ListByOrg(ctx context.Context, orgID string) ([]job.Job, error) {
	q := `SELECT ` + jobColumns + ` FROM jobs WHERE org_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []job.Job
	for rows.Next() {
		var j job.Job
		if err := rows.Scan(&j.ID, &j.OrgID, &j.Title, &j.Description, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// Get returns one job in the org.
func (r *JobRepo) Get(ctx context.Context, orgID, id string) (*job.Job, error) {
	q := `SELECT ` + jobColumns + ` FROM jobs WHERE org_id = $1 AND id = $2`
	return scanJob(r.pool.QueryRow(ctx, q, orgID, id))
}

// Update applies the non-nil fields and returns the updated job.
func (r *JobRepo) Update(ctx context.Context, orgID, id string, title, description *string) (*job.Job, error) {
	q := `
		UPDATE jobs
		SET title       = COALESCE($3, title),
		    description  = COALESCE($4, description),
		    updated_at   = now()
		WHERE org_id = $1 AND id = $2
		RETURNING ` + jobColumns
	return scanJob(r.pool.QueryRow(ctx, q, orgID, id, title, description))
}

// ExistsInOrg reports whether the job belongs to the org.
func (r *JobRepo) ExistsInOrg(ctx context.Context, orgID, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM jobs WHERE org_id = $1 AND id = $2)`, orgID, id).Scan(&exists)
	return exists, err
}

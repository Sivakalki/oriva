package postgres

import (
	"context"
	"errors"

	"oriva/backend-go/models/candidate"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CandidateRepo is the pgx-backed candidates store, scoped by org_id.
type CandidateRepo struct{ pool *pgxpool.Pool }

// NewCandidateRepo constructs a CandidateRepo.
func NewCandidateRepo(pool *pgxpool.Pool) *CandidateRepo { return &CandidateRepo{pool: pool} }

const candColumns = `id, org_id, email, name, resume_text, created_at, updated_at`

func scanCandidate(row pgx.Row) (*candidate.Candidate, error) {
	var c candidate.Candidate
	if err := row.Scan(&c.ID, &c.OrgID, &c.Email, &c.Name, &c.ResumeText, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// Create inserts a candidate. A duplicate (org_id, email) returns ErrConflict.
func (r *CandidateRepo) Create(ctx context.Context, orgID, email, name, resumeText string) (*candidate.Candidate, error) {
	q := `INSERT INTO candidates (org_id, email, name, resume_text) VALUES ($1, $2, $3, $4) RETURNING ` + candColumns
	c, err := scanCandidate(r.pool.QueryRow(ctx, q, orgID, email, name, resumeText))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return nil, ErrConflict
		}
		return nil, err
	}
	return c, nil
}

// ListByOrg returns the org's candidates, newest first.
func (r *CandidateRepo) ListByOrg(ctx context.Context, orgID string) ([]candidate.Candidate, error) {
	q := `SELECT ` + candColumns + ` FROM candidates WHERE org_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []candidate.Candidate
	for rows.Next() {
		var c candidate.Candidate
		if err := rows.Scan(&c.ID, &c.OrgID, &c.Email, &c.Name, &c.ResumeText, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get returns one candidate in the org.
func (r *CandidateRepo) Get(ctx context.Context, orgID, id string) (*candidate.Candidate, error) {
	q := `SELECT ` + candColumns + ` FROM candidates WHERE org_id = $1 AND id = $2`
	return scanCandidate(r.pool.QueryRow(ctx, q, orgID, id))
}

// Update applies the non-nil fields (email is immutable) and returns the row.
func (r *CandidateRepo) Update(ctx context.Context, orgID, id string, name, resumeText *string) (*candidate.Candidate, error) {
	q := `
		UPDATE candidates
		SET name        = COALESCE($3, name),
		    resume_text = COALESCE($4, resume_text),
		    updated_at  = now()
		WHERE org_id = $1 AND id = $2
		RETURNING ` + candColumns
	return scanCandidate(r.pool.QueryRow(ctx, q, orgID, id, name, resumeText))
}

// ExistsInOrg reports whether the candidate belongs to the org.
func (r *CandidateRepo) ExistsInOrg(ctx context.Context, orgID, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM candidates WHERE org_id = $1 AND id = $2)`, orgID, id).Scan(&exists)
	return exists, err
}

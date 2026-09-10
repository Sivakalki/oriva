package postgres

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"oriva/backend-go/models/interview"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InterviewRepo is the pgx-backed interview_sessions store, scoped by org_id.
type InterviewRepo struct{ pool *pgxpool.Pool }

// NewInterviewRepo constructs an InterviewRepo.
func NewInterviewRepo(pool *pgxpool.Pool) *InterviewRepo { return &InterviewRepo{pool: pool} }

// Schedule inserts a session in the "scheduled" state and returns its Detail.
func (r *InterviewRepo) Schedule(ctx context.Context, orgID, jobID, candidateID string, scheduledAt time.Time) (string, error) {
	var id string
	q := `
		INSERT INTO interview_sessions (org_id, job_id, candidate_id, state, scheduled_at)
		VALUES ($1, $2, $3, 'scheduled', $4)
		RETURNING id`
	if err := r.pool.QueryRow(ctx, q, orgID, jobID, candidateID, scheduledAt).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

const detailSelect = `
	SELECT s.id, s.state, ss.label, s.scheduled_at, s.created_at,
	       j.id, j.title,
	       c.id, c.name, c.email
	FROM interview_sessions s
	JOIN jobs           j  ON j.id  = s.job_id
	JOIN candidates     c  ON c.id  = s.candidate_id
	JOIN session_states ss ON ss.name = s.state`

func scanDetail(row pgx.Row) (*interview.Detail, error) {
	var d interview.Detail
	err := row.Scan(
		&d.ID, &d.State, &d.StateLabel, &d.ScheduledAt, &d.CreatedAt,
		&d.Job.ID, &d.Job.Title,
		&d.Candidate.ID, &d.Candidate.Name, &d.Candidate.Email,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// Get returns one interview Detail in the org.
func (r *InterviewRepo) Get(ctx context.Context, orgID, id string) (*interview.Detail, error) {
	q := detailSelect + ` WHERE s.org_id = $1 AND s.id = $2`
	return scanDetail(r.pool.QueryRow(ctx, q, orgID, id))
}

// List returns interview Details for the org, applying any set filters.
func (r *InterviewRepo) List(ctx context.Context, orgID string, f interview.Filter) ([]interview.Detail, error) {
	args := []any{orgID}
	var conds []string
	conds = append(conds, "s.org_id = $1")

	add := func(col, val string) {
		if val == "" {
			return
		}
		args = append(args, val)
		conds = append(conds, col+" = $"+strconv.Itoa(len(args)))
	}
	add("s.state", f.State)
	add("s.job_id", f.JobID)
	add("s.candidate_id", f.CandidateID)

	q := detailSelect + " WHERE " + strings.Join(conds, " AND ") + " ORDER BY s.scheduled_at ASC"
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []interview.Detail
	for rows.Next() {
		var d interview.Detail
		if err := rows.Scan(
			&d.ID, &d.State, &d.StateLabel, &d.ScheduledAt, &d.CreatedAt,
			&d.Job.ID, &d.Job.Title,
			&d.Candidate.ID, &d.Candidate.Name, &d.Candidate.Email,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

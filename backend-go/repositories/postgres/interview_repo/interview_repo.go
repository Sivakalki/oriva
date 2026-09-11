// Package interview_repo is the pgx-backed interview_sessions store.
package interview_repo

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InterviewRepo is the pgx-backed interview_sessions store, scoped by org_id.
type InterviewRepo struct{ pool *pgxpool.Pool }

// New constructs an InterviewRepo.
func New(pool *pgxpool.Pool) *InterviewRepo { return &InterviewRepo{pool: pool} }

// Schedule inserts a session in the "scheduled" state with the given join token.
func (r *InterviewRepo) Schedule(
	ctx context.Context, orgID, jobID, candidateID, joinToken string, scheduledAt time.Time,
) (string, error) {
	var id string
	q := `
		INSERT INTO interview_sessions (org_id, job_id, candidate_id, state, scheduled_at, join_token)
		VALUES ($1, $2, $3, 'scheduled', $4, $5)
		RETURNING id`
	if err := r.pool.QueryRow(ctx, q, orgID, jobID, candidateID, scheduledAt, joinToken).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

const detailSelect = `
	SELECT s.id, s.state, ss.label, s.scheduled_at, s.created_at, s.join_token,
	       j.id, j.title,
	       c.id, c.name, c.email
	FROM interview_sessions s
	JOIN jobs           j  ON j.id  = s.job_id
	JOIN candidates     c  ON c.id  = s.candidate_id
	JOIN session_states ss ON ss.name = s.state`

func scanDetailInto(row pgx.Row, d *interview.Detail) error {
	return row.Scan(
		&d.ID, &d.State, &d.StateLabel, &d.ScheduledAt, &d.CreatedAt, &d.JoinToken,
		&d.Job.ID, &d.Job.Title,
		&d.Candidate.ID, &d.Candidate.Name, &d.Candidate.Email,
	)
}

func scanDetail(row pgx.Row) (*interview.Detail, error) {
	var d interview.Detail
	if err := scanDetailInto(row, &d); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, postgres.ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// JoinInfo is the candidate-facing view resolved from a join token.
type JoinInfo struct {
	SessionID   string
	JobTitle    string
	ScheduledAt time.Time
	State       string
	IsTerminal  bool
}

// JoinByToken resolves a candidate join token, or ErrNotFound.
func (r *InterviewRepo) JoinByToken(ctx context.Context, token string) (*JoinInfo, error) {
	var ji JoinInfo
	err := r.pool.QueryRow(ctx, `
		SELECT s.id, j.title, s.scheduled_at, s.state, ss.is_terminal
		FROM interview_sessions s
		JOIN jobs j ON j.id = s.job_id
		JOIN session_states ss ON ss.name = s.state
		WHERE s.join_token = $1`, token).
		Scan(&ji.SessionID, &ji.JobTitle, &ji.ScheduledAt, &ji.State, &ji.IsTerminal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, postgres.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ji, nil
}

// Get returns one interview Detail in the org.
func (r *InterviewRepo) Get(ctx context.Context, orgID, id string) (*interview.Detail, error) {
	q := detailSelect + ` WHERE s.org_id = $1 AND s.id = $2`
	return scanDetail(r.pool.QueryRow(ctx, q, orgID, id))
}

// OrgOf returns the organization that owns the session, or ErrNotFound.
func (r *InterviewRepo) OrgOf(ctx context.Context, sessionID string) (string, error) {
	var orgID string
	err := r.pool.QueryRow(ctx,
		`SELECT org_id FROM interview_sessions WHERE id = $1`, sessionID).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", postgres.ErrNotFound
	}
	return orgID, err
}

// PlanData is the joined job/candidate/state view used by the get_interview_plan tool.
type PlanData struct {
	State           string
	JobTitle        string
	JobDescription  string
	CandidateName   string
	CandidateResume string
}

// PlanData returns the plan inputs for a session in the org, or ErrNotFound.
func (r *InterviewRepo) PlanData(ctx context.Context, orgID, sessionID string) (*PlanData, error) {
	var p PlanData
	err := r.pool.QueryRow(ctx, `
		SELECT s.state, j.title, j.description, c.name, c.resume_text
		FROM interview_sessions s
		JOIN jobs j ON j.id = s.job_id
		JOIN candidates c ON c.id = s.candidate_id
		WHERE s.org_id = $1 AND s.id = $2`, orgID, sessionID).
		Scan(&p.State, &p.JobTitle, &p.JobDescription, &p.CandidateName, &p.CandidateResume)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, postgres.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CurrentState returns the session's current state, or ErrNotFound.
func (r *InterviewRepo) CurrentState(ctx context.Context, orgID, id string) (string, error) {
	var state string
	err := r.pool.QueryRow(ctx,
		`SELECT state FROM interview_sessions WHERE org_id = $1 AND id = $2`, orgID, id).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", postgres.ErrNotFound
	}
	return state, err
}

// ApplyTransition moves the session from -> to and appends an audit event,
// atomically. It returns ErrNotFound if the session does not exist in the org,
// or ErrConflict if the row is no longer in state `from` (lost CAS race).
func (r *InterviewRepo) ApplyTransition(ctx context.Context, orgID, id, from, to, reason, actor string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE interview_sessions SET state = $4, updated_at = now()
		 WHERE org_id = $1 AND id = $2 AND state = $3`,
		orgID, id, from, to)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM interview_sessions WHERE org_id = $1 AND id = $2)`,
			orgID, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return postgres.ErrNotFound
		}
		return postgres.ErrConflict
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO session_state_events (session_id, org_id, from_state, to_state, reason, actor)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, orgID, from, to, reason, actor); err != nil {
		return err
	}

	return tx.Commit(ctx)
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
		if err := scanDetailInto(rows, &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

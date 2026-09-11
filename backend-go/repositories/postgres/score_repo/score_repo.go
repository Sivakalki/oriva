// Package score_repo is the pgx-backed scores store: one row per scored
// interview turn, plus one "overall" row per session once every turn is
// scored (services/scoring).
package score_repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ScoreRepo is the pgx-backed scores store, scoped by session_id.
type ScoreRepo struct{ pool *pgxpool.Pool }

// New constructs a ScoreRepo.
func New(pool *pgxpool.Pool) *ScoreRepo { return &ScoreRepo{pool: pool} }

// TurnScore is one scored turn, with the question/answer it scored (for
// building the overall-scoring prompt).
type TurnScore struct {
	TurnIndex int
	Question  string
	Answer    string
	Value     float64
	Rationale string
}

// RecordTurnScore upserts the score for one turn. Re-scoring the same turn
// overwrites the previous value.
func (r *ScoreRepo) RecordTurnScore(
	ctx context.Context, sessionID string, turnIndex int, value float64, rationale, model string,
) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scores (session_id, rubric_item, value, rationale, turn_index, model)
		VALUES ($1, 'turn', $2, $3, $4, $5)
		ON CONFLICT (session_id, turn_index) WHERE turn_index IS NOT NULL
		DO UPDATE SET value = EXCLUDED.value, rationale = EXCLUDED.rationale,
		              model = EXCLUDED.model, updated_at = now()`,
		sessionID, value, rationale, turnIndex, model)
	return err
}

// RecordOverallScore upserts the session's overall score.
func (r *ScoreRepo) RecordOverallScore(
	ctx context.Context, sessionID string, value float64, rationale, model string,
) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scores (session_id, rubric_item, value, rationale, turn_index, model)
		VALUES ($1, 'overall', $2, $3, NULL, $4)
		ON CONFLICT (session_id) WHERE turn_index IS NULL AND rubric_item = 'overall'
		DO UPDATE SET value = EXCLUDED.value, rationale = EXCLUDED.rationale,
		              model = EXCLUDED.model, updated_at = now()`,
		sessionID, value, rationale, model)
	return err
}

// TurnScores returns every scored turn for the session, joined with its
// question/answer, ordered by turn_index.
func (r *ScoreRepo) TurnScores(ctx context.Context, sessionID string) ([]TurnScore, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.turn_index, r.question, r.answer, s.value, s.rationale
		FROM scores s
		JOIN responses r ON r.session_id = s.session_id AND r.turn_index = s.turn_index
		WHERE s.session_id = $1 AND s.rubric_item = 'turn'
		ORDER BY s.turn_index ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TurnScore
	for rows.Next() {
		var t TurnScore
		if err := rows.Scan(&t.TurnIndex, &t.Question, &t.Answer, &t.Value, &t.Rationale); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// HasOverallScore reports whether the session already has an overall score
// (idempotency guard: ScoreOverall need not re-run if this is true, though
// re-running is harmless — RecordOverallScore upserts).
func (r *ScoreRepo) HasOverallScore(ctx context.Context, sessionID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM scores WHERE session_id = $1 AND rubric_item = 'overall')`,
		sessionID).Scan(&exists)
	return exists, err
}

// OverallScore is the session's aggregate score, or ok=false if not scored yet.
type OverallScore struct {
	Value     float64
	Rationale string
	Model     string
	CreatedAt time.Time
}

// GetOverallScore returns the session's overall score, if any.
func (r *ScoreRepo) GetOverallScore(ctx context.Context, sessionID string) (*OverallScore, error) {
	var s OverallScore
	err := r.pool.QueryRow(ctx, `
		SELECT value, rationale, model, created_at FROM scores
		WHERE session_id = $1 AND rubric_item = 'overall'`, sessionID).
		Scan(&s.Value, &s.Rationale, &s.Model, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

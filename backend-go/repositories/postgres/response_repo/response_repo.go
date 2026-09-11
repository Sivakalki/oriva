// Package response_repo is the pgx-backed transcript-turn store.
package response_repo

import (
	"context"
	"errors"
	"fmt"

	"oriva/backend-go/repositories/postgres"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResponseRepo is the pgx-backed transcript-turn store.
type ResponseRepo struct{ pool *pgxpool.Pool }

// New constructs a ResponseRepo.
func New(pool *pgxpool.Pool) *ResponseRepo { return &ResponseRepo{pool: pool} }

// Record inserts a turn. If turnIndex <= 0 it appends after the last turn for the
// session (first turn is 1). Returns the stored turn_index. A unique
// (session_id, turn_index) collision returns ErrConflict.
func (r *ResponseRepo) Record(
	ctx context.Context, sessionID string, turnIndex int, question, answer string,
) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	idx := turnIndex
	if idx <= 0 {
		if err := tx.QueryRow(ctx,
			`SELECT coalesce(max(turn_index), 0) + 1 FROM responses WHERE session_id = $1`,
			sessionID).Scan(&idx); err != nil {
			return 0, err
		}
	}

	var stored int
	err = tx.QueryRow(ctx,
		`INSERT INTO responses (session_id, turn_index, question, answer)
		 VALUES ($1, $2, $3, $4) RETURNING turn_index`,
		sessionID, idx, question, answer).Scan(&stored)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == postgres.PgUniqueViolation {
			return 0, postgres.ErrConflict
		}
		return 0, fmt.Errorf("insert response: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return stored, nil
}

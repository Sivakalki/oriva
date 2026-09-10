//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// truncateData clears all mutable rows (keeping the seeded session_states) so a
// test starts from a known state and does not leave FK references that would
// block a later `migrate down`.
func truncateData(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`TRUNCATE session_state_events, interview_sessions, responses, scores, candidates, jobs, users, organizations RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

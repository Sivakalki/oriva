//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"oriva/backend-go/db/postgres"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGraph(t *testing.T) {
	pool := testPool(t)
	states, transitions, err := postgres.LoadGraph(context.Background(), pool)
	require.NoError(t, err)
	assert.Len(t, states, 12)
	assert.Len(t, transitions, 21)

	terminal := map[string]bool{}
	for _, s := range states {
		terminal[s.Name] = s.IsTerminal
	}
	assert.True(t, terminal["scored"])
	assert.False(t, terminal["ready"])
}

func TestApplyTransition_CASAndAudit(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	org := newOrg(t, pool, "sm-org")

	job, err := postgres.NewJobRepo(pool).Create(ctx, org, "Role", "")
	require.NoError(t, err)
	cand, err := postgres.NewCandidateRepo(pool).Create(ctx, org, "sm@example.com", "SM", "")
	require.NoError(t, err)

	repo := postgres.NewInterviewRepo(pool)
	id, err := repo.Schedule(ctx, org, job.ID, cand.ID, time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	cur, err := repo.CurrentState(ctx, org, id)
	require.NoError(t, err)
	assert.Equal(t, "scheduled", cur)

	require.NoError(t, repo.ApplyTransition(ctx, org, id, "scheduled", "invited", "sent invite", "u1"))

	cur, err = repo.CurrentState(ctx, org, id)
	require.NoError(t, err)
	assert.Equal(t, "invited", cur)

	// A stale second attempt from "scheduled" now loses the CAS.
	err = repo.ApplyTransition(ctx, org, id, "scheduled", "invited", "", "u2")
	assert.ErrorIs(t, err, postgres.ErrConflict)

	// Unknown session id.
	err = repo.ApplyTransition(ctx, org, "00000000-0000-0000-0000-000000000000",
		"scheduled", "invited", "", "u1")
	assert.ErrorIs(t, err, postgres.ErrNotFound)

	var events int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM session_state_events WHERE session_id = $1`, id).Scan(&events))
	assert.Equal(t, 1, events)

	var from, to, actor string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT from_state, to_state, actor FROM session_state_events WHERE session_id = $1`, id).
		Scan(&from, &to, &actor))
	assert.Equal(t, "scheduled", from)
	assert.Equal(t, "invited", to)
	assert.Equal(t, "u1", actor)
}

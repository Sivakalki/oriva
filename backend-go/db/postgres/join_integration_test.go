//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"oriva/backend-go/db/postgres"
	"oriva/backend-go/jointoken"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterviewRepo_JoinByToken(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	org := newOrg(t, pool, "join-org")

	job, err := postgres.NewJobRepo(pool).Create(ctx, org, "Backend Eng", "")
	require.NoError(t, err)
	cand, err := postgres.NewCandidateRepo(pool).Create(ctx, org, "j@example.com", "J", "")
	require.NoError(t, err)

	repo := postgres.NewInterviewRepo(pool)
	at := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	tok1 := jointoken.New()
	sid1, err := repo.Schedule(ctx, org, job.ID, cand.ID, tok1, at)
	require.NoError(t, err)
	tok2 := jointoken.New()
	_, err = repo.Schedule(ctx, org, job.ID, cand.ID, tok2, at)
	require.NoError(t, err)
	assert.NotEqual(t, tok1, tok2)

	info, err := repo.JoinByToken(ctx, tok1)
	require.NoError(t, err)
	assert.Equal(t, sid1, info.SessionID)
	assert.Equal(t, "Backend Eng", info.JobTitle)
	assert.Equal(t, "scheduled", info.State)
	assert.False(t, info.IsTerminal)
	assert.WithinDuration(t, at, info.ScheduledAt, time.Second)

	_, err = repo.JoinByToken(ctx, "no-such-token")
	assert.ErrorIs(t, err, postgres.ErrNotFound)
}

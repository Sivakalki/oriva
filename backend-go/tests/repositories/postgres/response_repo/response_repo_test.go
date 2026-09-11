//go:build integration

package response_repo_test

import (
	"context"
	"testing"
	"time"

	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/candidate_repo"
	"oriva/backend-go/repositories/postgres/interview_repo"
	"oriva/backend-go/repositories/postgres/job_repo"
	"oriva/backend-go/repositories/postgres/response_repo"
	"oriva/backend-go/tests/repositories/postgres/testutil"
	"oriva/backend-go/utils/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseRepo_RecordAndConflict(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	org := testutil.NewOrg(t, pool, "response-repo-org")

	job, err := job_repo.New(pool).Create(ctx, org, "Backend Eng", "Go + pg")
	require.NoError(t, err)
	cand, err := candidate_repo.New(pool).Create(ctx, org, "resp@example.com", "Mac", "5y")
	require.NoError(t, err)

	iv := interview_repo.New(pool)
	sid, err := iv.Schedule(ctx, org, job.ID, cand.ID, helpers.NewJoinToken(), time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	rr := response_repo.New(pool)
	i1, err := rr.Record(ctx, sid, 0, "q1", "a1")
	require.NoError(t, err)
	assert.Equal(t, 1, i1)

	i2, err := rr.Record(ctx, sid, 0, "q2", "a2")
	require.NoError(t, err)
	assert.Equal(t, 2, i2)

	// Explicit collision
	_, err = rr.Record(ctx, sid, 1, "dup", "dup")
	assert.ErrorIs(t, err, postgres.ErrConflict)
}

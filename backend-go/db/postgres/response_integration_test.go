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

func TestResponseRepo_RecordAndOrgHelpers(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	org := newOrg(t, pool, "mcp-org")

	job, err := postgres.NewJobRepo(pool).Create(ctx, org, "Backend Eng", "Go + pg")
	require.NoError(t, err)
	cand, err := postgres.NewCandidateRepo(pool).Create(ctx, org, "mcp@example.com", "Mac", "5y")
	require.NoError(t, err)

	iv := postgres.NewInterviewRepo(pool)
	sid, err := iv.Schedule(ctx, org, job.ID, cand.ID, jointoken.New(), time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	// OrgOf
	gotOrg, err := iv.OrgOf(ctx, sid)
	require.NoError(t, err)
	assert.Equal(t, org, gotOrg)
	_, err = iv.OrgOf(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, postgres.ErrNotFound)

	// PlanData
	plan, err := iv.PlanData(ctx, org, sid)
	require.NoError(t, err)
	assert.Equal(t, "Backend Eng", plan.JobTitle)
	assert.Equal(t, "Mac", plan.CandidateName)
	assert.Equal(t, "scheduled", plan.State)

	// Record auto-increment
	rr := postgres.NewResponseRepo(pool)
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

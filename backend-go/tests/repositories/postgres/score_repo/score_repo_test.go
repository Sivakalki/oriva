//go:build integration

package score_repo_test

import (
	"context"
	"testing"
	"time"

	"oriva/backend-go/repositories/postgres/candidate_repo"
	"oriva/backend-go/repositories/postgres/interview_repo"
	"oriva/backend-go/repositories/postgres/job_repo"
	"oriva/backend-go/repositories/postgres/response_repo"
	"oriva/backend-go/repositories/postgres/score_repo"
	"oriva/backend-go/tests/repositories/postgres/testutil"
	"oriva/backend-go/utils/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoreRepo_TurnScoresAndOverall(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	org := testutil.NewOrg(t, pool, "score-repo-org")

	job, err := job_repo.New(pool).Create(ctx, org, "Backend Eng", "Go + pg")
	require.NoError(t, err)
	cand, err := candidate_repo.New(pool).Create(ctx, org, "score@example.com", "Sam", "5y")
	require.NoError(t, err)

	iv := interview_repo.New(pool)
	sid, err := iv.Schedule(ctx, org, job.ID, cand.ID, helpers.NewJoinToken(), time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	rr := response_repo.New(pool)
	idx1, err := rr.Record(ctx, sid, 0, "Tell me about yourself", "I have 5 years of Go experience")
	require.NoError(t, err)
	idx2, err := rr.Record(ctx, sid, 0, "Why this role?", "I like the problem space")
	require.NoError(t, err)

	sr := score_repo.New(pool)
	require.NoError(t, sr.RecordTurnScore(ctx, sid, idx1, 80, "solid, specific", "test-model"))
	require.NoError(t, sr.RecordTurnScore(ctx, sid, idx2, 60, "a bit generic", "test-model"))

	// Re-scoring the same turn overwrites, not accumulates.
	require.NoError(t, sr.RecordTurnScore(ctx, sid, idx1, 90, "even better on reflection", "test-model"))

	turns, err := sr.TurnScores(ctx, sid)
	require.NoError(t, err)
	require.Len(t, turns, 2)
	assert.Equal(t, 90.0, turns[0].Value)
	assert.Equal(t, "even better on reflection", turns[0].Rationale)
	assert.Equal(t, "Tell me about yourself", turns[0].Question)
	assert.Equal(t, 60.0, turns[1].Value)

	has, err := sr.HasOverallScore(ctx, sid)
	require.NoError(t, err)
	assert.False(t, has)

	require.NoError(t, sr.RecordOverallScore(ctx, sid, 75, "good overall, one weak spot", "test-model"))

	has, err = sr.HasOverallScore(ctx, sid)
	require.NoError(t, err)
	assert.True(t, has)

	overall, err := sr.GetOverallScore(ctx, sid)
	require.NoError(t, err)
	assert.Equal(t, 75.0, overall.Value)
	assert.Equal(t, "good overall, one weak spot", overall.Rationale)

	// Re-scoring overall overwrites too.
	require.NoError(t, sr.RecordOverallScore(ctx, sid, 82, "revised", "test-model"))
	overall, err = sr.GetOverallScore(ctx, sid)
	require.NoError(t, err)
	assert.Equal(t, 82.0, overall.Value)

	// Overall score doesn't show up in TurnScores.
	turns, err = sr.TurnScores(ctx, sid)
	require.NoError(t, err)
	assert.Len(t, turns, 2)
}

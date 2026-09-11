//go:build integration

package interview_repo_test

import (
	"context"
	"testing"
	"time"

	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/candidate_repo"
	"oriva/backend-go/repositories/postgres/interview_repo"
	"oriva/backend-go/repositories/postgres/job_repo"
	"oriva/backend-go/tests/repositories/postgres/testutil"
	"oriva/backend-go/utils/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterviewRepo_ScheduleGetList(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	org := testutil.NewOrg(t, pool, "crud-interview")

	j, err := job_repo.New(pool).Create(ctx, org, "Role", "")
	require.NoError(t, err)
	c, err := candidate_repo.New(pool).Create(ctx, org, "iv@example.com", "Iv", "")
	require.NoError(t, err)

	repo := interview_repo.New(pool)
	at := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	id, err := repo.Schedule(ctx, org, j.ID, c.ID, helpers.NewJoinToken(), at, 30)
	require.NoError(t, err)

	d, err := repo.Get(ctx, org, id)
	require.NoError(t, err)
	assert.Equal(t, "scheduled", d.State)
	assert.Equal(t, "Scheduled", d.StateLabel)
	assert.Equal(t, j.ID, d.Job.ID)
	assert.Equal(t, "iv@example.com", d.Candidate.Email)

	list, err := repo.List(ctx, org, interview.Filter{State: "scheduled", JobID: j.ID})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	none, err := repo.List(ctx, org, interview.Filter{State: "completed"})
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestInterviewRepo_JoinByToken(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	org := testutil.NewOrg(t, pool, "join-org")

	job, err := job_repo.New(pool).Create(ctx, org, "Backend Eng", "")
	require.NoError(t, err)
	cand, err := candidate_repo.New(pool).Create(ctx, org, "j@example.com", "J", "")
	require.NoError(t, err)

	repo := interview_repo.New(pool)
	at := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)

	tok1 := helpers.NewJoinToken()
	sid1, err := repo.Schedule(ctx, org, job.ID, cand.ID, tok1, at, 30)
	require.NoError(t, err)
	tok2 := helpers.NewJoinToken()
	_, err = repo.Schedule(ctx, org, job.ID, cand.ID, tok2, at, 30)
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

func TestInterviewRepo_ApplyTransition_CASAndAudit(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	org := testutil.NewOrg(t, pool, "sm-org")

	job, err := job_repo.New(pool).Create(ctx, org, "Role", "")
	require.NoError(t, err)
	cand, err := candidate_repo.New(pool).Create(ctx, org, "sm@example.com", "SM", "")
	require.NoError(t, err)

	repo := interview_repo.New(pool)
	id, err := repo.Schedule(ctx, org, job.ID, cand.ID, helpers.NewJoinToken(), time.Now().Add(24*time.Hour), 30)
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

func TestInterviewRepo_OrgOfAndPlanData(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	org := testutil.NewOrg(t, pool, "mcp-org")

	job, err := job_repo.New(pool).Create(ctx, org, "Backend Eng", "Go + pg")
	require.NoError(t, err)
	cand, err := candidate_repo.New(pool).Create(ctx, org, "mcp@example.com", "Mac", "5y")
	require.NoError(t, err)

	iv := interview_repo.New(pool)
	sid, err := iv.Schedule(ctx, org, job.ID, cand.ID, helpers.NewJoinToken(), time.Now().Add(24*time.Hour), 30)
	require.NoError(t, err)

	gotOrg, err := iv.OrgOf(ctx, sid)
	require.NoError(t, err)
	assert.Equal(t, org, gotOrg)
	_, err = iv.OrgOf(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, postgres.ErrNotFound)

	plan, err := iv.PlanData(ctx, org, sid)
	require.NoError(t, err)
	assert.Equal(t, "Backend Eng", plan.JobTitle)
	assert.Equal(t, "Mac", plan.CandidateName)
	assert.Equal(t, "scheduled", plan.State)
}

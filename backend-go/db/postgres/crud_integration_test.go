//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"oriva/backend-go/config"
	"oriva/backend-go/db/postgres"
	"oriva/backend-go/jointoken"
	"oriva/backend-go/models/interview"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	d := dsn(t)
	require.NoError(t, postgres.RunUp(d))
	pool, err := postgres.Connect(context.Background(), config.Postgres{DSN: d, MaxConns: 4})
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	truncateData(t, pool)
	t.Cleanup(func() { truncateData(t, pool) })
	return pool
}

func newOrg(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	id, err := postgres.NewOrgRepo(pool).UpsertByName(context.Background(), name)
	require.NoError(t, err)
	return id
}

func TestJobRepo_CRUDAndOrgScope(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	orgA := newOrg(t, pool, "crud-jobs-A")
	orgB := newOrg(t, pool, "crud-jobs-B")
	repo := postgres.NewJobRepo(pool)

	j, err := repo.Create(ctx, orgA, "Backend Eng", "desc")
	require.NoError(t, err)

	got, err := repo.Get(ctx, orgA, j.ID)
	require.NoError(t, err)
	assert.Equal(t, "Backend Eng", got.Title)

	newTitle := "Senior Backend Eng"
	upd, err := repo.Update(ctx, orgA, j.ID, &newTitle, nil)
	require.NoError(t, err)
	assert.Equal(t, "Senior Backend Eng", upd.Title)
	assert.Equal(t, "desc", upd.Description)

	_, err = repo.Get(ctx, orgB, j.ID)
	assert.ErrorIs(t, err, postgres.ErrNotFound)

	ok, err := repo.ExistsInOrg(ctx, orgB, j.ID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCandidateRepo_Conflict(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	orgA := newOrg(t, pool, "crud-cand-A")
	orgB := newOrg(t, pool, "crud-cand-B")
	repo := postgres.NewCandidateRepo(pool)

	_, err := repo.Create(ctx, orgA, "dup@example.com", "A", "")
	require.NoError(t, err)

	_, err = repo.Create(ctx, orgA, "dup@example.com", "A2", "")
	assert.ErrorIs(t, err, postgres.ErrConflict)

	// same email, different org is fine
	_, err = repo.Create(ctx, orgB, "dup@example.com", "B", "")
	require.NoError(t, err)
}

func TestInterviewRepo_ScheduleGetList(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	org := newOrg(t, pool, "crud-interview")

	j, err := postgres.NewJobRepo(pool).Create(ctx, org, "Role", "")
	require.NoError(t, err)
	c, err := postgres.NewCandidateRepo(pool).Create(ctx, org, "iv@example.com", "Iv", "")
	require.NoError(t, err)

	repo := postgres.NewInterviewRepo(pool)
	at := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	id, err := repo.Schedule(ctx, org, j.ID, c.ID, jointoken.New(), at)
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

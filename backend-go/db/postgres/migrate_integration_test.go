//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"oriva/backend-go/config"
	"oriva/backend-go/db/postgres"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dsn(t *testing.T) string {
	t.Helper()
	d := os.Getenv("TEST_DATABASE_URL")
	if d == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return d
}

func TestMigrations_UpSeedsStates(t *testing.T) {
	d := dsn(t)
	require.NoError(t, postgres.RunDown(d))
	require.NoError(t, postgres.RunUp(d))

	ctx := context.Background()
	pool, err := postgres.Connect(ctx, config.Postgres{DSN: d, MaxConns: 2})
	require.NoError(t, err)
	defer pool.Close()

	var stateCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM session_states`).Scan(&stateCount))
	assert.Equal(t, 12, stateCount)

	var terminalCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM session_states WHERE is_terminal`).Scan(&terminalCount))
	assert.Equal(t, 4, terminalCount)

	var rejoin int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM session_state_transitions WHERE from_state='interrupted' AND to_state='dispatched'`).Scan(&rejoin))
	assert.Equal(t, 1, rejoin)

	var fromTerminal int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM session_state_transitions
		 WHERE from_state IN ('scored','declined','abandoned','failed')`).Scan(&fromTerminal))
	assert.Equal(t, 0, fromTerminal)
}

func TestMigrations_DownUpIdempotent(t *testing.T) {
	d := dsn(t)
	require.NoError(t, postgres.RunDown(d))
	require.NoError(t, postgres.RunUp(d))
	require.NoError(t, postgres.RunDown(d))
	require.NoError(t, postgres.RunUp(d))
}

func TestUserRepo_FindByEmail(t *testing.T) {
	d := dsn(t)
	require.NoError(t, postgres.RunUp(d))

	ctx := context.Background()
	pool, err := postgres.Connect(ctx, config.Postgres{DSN: d, MaxConns: 2})
	require.NoError(t, err)
	defer pool.Close()

	orgID, err := postgres.NewOrgRepo(pool).UpsertByName(ctx, "IntegrationOrg")
	require.NoError(t, err)

	repo := postgres.NewUserRepo(pool)
	_, err = repo.Upsert(ctx, orgID, "int-test@example.com", "hash", "scheduler")
	require.NoError(t, err)

	got, err := repo.FindByEmail(ctx, "INT-TEST@example.com") // citext: case-insensitive
	require.NoError(t, err)
	assert.Equal(t, "scheduler", got.Role)

	_, err = repo.FindByEmail(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, postgres.ErrNotFound)
}

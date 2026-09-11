//go:build integration

package migrate_test

import (
	"context"
	"testing"

	"oriva/backend-go/config"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/tests/repositories/postgres/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrations_UpSeedsStates(t *testing.T) {
	d := testutil.DSN(t)
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
	d := testutil.DSN(t)
	require.NoError(t, postgres.RunDown(d))
	require.NoError(t, postgres.RunUp(d))
	require.NoError(t, postgres.RunDown(d))
	require.NoError(t, postgres.RunUp(d))
}

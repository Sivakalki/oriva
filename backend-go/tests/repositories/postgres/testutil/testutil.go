//go:build integration

// Package testutil holds shared fixtures for the repositories/postgres/*
// integration test suites: a truncated, migrated pool and a helper to seed
// an organization row.
package testutil

import (
	"context"
	"os"
	"testing"

	"oriva/backend-go/config"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/org_repo"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// DSN returns TEST_DATABASE_URL, skipping the test if it is unset.
func DSN(t *testing.T) string {
	t.Helper()
	d := os.Getenv("TEST_DATABASE_URL")
	if d == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return d
}

// TestPool migrates up, connects, truncates mutable data before and after
// the test, and returns the pool.
func TestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	d := DSN(t)
	require.NoError(t, postgres.RunUp(d))
	pool, err := postgres.Connect(context.Background(), config.Postgres{DSN: d, MaxConns: 4})
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	TruncateData(t, pool)
	t.Cleanup(func() { TruncateData(t, pool) })
	return pool
}

// NewOrg seeds (or reuses) an organization row and returns its id.
func NewOrg(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	id, err := org_repo.New(pool).UpsertByName(context.Background(), name)
	require.NoError(t, err)
	return id
}

// TruncateData clears all mutable rows (keeping the seeded session_states) so a
// test starts from a known state and does not leave FK references that would
// block a later `migrate down`.
func TruncateData(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`TRUNCATE session_state_events, interview_sessions, responses, scores, candidates, jobs, users, organizations RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

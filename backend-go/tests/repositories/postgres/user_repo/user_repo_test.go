//go:build integration

package user_repo_test

import (
	"context"
	"testing"

	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/user_repo"
	"oriva/backend-go/tests/repositories/postgres/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepo_FindByEmail(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	orgID := testutil.NewOrg(t, pool, "IntegrationOrg")

	repo := user_repo.New(pool)
	_, err := repo.Upsert(ctx, orgID, "int-test@example.com", "hash", "scheduler")
	require.NoError(t, err)

	got, err := repo.FindByEmail(ctx, "INT-TEST@example.com") // citext: case-insensitive
	require.NoError(t, err)
	assert.Equal(t, "scheduler", got.Role)

	_, err = repo.FindByEmail(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, postgres.ErrNotFound)
}

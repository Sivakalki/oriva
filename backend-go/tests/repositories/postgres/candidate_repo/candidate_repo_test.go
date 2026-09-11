//go:build integration

package candidate_repo_test

import (
	"context"
	"testing"

	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/candidate_repo"
	"oriva/backend-go/tests/repositories/postgres/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCandidateRepo_Conflict(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	orgA := testutil.NewOrg(t, pool, "crud-cand-A")
	orgB := testutil.NewOrg(t, pool, "crud-cand-B")
	repo := candidate_repo.New(pool)

	_, err := repo.Create(ctx, orgA, "dup@example.com", "A", "")
	require.NoError(t, err)

	_, err = repo.Create(ctx, orgA, "dup@example.com", "A2", "")
	assert.ErrorIs(t, err, postgres.ErrConflict)

	// same email, different org is fine
	_, err = repo.Create(ctx, orgB, "dup@example.com", "B", "")
	require.NoError(t, err)
}

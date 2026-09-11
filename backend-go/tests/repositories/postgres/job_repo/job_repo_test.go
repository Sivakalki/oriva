//go:build integration

package job_repo_test

import (
	"context"
	"testing"

	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/job_repo"
	"oriva/backend-go/tests/repositories/postgres/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobRepo_CRUDAndOrgScope(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	orgA := testutil.NewOrg(t, pool, "crud-jobs-A")
	orgB := testutil.NewOrg(t, pool, "crud-jobs-B")
	repo := job_repo.New(pool)

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

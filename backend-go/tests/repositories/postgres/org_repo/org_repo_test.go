//go:build integration

package org_repo_test

import (
	"context"
	"testing"

	"oriva/backend-go/repositories/postgres/org_repo"
	"oriva/backend-go/tests/repositories/postgres/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgRepo_UpsertByName_Idempotent(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestPool(t)
	repo := org_repo.New(pool)

	id1, err := repo.UpsertByName(ctx, "org-repo-test")
	require.NoError(t, err)

	id2, err := repo.UpsertByName(ctx, "org-repo-test")
	require.NoError(t, err)
	assert.Equal(t, id1, id2)

	id3, err := repo.UpsertByName(ctx, "org-repo-test-other")
	require.NoError(t, err)
	assert.NotEqual(t, id1, id3)
}

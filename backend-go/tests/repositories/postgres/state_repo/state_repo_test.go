//go:build integration

package state_repo_test

import (
	"context"
	"testing"

	"oriva/backend-go/repositories/postgres/state_repo"
	"oriva/backend-go/tests/repositories/postgres/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGraph(t *testing.T) {
	pool := testutil.TestPool(t)
	states, transitions, err := state_repo.LoadGraph(context.Background(), pool)
	require.NoError(t, err)
	assert.Len(t, states, 12)
	assert.Len(t, transitions, 21)

	terminal := map[string]bool{}
	for _, s := range states {
		terminal[s.Name] = s.IsTerminal
	}
	assert.True(t, terminal["scored"])
	assert.False(t, terminal["ready"])
}

package sessions

import (
	"context"
	"testing"

	"oriva/backend-go/db/postgres"
	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/statemachine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func machine(t *testing.T) *statemachine.Machine {
	t.Helper()
	m, err := statemachine.New(
		[]statemachine.State{
			{Name: "scheduled", Label: "Scheduled"},
			{Name: "invited", Label: "Invited"},
			{Name: "ready", Label: "Ready"},
			{Name: "scored", Label: "Scored", IsTerminal: true},
		},
		[]statemachine.Transition{
			{From: "scheduled", To: "invited"},
			{From: "invited", To: "ready"},
		},
	)
	require.NoError(t, err)
	return m
}

type fakeRepo struct {
	current     string
	currentErr  error
	applyErr    error
	applied     bool
	appliedArgs [2]string
}

func (f *fakeRepo) CurrentState(context.Context, string, string) (string, error) {
	return f.current, f.currentErr
}
func (f *fakeRepo) ApplyTransition(_ context.Context, _, _, from, to, _, _ string) error {
	f.applied = true
	f.appliedArgs = [2]string{from, to}
	return f.applyErr
}
func (f *fakeRepo) Get(context.Context, string, string) (*interview.Detail, error) {
	return &interview.Detail{ID: "s1", State: f.appliedArgs[1]}, nil
}

func kind(t *testing.T, err error) apxerrors.Kind {
	t.Helper()
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	return ae.Kind
}

func TestAdvance_Success(t *testing.T) {
	repo := &fakeRepo{current: "scheduled"}
	d, err := NewService(repo, machine(t)).Advance(context.Background(), "o1", "s1", "invited", "email sent", "u1")
	require.NoError(t, err)
	assert.Equal(t, "invited", d.State)
	assert.True(t, repo.applied)
	assert.Equal(t, [2]string{"scheduled", "invited"}, repo.appliedArgs)
}

func TestAdvance_IllegalTransition(t *testing.T) {
	repo := &fakeRepo{current: "ready"}
	_, err := NewService(repo, machine(t)).Advance(context.Background(), "o1", "s1", "scored", "", "u1")
	assert.Equal(t, apxerrors.Conflict, kind(t, err))
	assert.False(t, repo.applied)
}

func TestAdvance_UnknownTarget(t *testing.T) {
	repo := &fakeRepo{current: "scheduled"}
	_, err := NewService(repo, machine(t)).Advance(context.Background(), "o1", "s1", "bogus", "", "u1")
	assert.Equal(t, apxerrors.Invalid, kind(t, err))
	assert.False(t, repo.applied)
}

func TestAdvance_NotFound(t *testing.T) {
	repo := &fakeRepo{currentErr: postgres.ErrNotFound}
	_, err := NewService(repo, machine(t)).Advance(context.Background(), "o1", "s1", "invited", "", "u1")
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}

func TestAdvance_CASConflict(t *testing.T) {
	repo := &fakeRepo{current: "scheduled", applyErr: postgres.ErrConflict}
	_, err := NewService(repo, machine(t)).Advance(context.Background(), "o1", "s1", "invited", "", "u1")
	assert.Equal(t, apxerrors.Conflict, kind(t, err))
}

func TestGraph(t *testing.T) {
	g := NewService(&fakeRepo{}, machine(t)).Graph()
	assert.Len(t, g.States, 4)
	assert.Len(t, g.Transitions, 2)
}

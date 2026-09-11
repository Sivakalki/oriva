package sessions_test

import (
	"context"
	"testing"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/services/sessions"
	"oriva/backend-go/statemachine"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newSvc(t *testing.T, repo *fakeRepo) *sessions.Service {
	t.Helper()
	return sessions.NewService(repo, machine(t), zap.NewNop())
}

func machine(t *testing.T) *statemachine.Machine {
	t.Helper()
	m, err := statemachine.New(
		[]statemachine.State{
			{Name: "scheduled", Label: "Scheduled"},
			{Name: "invited", Label: "Invited"},
			{Name: "ready", Label: "Ready"},
			{Name: "scoring", Label: "Scoring"},
			{Name: "scored", Label: "Scored", IsTerminal: true},
		},
		[]statemachine.Transition{
			{From: "scheduled", To: "invited"},
			{From: "invited", To: "ready"},
			{From: "ready", To: "scoring"},
			{From: "scoring", To: "scored"},
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
	d, err := newSvc(t, repo).Advance(context.Background(), "o1", "s1", "invited", "email sent", "u1")
	require.NoError(t, err)
	assert.Equal(t, "invited", d.State)
	assert.True(t, repo.applied)
	assert.Equal(t, [2]string{"scheduled", "invited"}, repo.appliedArgs)
}

func TestAdvance_IllegalTransition(t *testing.T) {
	repo := &fakeRepo{current: "ready"}
	_, err := newSvc(t, repo).Advance(context.Background(), "o1", "s1", "scored", "", "u1")
	assert.Equal(t, apxerrors.Conflict, kind(t, err))
	assert.False(t, repo.applied)
}

func TestAdvance_UnknownTarget(t *testing.T) {
	repo := &fakeRepo{current: "scheduled"}
	_, err := newSvc(t, repo).Advance(context.Background(), "o1", "s1", "bogus", "", "u1")
	assert.Equal(t, apxerrors.Invalid, kind(t, err))
	assert.False(t, repo.applied)
}

func TestAdvance_NotFound(t *testing.T) {
	repo := &fakeRepo{currentErr: postgres.ErrNotFound}
	_, err := newSvc(t, repo).Advance(context.Background(), "o1", "s1", "invited", "", "u1")
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}

func TestAdvance_CASConflict(t *testing.T) {
	repo := &fakeRepo{current: "scheduled", applyErr: postgres.ErrConflict}
	_, err := newSvc(t, repo).Advance(context.Background(), "o1", "s1", "invited", "", "u1")
	assert.Equal(t, apxerrors.Conflict, kind(t, err))
}

func TestGraph(t *testing.T) {
	g := newSvc(t, &fakeRepo{}).Graph()
	assert.Len(t, g.States, 5)
	assert.Len(t, g.Transitions, 4)
}

type fakeScorer struct {
	called             chan struct{}
	gotOrg, gotSession string
}

func newFakeScorer() *fakeScorer { return &fakeScorer{called: make(chan struct{}, 1)} }

func (f *fakeScorer) ScoreOverall(_ context.Context, orgID, sessionID string) error {
	f.gotOrg, f.gotSession = orgID, sessionID
	f.called <- struct{}{}
	return nil
}

func TestAdvance_TriggersScoringOnEnteringScoringState(t *testing.T) {
	repo := &fakeRepo{current: "ready"}
	svc := newSvc(t, repo)
	scorer := newFakeScorer()
	svc.SetScorer(scorer)

	_, err := svc.Advance(context.Background(), "o1", "s1", "scoring", "", "ai-service")
	require.NoError(t, err)

	select {
	case <-scorer.called:
		assert.Equal(t, "o1", scorer.gotOrg)
		assert.Equal(t, "s1", scorer.gotSession)
	case <-time.After(time.Second):
		t.Fatal("ScoreOverall was not called within 1s of entering the scoring state")
	}
}

func TestAdvance_NoScorerConfigured_DoesNotPanic(t *testing.T) {
	repo := &fakeRepo{current: "ready"}
	svc := newSvc(t, repo) // SetScorer never called
	_, err := svc.Advance(context.Background(), "o1", "s1", "scoring", "", "ai-service")
	require.NoError(t, err)
}

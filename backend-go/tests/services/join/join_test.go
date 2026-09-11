package join_test

import (
	"context"
	"testing"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/interview_repo"
	"oriva/backend-go/services/join"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeRepo struct {
	info *interview_repo.JoinInfo
	err  error
}

func (f fakeRepo) JoinByToken(context.Context, string) (*interview_repo.JoinInfo, error) {
	return f.info, f.err
}

func (f fakeRepo) OrgOf(context.Context, string) (string, error) {
	return "org-1", nil
}

type fakeAdvancer struct {
	calls []string
	err   error
}

func (f *fakeAdvancer) Advance(_ context.Context, _, _, toState, _, _ string) (*interview.Detail, error) {
	f.calls = append(f.calls, toState)
	return &interview.Detail{State: toState}, f.err
}

func svcAt(now time.Time, info *interview_repo.JoinInfo) *join.Service {
	return join.NewServiceWithClock(fakeRepo{info: info}, &fakeAdvancer{}, "ws://ai.test/ws", zap.NewNop(), func() time.Time { return now })
}

func TestStatus_Phases(t *testing.T) {
	sched := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	base := &interview_repo.JoinInfo{SessionID: "sess-1", JobTitle: "Role", ScheduledAt: sched}

	cases := []struct {
		name      string
		now       time.Time
		terminal  bool
		wantPhase string
		wantLate  int
	}{
		{"before", sched.Add(-time.Minute), false, join.PhaseBefore, 0},
		{"open at start", sched, false, join.PhaseOpen, 0},
		{"open within grace", sched.Add(10 * time.Minute), false, join.PhaseOpen, 0},
		{"late past grace", sched.Add(20 * time.Minute), false, join.PhaseLate, 1200},
		{"closed when terminal", sched.Add(20 * time.Minute), true, join.PhaseClosed, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := *base
			info.IsTerminal = c.terminal
			st, err := svcAt(c.now, &info).Status(context.Background(), "tok")
			require.NoError(t, err)
			assert.Equal(t, c.wantPhase, st.Phase)
			assert.Equal(t, c.wantLate, st.LateBySeconds)
			assert.Equal(t, "Role", st.JobTitle)
			assert.Equal(t, "sess-1", st.SessionID)
			assert.Equal(t, "ws://ai.test/ws", st.AIWsURL)
		})
	}
}

func TestStatus_UnknownToken(t *testing.T) {
	s := join.NewService(fakeRepo{err: postgres.ErrNotFound}, &fakeAdvancer{}, "ws://ai.test/ws", zap.NewNop())
	_, err := s.Status(context.Background(), "nope")
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	assert.Equal(t, apxerrors.NotFound, ae.Kind)
}

func TestStart_WalksPreCallChain(t *testing.T) {
	info := &interview_repo.JoinInfo{SessionID: "sess-1", State: "scheduled"}
	adv := &fakeAdvancer{}
	s := join.NewServiceWithClock(fakeRepo{info: info}, adv, "ws://ai.test/ws", zap.NewNop(), time.Now)
	err := s.Start(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, []string{"invited", "ready", "dispatched", "in_progress"}, adv.calls)
}

func TestStart_PartwayThroughChain(t *testing.T) {
	info := &interview_repo.JoinInfo{SessionID: "sess-1", State: "dispatched"}
	adv := &fakeAdvancer{}
	s := join.NewServiceWithClock(fakeRepo{info: info}, adv, "ws://ai.test/ws", zap.NewNop(), time.Now)
	err := s.Start(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, []string{"in_progress"}, adv.calls)
}

func TestStart_TerminalIsNoop(t *testing.T) {
	info := &interview_repo.JoinInfo{SessionID: "sess-1", State: "completed", IsTerminal: true}
	adv := &fakeAdvancer{}
	s := join.NewServiceWithClock(fakeRepo{info: info}, adv, "ws://ai.test/ws", zap.NewNop(), time.Now)
	err := s.Start(context.Background(), "tok")
	require.NoError(t, err)
	assert.Empty(t, adv.calls)
}

func TestStart_NotInPreCallChainIsNoop(t *testing.T) {
	info := &interview_repo.JoinInfo{SessionID: "sess-1", State: "scoring"}
	adv := &fakeAdvancer{}
	s := join.NewServiceWithClock(fakeRepo{info: info}, adv, "ws://ai.test/ws", zap.NewNop(), time.Now)
	err := s.Start(context.Background(), "tok")
	require.NoError(t, err)
	assert.Empty(t, adv.calls)
}

func TestStart_UnknownToken(t *testing.T) {
	s := join.NewService(fakeRepo{err: postgres.ErrNotFound}, &fakeAdvancer{}, "ws://ai.test/ws", zap.NewNop())
	err := s.Start(context.Background(), "nope")
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	assert.Equal(t, apxerrors.NotFound, ae.Kind)
}

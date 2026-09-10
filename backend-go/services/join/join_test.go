package join

import (
	"context"
	"testing"
	"time"

	"oriva/backend-go/db/postgres"
	apxerrors "oriva/backend-go/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	info *postgres.JoinInfo
	err  error
}

func (f fakeRepo) JoinByToken(context.Context, string) (*postgres.JoinInfo, error) {
	return f.info, f.err
}

func svcAt(now time.Time, info *postgres.JoinInfo) *Service {
	return &Service{repo: fakeRepo{info: info}, now: func() time.Time { return now }}
}

func TestStatus_Phases(t *testing.T) {
	sched := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	base := &postgres.JoinInfo{JobTitle: "Role", ScheduledAt: sched}

	cases := []struct {
		name      string
		now       time.Time
		terminal  bool
		wantPhase string
		wantLate  int
	}{
		{"before", sched.Add(-time.Minute), false, PhaseBefore, 0},
		{"open at start", sched, false, PhaseOpen, 0},
		{"open within grace", sched.Add(10 * time.Minute), false, PhaseOpen, 0},
		{"late past grace", sched.Add(20 * time.Minute), false, PhaseLate, 1200},
		{"closed when terminal", sched.Add(20 * time.Minute), true, PhaseClosed, 0},
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
		})
	}
}

func TestStatus_UnknownToken(t *testing.T) {
	s := &Service{repo: fakeRepo{err: postgres.ErrNotFound}, now: time.Now}
	_, err := s.Status(context.Background(), "nope")
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	assert.Equal(t, apxerrors.NotFound, ae.Kind)
}

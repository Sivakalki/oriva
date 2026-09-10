package interviews

import (
	"context"
	"testing"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	scheduled bool
	getCalls  int
}

func (f *fakeRepo) Schedule(context.Context, string, string, string, time.Time) (string, error) {
	f.scheduled = true
	return "s1", nil
}
func (f *fakeRepo) Get(context.Context, string, string) (*interview.Detail, error) {
	f.getCalls++
	return &interview.Detail{ID: "s1", State: "scheduled", StateLabel: "Scheduled"}, nil
}
func (f *fakeRepo) List(context.Context, string, interview.Filter) ([]interview.Detail, error) {
	return nil, nil
}

type checker struct{ ok bool }

func (c checker) ExistsInOrg(context.Context, string, string) (bool, error) { return c.ok, nil }

func kind(t *testing.T, err error) apxerrors.Kind {
	t.Helper()
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	return ae.Kind
}

func future() string { return time.Now().Add(24 * time.Hour).Format(time.RFC3339) }

func TestSchedule_PastTime(t *testing.T) {
	svc := NewService(&fakeRepo{}, checker{true}, checker{true})
	_, err := svc.Schedule(context.Background(), "o1", ScheduleInput{
		JobID: "j1", CandidateID: "c1", ScheduledAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	assert.Equal(t, apxerrors.Invalid, kind(t, err))
}

func TestSchedule_BadTimeFormat(t *testing.T) {
	svc := NewService(&fakeRepo{}, checker{true}, checker{true})
	_, err := svc.Schedule(context.Background(), "o1", ScheduleInput{JobID: "j1", CandidateID: "c1", ScheduledAt: "soon"})
	assert.Equal(t, apxerrors.Invalid, kind(t, err))
}

func TestSchedule_UnknownJob(t *testing.T) {
	svc := NewService(&fakeRepo{}, checker{false}, checker{true})
	_, err := svc.Schedule(context.Background(), "o1", ScheduleInput{JobID: "j1", CandidateID: "c1", ScheduledAt: future()})
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}

func TestSchedule_UnknownCandidate(t *testing.T) {
	svc := NewService(&fakeRepo{}, checker{true}, checker{false})
	_, err := svc.Schedule(context.Background(), "o1", ScheduleInput{JobID: "j1", CandidateID: "c1", ScheduledAt: future()})
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}

func TestSchedule_Happy(t *testing.T) {
	f := &fakeRepo{}
	svc := NewService(f, checker{true}, checker{true})
	d, err := svc.Schedule(context.Background(), "o1", ScheduleInput{JobID: "j1", CandidateID: "c1", ScheduledAt: future()})
	require.NoError(t, err)
	assert.True(t, f.scheduled)
	assert.Equal(t, 1, f.getCalls)
	assert.Equal(t, "Scheduled", d.StateLabel)
}

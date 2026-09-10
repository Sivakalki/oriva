package interviews

import (
	"context"
	"testing"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/candidate"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeRepo struct {
	scheduled bool
	gotToken  string
	getCalls  int
}

func (f *fakeRepo) Schedule(_ context.Context, _, _, _, token string, _ time.Time) (string, error) {
	f.scheduled = true
	f.gotToken = token
	return "s1", nil
}
func (f *fakeRepo) Get(context.Context, string, string) (*interview.Detail, error) {
	f.getCalls++
	return &interview.Detail{
		ID: "s1", State: "scheduled", StateLabel: "Scheduled",
		JoinToken: f.gotToken, Job: interview.JobRef{Title: "Backend Eng"},
	}, nil
}
func (f *fakeRepo) List(context.Context, string, interview.Filter) ([]interview.Detail, error) {
	return nil, nil
}

type jobCheck struct{ ok bool }

func (c jobCheck) ExistsInOrg(context.Context, string, string) (bool, error) { return c.ok, nil }

type candRepo struct {
	ok    bool
	email string
}

func (c candRepo) ExistsInOrg(context.Context, string, string) (bool, error) { return c.ok, nil }
func (c candRepo) Get(context.Context, string, string) (*candidate.Candidate, error) {
	return &candidate.Candidate{Email: c.email, Name: "Alice"}, nil
}

type spySender struct {
	sent *notify.Email
	err  error
}

func (s *spySender) Send(_ context.Context, e notify.Email) error {
	s.sent = &e
	return s.err
}

func newSvc(repo interviewRepo, job jobCheck, cand candRepo, sender notify.Sender) *Service {
	return NewService(repo, job, cand, sender, "http://fe.test", zap.NewNop())
}

func kind(t *testing.T, err error) apxerrors.Kind {
	t.Helper()
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	return ae.Kind
}

func future() string { return time.Now().Add(24 * time.Hour).Format(time.RFC3339) }

func input() ScheduleInput {
	return ScheduleInput{JobID: "j1", CandidateID: "c1", ScheduledAt: future()}
}

func TestSchedule_PastTime(t *testing.T) {
	svc := newSvc(&fakeRepo{}, jobCheck{true}, candRepo{ok: true}, &spySender{})
	in := input()
	in.ScheduledAt = time.Now().Add(-time.Hour).Format(time.RFC3339)
	_, err := svc.Schedule(context.Background(), "o1", in)
	assert.Equal(t, apxerrors.Invalid, kind(t, err))
}

func TestSchedule_BadTimeFormat(t *testing.T) {
	svc := newSvc(&fakeRepo{}, jobCheck{true}, candRepo{ok: true}, &spySender{})
	in := input()
	in.ScheduledAt = "soon"
	_, err := svc.Schedule(context.Background(), "o1", in)
	assert.Equal(t, apxerrors.Invalid, kind(t, err))
}

func TestSchedule_UnknownJob(t *testing.T) {
	svc := newSvc(&fakeRepo{}, jobCheck{false}, candRepo{ok: true}, &spySender{})
	_, err := svc.Schedule(context.Background(), "o1", input())
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}

func TestSchedule_UnknownCandidate(t *testing.T) {
	svc := newSvc(&fakeRepo{}, jobCheck{true}, candRepo{ok: false}, &spySender{})
	_, err := svc.Schedule(context.Background(), "o1", input())
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}

func TestSchedule_Happy_EmailsInvite(t *testing.T) {
	f := &fakeRepo{}
	sender := &spySender{}
	svc := newSvc(f, jobCheck{true}, candRepo{ok: true, email: "alice@example.com"}, sender)

	d, err := svc.Schedule(context.Background(), "o1", input())
	require.NoError(t, err)
	assert.True(t, f.scheduled)
	assert.NotEmpty(t, f.gotToken)
	wantURL := "http://fe.test/join/" + f.gotToken
	assert.Equal(t, wantURL, d.JoinURL)

	require.NotNil(t, sender.sent)
	assert.Equal(t, "alice@example.com", sender.sent.To)
	assert.Contains(t, sender.sent.Text, wantURL)
	assert.Contains(t, sender.sent.Text, "Backend Eng")
}

func TestSchedule_Happy_SendFailureNotFatal(t *testing.T) {
	sender := &spySender{err: assertAnErr}
	svc := newSvc(&fakeRepo{}, jobCheck{true}, candRepo{ok: true, email: "x@y.com"}, sender)
	d, err := svc.Schedule(context.Background(), "o1", input())
	require.NoError(t, err)
	assert.Equal(t, "s1", d.ID)
}

var assertAnErr = &apxerrors.Error{Kind: apxerrors.Internal, Message: "smtp down"}

package interviews_test

import (
	"context"
	"testing"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/candidate"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/notifications/email_repo"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/score_repo"
	"oriva/backend-go/services/interviews"

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
	sent *email_repo.Email
	err  error
}

func (s *spySender) Send(_ context.Context, e email_repo.Email) error {
	s.sent = &e
	return s.err
}

type fakeScores struct {
	overall    *score_repo.OverallScore
	overallErr error
	turns      []score_repo.TurnScore
	turnsErr   error
}

func (f fakeScores) GetOverallScore(context.Context, string) (*score_repo.OverallScore, error) {
	if f.overallErr != nil {
		return nil, f.overallErr
	}
	return f.overall, nil
}
func (f fakeScores) TurnScores(context.Context, string) ([]score_repo.TurnScore, error) {
	return f.turns, f.turnsErr
}

func newSvc(repo *fakeRepo, job jobCheck, cand candRepo, sender email_repo.Sender) *interviews.Service {
	return newSvcWithScores(repo, job, cand, fakeScores{overallErr: postgres.ErrNotFound}, sender)
}

func newSvcWithScores(
	repo *fakeRepo, job jobCheck, cand candRepo, scores fakeScores, sender email_repo.Sender,
) *interviews.Service {
	return interviews.NewService(repo, job, cand, scores, sender, "http://fe.test", zap.NewNop())
}

func kind(t *testing.T, err error) apxerrors.Kind {
	t.Helper()
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	return ae.Kind
}

func future() string { return time.Now().Add(24 * time.Hour).Format(time.RFC3339) }

func input() interviews.ScheduleInput {
	return interviews.ScheduleInput{JobID: "j1", CandidateID: "c1", ScheduledAt: future()}
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

func TestGet_NoScoreYet_LeavesScoreFieldsNil(t *testing.T) {
	svc := newSvc(&fakeRepo{}, jobCheck{true}, candRepo{}, &spySender{})
	d, err := svc.Get(context.Background(), "o1", "s1")
	require.NoError(t, err)
	assert.Nil(t, d.OverallScore)
	assert.Empty(t, d.TurnScores)
}

func TestGet_AttachesOverallScoreAndTurns(t *testing.T) {
	scoredAt := time.Now()
	scores := fakeScores{
		overall: &score_repo.OverallScore{
			Value: 82, Rationale: "strong overall", Model: "qwen2.5:7b-instruct", CreatedAt: scoredAt,
		},
		turns: []score_repo.TurnScore{
			{TurnIndex: 1, Question: "Q1", Answer: "A1", Value: 80, Rationale: "solid"},
			{TurnIndex: 2, Question: "Q2", Answer: "A2", Value: 84, Rationale: "great detail"},
		},
	}
	svc := newSvcWithScores(&fakeRepo{}, jobCheck{true}, candRepo{}, scores, &spySender{})

	d, err := svc.Get(context.Background(), "o1", "s1")
	require.NoError(t, err)
	require.NotNil(t, d.OverallScore)
	assert.Equal(t, 82.0, d.OverallScore.Value)
	assert.Equal(t, "strong overall", d.OverallScore.Rationale)
	assert.Equal(t, "qwen2.5:7b-instruct", d.OverallScore.Model)
	assert.WithinDuration(t, scoredAt, d.OverallScore.ScoredAt, time.Second)

	require.Len(t, d.TurnScores, 2)
	assert.Equal(t, "Q1", d.TurnScores[0].Question)
	assert.Equal(t, 84.0, d.TurnScores[1].Value)
}

func TestGet_ScoreLoadFailure_StillReturnsInterview(t *testing.T) {
	scores := fakeScores{overallErr: assertAnErr, turnsErr: assertAnErr}
	svc := newSvcWithScores(&fakeRepo{}, jobCheck{true}, candRepo{}, scores, &spySender{})

	d, err := svc.Get(context.Background(), "o1", "s1")
	require.NoError(t, err) // a scoring-load failure is logged, not fatal
	assert.Equal(t, "s1", d.ID)
	assert.Nil(t, d.OverallScore)
}

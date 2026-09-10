package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/candidate"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/models/job"
	"oriva/backend-go/services/candidates"
	"oriva/backend-go/services/interviews"
	"oriva/backend-go/services/jobs"
	"oriva/backend-go/utils/authctx"
	"oriva/backend-go/utils/jwt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func authed(method, target, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	return r.WithContext(authctx.WithClaims(r.Context(), &jwt.Claims{OrgID: "o1", Role: jwt.RoleScheduler}))
}

func errKind(t *testing.T, err error) apxerrors.Kind {
	t.Helper()
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	return ae.Kind
}

// --- orgID helper ---

func TestOrgID_NoClaims(t *testing.T) {
	_, err := orgID(httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, apxerrors.Unauthorized, errKind(t, err))
}

// --- jobs ---

type fakeJobs struct{ err error }

func (f fakeJobs) Create(context.Context, string, jobs.CreateInput) (*job.Job, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &job.Job{ID: "j1", Title: "T"}, nil
}
func (f fakeJobs) List(context.Context, string) ([]job.Job, error) { return nil, nil }
func (f fakeJobs) Get(context.Context, string, string) (*job.Job, error) {
	return &job.Job{ID: "j1"}, nil
}
func (f fakeJobs) Update(context.Context, string, string, jobs.UpdateInput) (*job.Job, error) {
	return &job.Job{ID: "j1"}, nil
}

func TestJobsCreate(t *testing.T) {
	h := NewJobsHandler(fakeJobs{})
	body, status, err := h.Create(httptest.NewRecorder(), authed(http.MethodPost, "/jobs", `{"title":"T"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "j1", body.(*job.Job).ID)

	_, _, err = h.Create(httptest.NewRecorder(), authed(http.MethodPost, "/jobs", `{`))
	assert.Equal(t, apxerrors.Invalid, errKind(t, err))
}

func TestJobsList_Envelope(t *testing.T) {
	body, status, err := NewJobsHandler(fakeJobs{}).List(httptest.NewRecorder(), authed(http.MethodGet, "/jobs", ""))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, []job.Job{}, body.(map[string]any)["data"])
}

// --- candidates ---

type fakeCands struct{ err error }

func (f fakeCands) Create(context.Context, string, candidates.CreateInput) (*candidate.Candidate, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &candidate.Candidate{ID: "c1"}, nil
}
func (f fakeCands) List(context.Context, string) ([]candidate.Candidate, error) { return nil, nil }
func (f fakeCands) Get(context.Context, string, string) (*candidate.Candidate, error) {
	return &candidate.Candidate{ID: "c1"}, nil
}
func (f fakeCands) Update(context.Context, string, string, candidates.UpdateInput) (*candidate.Candidate, error) {
	return &candidate.Candidate{ID: "c1"}, nil
}

func TestCandidatesCreate_Conflict(t *testing.T) {
	h := NewCandidatesHandler(fakeCands{err: apxerrors.E(apxerrors.Conflict, "dup")})
	_, _, err := h.Create(httptest.NewRecorder(), authed(http.MethodPost, "/candidates", `{"email":"a@b.com","name":"A"}`))
	assert.Equal(t, apxerrors.Conflict, errKind(t, err))
}

// --- interviews ---

type fakeInterviews struct{ err error }

func (f fakeInterviews) Schedule(context.Context, string, interviews.ScheduleInput) (*interview.Detail, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &interview.Detail{ID: "s1", State: "scheduled"}, nil
}
func (f fakeInterviews) Get(context.Context, string, string) (*interview.Detail, error) {
	return &interview.Detail{ID: "s1"}, nil
}
func (f fakeInterviews) List(context.Context, string, interview.Filter) ([]interview.Detail, error) {
	return nil, nil
}

func TestInterviewsSchedule(t *testing.T) {
	h := NewInterviewsHandler(fakeInterviews{})
	body, status, err := h.Schedule(httptest.NewRecorder(),
		authed(http.MethodPost, "/interviews", `{"job_id":"j1","candidate_id":"c1","scheduled_at":"2099-01-01T00:00:00Z"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "s1", body.(*interview.Detail).ID)
}

func TestInterviewsSchedule_NotFound(t *testing.T) {
	h := NewInterviewsHandler(fakeInterviews{err: apxerrors.E(apxerrors.NotFound, "job not found")})
	_, _, err := h.Schedule(httptest.NewRecorder(),
		authed(http.MethodPost, "/interviews", `{"job_id":"x","candidate_id":"c1","scheduled_at":"2099-01-01T00:00:00Z"}`))
	assert.Equal(t, apxerrors.NotFound, errKind(t, err))
}

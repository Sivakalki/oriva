package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/services/interviews"
	"oriva/backend-go/utils/authctx"

	"github.com/go-chi/chi/v5"
)

type interviewsService interface {
	Schedule(ctx context.Context, orgID string, in interviews.ScheduleInput) (*interview.Detail, error)
	Get(ctx context.Context, orgID, id string) (*interview.Detail, error)
	List(ctx context.Context, orgID string, f interview.Filter) ([]interview.Detail, error)
}

type sessionsService interface {
	Advance(ctx context.Context, orgID, sessionID, toState, reason, actor string) (*interview.Detail, error)
}

// Interviews is the interviews HTTP handler.
type Interviews struct {
	svc      interviewsService
	sessions sessionsService
}

// NewInterviewsHandler constructs an Interviews handler.
func NewInterviewsHandler(svc interviewsService, sessions sessionsService) *Interviews {
	return &Interviews{svc: svc, sessions: sessions}
}

type scheduleBody struct {
	JobID       string `json:"job_id"`
	CandidateID string `json:"candidate_id"`
	ScheduledAt string `json:"scheduled_at"`
}

// Schedule handles POST /interviews.
func (h *Interviews) Schedule(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := OrgID(r)
	if err != nil {
		return nil, 0, err
	}
	var b scheduleBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, 0, apxerrors.InvalidBodyErr(err)
	}
	d, err := h.svc.Schedule(r.Context(), org, interviews.ScheduleInput{
		JobID: b.JobID, CandidateID: b.CandidateID, ScheduledAt: b.ScheduledAt,
	})
	if err != nil {
		return nil, 0, err
	}
	return d, http.StatusCreated, nil
}

// List handles GET /interviews with optional ?state= ?job_id= ?candidate_id=.
func (h *Interviews) List(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := OrgID(r)
	if err != nil {
		return nil, 0, err
	}
	q := r.URL.Query()
	f := interview.Filter{
		State:       q.Get("state"),
		JobID:       q.Get("job_id"),
		CandidateID: q.Get("candidate_id"),
	}
	ds, err := h.svc.List(r.Context(), org, f)
	if err != nil {
		return nil, 0, err
	}
	return listEnvelope(ds), http.StatusOK, nil
}

type advanceBody struct {
	ToState string `json:"to_state"`
	Reason  string `json:"reason"`
}

// Advance handles POST /interviews/{id}/advance.
func (h *Interviews) Advance(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := OrgID(r)
	if err != nil {
		return nil, 0, err
	}
	var b advanceBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, 0, apxerrors.InvalidBodyErr(err)
	}

	actor := ""
	if claims, ok := authctx.FromContext(r.Context()); ok {
		actor = claims.Subject
	}

	d, err := h.sessions.Advance(r.Context(), org, chi.URLParam(r, "id"), b.ToState, b.Reason, actor)
	if err != nil {
		return nil, 0, err
	}
	return d, http.StatusOK, nil
}

// Get handles GET /interviews/{id}.
func (h *Interviews) Get(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := OrgID(r)
	if err != nil {
		return nil, 0, err
	}
	d, err := h.svc.Get(r.Context(), org, chi.URLParam(r, "id"))
	if err != nil {
		return nil, 0, err
	}
	return d, http.StatusOK, nil
}

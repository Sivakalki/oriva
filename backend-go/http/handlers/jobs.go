package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/job"
	"oriva/backend-go/services/jobs"

	"github.com/go-chi/chi/v5"
)

type jobsService interface {
	Create(ctx context.Context, orgID string, in jobs.CreateInput) (*job.Job, error)
	List(ctx context.Context, orgID string) ([]job.Job, error)
	Get(ctx context.Context, orgID, id string) (*job.Job, error)
	Update(ctx context.Context, orgID, id string, in jobs.UpdateInput) (*job.Job, error)
}

// Jobs is the jobs HTTP handler.
type Jobs struct{ svc jobsService }

// NewJobsHandler constructs a Jobs handler.
func NewJobsHandler(svc jobsService) *Jobs { return &Jobs{svc: svc} }

type jobCreateBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type jobUpdateBody struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

// Create handles POST /jobs.
func (h *Jobs) Create(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := OrgID(r)
	if err != nil {
		return nil, 0, err
	}
	var b jobCreateBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, 0, apxerrors.InvalidBodyErr(err)
	}
	j, err := h.svc.Create(r.Context(), org, jobs.CreateInput{Title: b.Title, Description: b.Description})
	if err != nil {
		return nil, 0, err
	}
	return j, http.StatusCreated, nil
}

// List handles GET /jobs.
func (h *Jobs) List(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := OrgID(r)
	if err != nil {
		return nil, 0, err
	}
	js, err := h.svc.List(r.Context(), org)
	if err != nil {
		return nil, 0, err
	}
	return listEnvelope(js), http.StatusOK, nil
}

// Get handles GET /jobs/{id}.
func (h *Jobs) Get(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := OrgID(r)
	if err != nil {
		return nil, 0, err
	}
	j, err := h.svc.Get(r.Context(), org, chi.URLParam(r, "id"))
	if err != nil {
		return nil, 0, err
	}
	return j, http.StatusOK, nil
}

// Update handles PATCH /jobs/{id}.
func (h *Jobs) Update(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := OrgID(r)
	if err != nil {
		return nil, 0, err
	}
	var b jobUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, 0, apxerrors.InvalidBodyErr(err)
	}
	j, err := h.svc.Update(r.Context(), org, chi.URLParam(r, "id"), jobs.UpdateInput{Title: b.Title, Description: b.Description})
	if err != nil {
		return nil, 0, err
	}
	return j, http.StatusOK, nil
}

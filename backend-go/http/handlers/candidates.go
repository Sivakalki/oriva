package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/candidate"
	"oriva/backend-go/services/candidates"

	"github.com/go-chi/chi/v5"
)

type candidatesService interface {
	Create(ctx context.Context, orgID string, in candidates.CreateInput) (*candidate.Candidate, error)
	List(ctx context.Context, orgID string) ([]candidate.Candidate, error)
	Get(ctx context.Context, orgID, id string) (*candidate.Candidate, error)
	Update(ctx context.Context, orgID, id string, in candidates.UpdateInput) (*candidate.Candidate, error)
}

// Candidates is the candidates HTTP handler.
type Candidates struct{ svc candidatesService }

// NewCandidatesHandler constructs a Candidates handler.
func NewCandidatesHandler(svc candidatesService) *Candidates { return &Candidates{svc: svc} }

type candCreateBody struct {
	Email      string `json:"email"`
	Name       string `json:"name"`
	ResumeText string `json:"resume_text"`
}

type candUpdateBody struct {
	Name       *string `json:"name"`
	ResumeText *string `json:"resume_text"`
}

// Create handles POST /candidates.
func (h *Candidates) Create(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := orgID(r)
	if err != nil {
		return nil, 0, err
	}
	var b candCreateBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, 0, apxerrors.InvalidBodyErr(err)
	}
	c, err := h.svc.Create(r.Context(), org, candidates.CreateInput{
		Email: b.Email, Name: b.Name, ResumeText: b.ResumeText,
	})
	if err != nil {
		return nil, 0, err
	}
	return c, http.StatusCreated, nil
}

// List handles GET /candidates.
func (h *Candidates) List(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := orgID(r)
	if err != nil {
		return nil, 0, err
	}
	cs, err := h.svc.List(r.Context(), org)
	if err != nil {
		return nil, 0, err
	}
	return listEnvelope(cs), http.StatusOK, nil
}

// Get handles GET /candidates/{id}.
func (h *Candidates) Get(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := orgID(r)
	if err != nil {
		return nil, 0, err
	}
	c, err := h.svc.Get(r.Context(), org, chi.URLParam(r, "id"))
	if err != nil {
		return nil, 0, err
	}
	return c, http.StatusOK, nil
}

// Update handles PATCH /candidates/{id}.
func (h *Candidates) Update(w http.ResponseWriter, r *http.Request) (any, int, error) {
	org, err := orgID(r)
	if err != nil {
		return nil, 0, err
	}
	var b candUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, 0, apxerrors.InvalidBodyErr(err)
	}
	c, err := h.svc.Update(r.Context(), org, chi.URLParam(r, "id"), candidates.UpdateInput{
		Name: b.Name, ResumeText: b.ResumeText,
	})
	if err != nil {
		return nil, 0, err
	}
	return c, http.StatusOK, nil
}

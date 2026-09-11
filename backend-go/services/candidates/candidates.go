// Package candidates holds candidate-profile business logic.
package candidates

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/candidate"
	"oriva/backend-go/repositories/postgres"
)

const (
	maxName   = 200
	maxResume = 100000
)

type repo interface {
	Create(ctx context.Context, orgID, email, name, resumeText string) (*candidate.Candidate, error)
	ListByOrg(ctx context.Context, orgID string) ([]candidate.Candidate, error)
	Get(ctx context.Context, orgID, id string) (*candidate.Candidate, error)
	Update(ctx context.Context, orgID, id string, name, resumeText *string) (*candidate.Candidate, error)
}

// Service is the candidates service.
type Service struct{ repo repo }

// NewService constructs a candidates Service.
func NewService(r repo) *Service { return &Service{repo: r} }

// CreateInput is the create payload.
type CreateInput struct {
	Email      string
	Name       string
	ResumeText string
}

// Create validates and inserts a candidate.
func (s *Service) Create(ctx context.Context, orgID string, in CreateInput) (*candidate.Candidate, error) {
	in.Email = strings.TrimSpace(in.Email)
	in.Name = strings.TrimSpace(in.Name)

	ve := apxerrors.ValidationErrs()
	addr, err := mail.ParseAddress(in.Email)
	if in.Email == "" || err != nil {
		ve.Add("email", "invalid email")
	}
	if in.Name == "" || len(in.Name) > maxName {
		ve.Add("name", "invalid")
	}
	if len(in.ResumeText) > maxResume {
		ve.Add("resume_text", "too long")
	}
	if verr := ve.Err(); verr != nil {
		return nil, apxerrors.ValidationFailedErr(verr)
	}

	c, err := s.repo.Create(ctx, orgID, addr.Address, in.Name, in.ResumeText)
	if errors.Is(err, postgres.ErrConflict) {
		return nil, apxerrors.E(apxerrors.Conflict, "a candidate with that email already exists")
	}
	return c, err
}

// List returns the org's candidates.
func (s *Service) List(ctx context.Context, orgID string) ([]candidate.Candidate, error) {
	return s.repo.ListByOrg(ctx, orgID)
}

// Get returns one candidate or a NotFound error.
func (s *Service) Get(ctx context.Context, orgID, id string) (*candidate.Candidate, error) {
	c, err := s.repo.Get(ctx, orgID, id)
	return c, mapErr(err)
}

// UpdateInput carries the optional fields to change. Email is immutable.
type UpdateInput struct {
	Name       *string
	ResumeText *string
}

// Update applies a partial change; at least one field must be present.
func (s *Service) Update(ctx context.Context, orgID, id string, in UpdateInput) (*candidate.Candidate, error) {
	ve := apxerrors.ValidationErrs()
	if in.Name == nil && in.ResumeText == nil {
		ve.Add("body", "at least one field required")
	}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		in.Name = &n
		if n == "" || len(n) > maxName {
			ve.Add("name", "invalid")
		}
	}
	if in.ResumeText != nil && len(*in.ResumeText) > maxResume {
		ve.Add("resume_text", "too long")
	}
	if err := ve.Err(); err != nil {
		return nil, apxerrors.ValidationFailedErr(err)
	}
	c, err := s.repo.Update(ctx, orgID, id, in.Name, in.ResumeText)
	return c, mapErr(err)
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, postgres.ErrNotFound) {
		return apxerrors.E(apxerrors.NotFound, "candidate not found")
	}
	return err
}

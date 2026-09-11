// Package jobs holds job-management business logic.
package jobs

import (
	"context"
	"errors"
	"strings"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/job"
	"oriva/backend-go/repositories/postgres"
)

const (
	maxTitle = 200
	maxDesc  = 20000
)

// repo is the store contract, defined at the point of consumption.
type repo interface {
	Create(ctx context.Context, orgID, title, description string) (*job.Job, error)
	ListByOrg(ctx context.Context, orgID string) ([]job.Job, error)
	Get(ctx context.Context, orgID, id string) (*job.Job, error)
	Update(ctx context.Context, orgID, id string, title, description *string) (*job.Job, error)
}

// Service is the jobs service.
type Service struct{ repo repo }

// NewService constructs a jobs Service.
func NewService(r repo) *Service { return &Service{repo: r} }

// CreateInput is the create payload.
type CreateInput struct {
	Title       string
	Description string
}

// Create validates and inserts a job.
func (s *Service) Create(ctx context.Context, orgID string, in CreateInput) (*job.Job, error) {
	in.Title = strings.TrimSpace(in.Title)
	ve := apxerrors.ValidationErrs()
	if in.Title == "" {
		ve.Add("title", "cannot be empty")
	}
	if len(in.Title) > maxTitle {
		ve.Add("title", "too long")
	}
	if len(in.Description) > maxDesc {
		ve.Add("description", "too long")
	}
	if err := ve.Err(); err != nil {
		return nil, apxerrors.ValidationFailedErr(err)
	}
	return s.repo.Create(ctx, orgID, in.Title, in.Description)
}

// List returns the org's jobs.
func (s *Service) List(ctx context.Context, orgID string) ([]job.Job, error) {
	return s.repo.ListByOrg(ctx, orgID)
}

// Get returns one job or a NotFound error.
func (s *Service) Get(ctx context.Context, orgID, id string) (*job.Job, error) {
	j, err := s.repo.Get(ctx, orgID, id)
	return j, mapErr(err)
}

// UpdateInput carries the optional fields to change.
type UpdateInput struct {
	Title       *string
	Description *string
}

// Update applies a partial change; at least one field must be present.
func (s *Service) Update(ctx context.Context, orgID, id string, in UpdateInput) (*job.Job, error) {
	ve := apxerrors.ValidationErrs()
	if in.Title == nil && in.Description == nil {
		ve.Add("body", "at least one field required")
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		in.Title = &t
		if t == "" || len(t) > maxTitle {
			ve.Add("title", "invalid")
		}
	}
	if in.Description != nil && len(*in.Description) > maxDesc {
		ve.Add("description", "too long")
	}
	if err := ve.Err(); err != nil {
		return nil, apxerrors.ValidationFailedErr(err)
	}
	j, err := s.repo.Update(ctx, orgID, id, in.Title, in.Description)
	return j, mapErr(err)
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, postgres.ErrNotFound) {
		return apxerrors.E(apxerrors.NotFound, "job not found")
	}
	return err
}

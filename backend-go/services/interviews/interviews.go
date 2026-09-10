// Package interviews holds interview-scheduling business logic.
package interviews

import (
	"context"
	"errors"
	"time"

	"oriva/backend-go/db/postgres"
	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"
)

// interviewRepo, jobChecker and candChecker are the consumed contracts.
type interviewRepo interface {
	Schedule(ctx context.Context, orgID, jobID, candidateID string, scheduledAt time.Time) (string, error)
	Get(ctx context.Context, orgID, id string) (*interview.Detail, error)
	List(ctx context.Context, orgID string, f interview.Filter) ([]interview.Detail, error)
}

type jobChecker interface {
	ExistsInOrg(ctx context.Context, orgID, id string) (bool, error)
}

type candChecker interface {
	ExistsInOrg(ctx context.Context, orgID, id string) (bool, error)
}

// Service is the interviews service.
type Service struct {
	repo  interviewRepo
	jobs  jobChecker
	cands candChecker
	now   func() time.Time
}

// NewService constructs an interviews Service.
func NewService(r interviewRepo, jobs jobChecker, cands candChecker) *Service {
	return &Service{repo: r, jobs: jobs, cands: cands, now: time.Now}
}

// ScheduleInput is the schedule payload. ScheduledAt is an RFC3339 string.
type ScheduleInput struct {
	JobID       string
	CandidateID string
	ScheduledAt string
}

// Schedule validates the request, checks org ownership of the job and candidate,
// and creates a session in the "scheduled" state.
func (s *Service) Schedule(ctx context.Context, orgID string, in ScheduleInput) (*interview.Detail, error) {
	ve := apxerrors.ValidationErrs()
	if in.JobID == "" {
		ve.Add("job_id", "cannot be empty")
	}
	if in.CandidateID == "" {
		ve.Add("candidate_id", "cannot be empty")
	}
	at, err := time.Parse(time.RFC3339, in.ScheduledAt)
	if err != nil {
		ve.Add("scheduled_at", "must be an RFC3339 timestamp")
	} else if !at.After(s.now()) {
		ve.Add("scheduled_at", "must be in the future")
	}
	if verr := ve.Err(); verr != nil {
		return nil, apxerrors.ValidationFailedErr(verr)
	}

	if ok, err := s.jobs.ExistsInOrg(ctx, orgID, in.JobID); err != nil {
		return nil, err
	} else if !ok {
		return nil, apxerrors.E(apxerrors.NotFound, "job not found")
	}
	if ok, err := s.cands.ExistsInOrg(ctx, orgID, in.CandidateID); err != nil {
		return nil, err
	} else if !ok {
		return nil, apxerrors.E(apxerrors.NotFound, "candidate not found")
	}

	id, err := s.repo.Schedule(ctx, orgID, in.JobID, in.CandidateID, at.UTC())
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, orgID, id)
}

// Get returns one interview Detail or a NotFound error.
func (s *Service) Get(ctx context.Context, orgID, id string) (*interview.Detail, error) {
	d, err := s.repo.Get(ctx, orgID, id)
	if errors.Is(err, postgres.ErrNotFound) {
		return nil, apxerrors.E(apxerrors.NotFound, "interview not found")
	}
	return d, err
}

// List returns interview Details for the org with any filters applied.
func (s *Service) List(ctx context.Context, orgID string, f interview.Filter) ([]interview.Detail, error) {
	return s.repo.List(ctx, orgID, f)
}

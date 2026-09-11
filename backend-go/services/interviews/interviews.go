// Package interviews holds interview-scheduling business logic.
package interviews

import (
	"context"
	"errors"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/candidate"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/notifications/email_repo"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/score_repo"
	"oriva/backend-go/utils/helpers"

	"go.uber.org/zap"
)

// DefaultDurationMinutes is applied when the recruiter doesn't set one.
const DefaultDurationMinutes = 30

// MinDurationMinutes / MaxDurationMinutes bound what a recruiter can set.
const (
	MinDurationMinutes = 5
	MaxDurationMinutes = 120
)

// Consumed contracts.
type interviewRepo interface {
	Schedule(
		ctx context.Context, orgID, jobID, candidateID, joinToken string, scheduledAt time.Time, durationMinutes int,
	) (string, error)
	Get(ctx context.Context, orgID, id string) (*interview.Detail, error)
	List(ctx context.Context, orgID string, f interview.Filter) ([]interview.Detail, error)
}

type jobChecker interface {
	ExistsInOrg(ctx context.Context, orgID, id string) (bool, error)
}

type candReader interface {
	ExistsInOrg(ctx context.Context, orgID, id string) (bool, error)
	Get(ctx context.Context, orgID, id string) (*candidate.Candidate, error)
}

// scoreReader is the consumed contract for surfacing scores on GET
// /interviews/{id} (recruiter-only route; see http/server.go).
type scoreReader interface {
	GetOverallScore(ctx context.Context, sessionID string) (*score_repo.OverallScore, error)
	TurnScores(ctx context.Context, sessionID string) ([]score_repo.TurnScore, error)
}

// Service is the interviews service.
type Service struct {
	repo     interviewRepo
	jobs     jobChecker
	cands    candReader
	scores   scoreReader
	notifier email_repo.Sender
	baseURL  string
	logger   *zap.Logger
	now      func() time.Time
}

// NewService constructs an interviews Service.
func NewService(
	r interviewRepo, jobs jobChecker, cands candReader, scores scoreReader,
	notifier email_repo.Sender, baseURL string, logger *zap.Logger,
) *Service {
	return &Service{
		repo: r, jobs: jobs, cands: cands, scores: scores, notifier: notifier,
		baseURL: baseURL, logger: logger, now: time.Now,
	}
}

// ScheduleInput is the schedule payload. ScheduledAt is an RFC3339 string.
// DurationMinutes <= 0 gets DefaultDurationMinutes applied.
type ScheduleInput struct {
	JobID           string
	CandidateID     string
	ScheduledAt     string
	DurationMinutes int
}

// Schedule validates the request, checks org ownership of the job and candidate,
// creates a session in the "scheduled" state, and emails the candidate an invite.
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
	if in.DurationMinutes == 0 {
		in.DurationMinutes = DefaultDurationMinutes
	} else if in.DurationMinutes < MinDurationMinutes || in.DurationMinutes > MaxDurationMinutes {
		ve.Add("duration_minutes", "must be between 5 and 120")
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

	token := helpers.NewJoinToken()
	id, err := s.repo.Schedule(ctx, orgID, in.JobID, in.CandidateID, token, at.UTC(), in.DurationMinutes)
	if err != nil {
		return nil, err
	}

	d, err := s.repo.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	d.JoinURL = s.joinURL(token)

	s.sendInvite(ctx, orgID, in.CandidateID, d, at.UTC())
	return d, nil
}

func (s *Service) sendInvite(ctx context.Context, orgID, candidateID string, d *interview.Detail, at time.Time) {
	c, err := s.cands.Get(ctx, orgID, candidateID)
	if err != nil {
		s.logger.Warn("invite: candidate lookup failed", zap.Error(err))
		return
	}
	email := email_repo.InviteEmail(c.Email, c.Name, d.Job.Title, d.JoinURL, at)
	if err := s.notifier.Send(ctx, email); err != nil {
		s.logger.Warn("invite: send failed", zap.String("to", c.Email), zap.Error(err))
	}
}

func (s *Service) joinURL(token string) string {
	return s.baseURL + "/join/" + token
}

// Get returns one interview Detail or a NotFound error. Populates
// OverallScore/TurnScores when scoring has run (see services/scoring); a
// load failure there is logged, not fatal -- the rest of the interview's
// detail is still useful without its score.
func (s *Service) Get(ctx context.Context, orgID, id string) (*interview.Detail, error) {
	d, err := s.repo.Get(ctx, orgID, id)
	if errors.Is(err, postgres.ErrNotFound) {
		return nil, apxerrors.E(apxerrors.NotFound, "interview not found")
	}
	if err != nil {
		return nil, err
	}
	d.JoinURL = s.joinURL(d.JoinToken)
	s.attachScores(ctx, d)
	return d, nil
}

func (s *Service) attachScores(ctx context.Context, d *interview.Detail) {
	if overall, err := s.scores.GetOverallScore(ctx, d.ID); err == nil {
		d.OverallScore = &interview.ScoreSummary{
			Value: overall.Value, Rationale: overall.Rationale,
			Model: overall.Model, ScoredAt: overall.CreatedAt,
		}
	} else if !errors.Is(err, postgres.ErrNotFound) {
		s.logger.Warn("load overall score failed", zap.String("session_id", d.ID), zap.Error(err))
	}

	turns, err := s.scores.TurnScores(ctx, d.ID)
	if err != nil {
		s.logger.Warn("load turn scores failed", zap.String("session_id", d.ID), zap.Error(err))
		return
	}
	d.TurnScores = make([]interview.TurnScore, len(turns))
	for i, t := range turns {
		d.TurnScores[i] = interview.TurnScore{
			TurnIndex: t.TurnIndex, Question: t.Question, Answer: t.Answer,
			Value: t.Value, Rationale: t.Rationale,
		}
	}
}

// List returns interview Details for the org with any filters and the join URL filled.
func (s *Service) List(ctx context.Context, orgID string, f interview.Filter) ([]interview.Detail, error) {
	ds, err := s.repo.List(ctx, orgID, f)
	if err != nil {
		return nil, err
	}
	for i := range ds {
		ds[i].JoinURL = s.joinURL(ds[i].JoinToken)
	}
	return ds, nil
}

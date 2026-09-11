// Package join resolves a candidate join token into a status the landing page
// can render (has the interview started, is the candidate late, is it over).
package join

import (
	"context"
	"errors"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/interview_repo"
)

// Grace is how long after the scheduled time a candidate is still "open"
// rather than "late".
const Grace = 15 * time.Minute

type repo interface {
	JoinByToken(ctx context.Context, token string) (*interview_repo.JoinInfo, error)
}

// Service resolves join tokens.
type Service struct {
	repo    repo
	aiWsURL string
	now     func() time.Time
}

// NewService constructs a join Service.
func NewService(r repo, aiWsURL string) *Service {
	return NewServiceWithClock(r, aiWsURL, time.Now)
}

// NewServiceWithClock constructs a join Service with an injectable clock
// (tests use this to control "now" without needing package-internal access).
func NewServiceWithClock(r repo, aiWsURL string, now func() time.Time) *Service {
	return &Service{repo: r, aiWsURL: aiWsURL, now: now}
}

// Phase values.
const (
	PhaseBefore = "before"
	PhaseOpen   = "open"
	PhaseLate   = "late"
	PhaseClosed = "closed"
)

// Status is the candidate-facing view.
type Status struct {
	JobTitle      string    `json:"job_title"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	ServerNow     time.Time `json:"server_now"`
	Phase         string    `json:"phase"`
	LateBySeconds int       `json:"late_by_seconds"`
	SessionID     string    `json:"session_id"`
	AIWsURL       string    `json:"ai_ws_url"`
}

// Status resolves the token or returns a NotFound error.
func (s *Service) Status(ctx context.Context, token string) (*Status, error) {
	info, err := s.repo.JoinByToken(ctx, token)
	if errors.Is(err, postgres.ErrNotFound) {
		return nil, apxerrors.E(apxerrors.NotFound, "invalid or expired interview link")
	}
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	st := &Status{
		JobTitle:    info.JobTitle,
		ScheduledAt: info.ScheduledAt.UTC(),
		ServerNow:   now,
		SessionID:   info.SessionID,
		AIWsURL:     s.aiWsURL,
	}

	switch {
	case info.IsTerminal:
		st.Phase = PhaseClosed
	case now.Before(info.ScheduledAt):
		st.Phase = PhaseBefore
	case now.Before(info.ScheduledAt.Add(Grace)):
		st.Phase = PhaseOpen
	default:
		st.Phase = PhaseLate
		st.LateBySeconds = int(now.Sub(info.ScheduledAt).Seconds())
	}
	return st, nil
}

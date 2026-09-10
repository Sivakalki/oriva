// Package join resolves a candidate join token into a status the landing page
// can render (has the interview started, is the candidate late, is it over).
package join

import (
	"context"
	"errors"
	"time"

	"oriva/backend-go/db/postgres"
	apxerrors "oriva/backend-go/errors"
)

// Grace is how long after the scheduled time a candidate is still "open"
// rather than "late".
const Grace = 15 * time.Minute

type repo interface {
	JoinByToken(ctx context.Context, token string) (*postgres.JoinInfo, error)
}

// Service resolves join tokens.
type Service struct {
	repo repo
	now  func() time.Time
}

// NewService constructs a join Service.
func NewService(r repo) *Service { return &Service{repo: r, now: time.Now} }

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

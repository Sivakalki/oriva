// Package join resolves a candidate join token into a status the landing page
// can render (has the interview started, is the candidate late, is it over).
package join

import (
	"context"
	"errors"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/repositories/postgres/interview_repo"

	"go.uber.org/zap"
)

// Grace is how long after the scheduled time a candidate is still "open"
// rather than "late".
const Grace = 15 * time.Minute

// preCallChain is the state-machine path a session must walk before the
// call can be considered live. Start() drives it from wherever the session
// currently sits up to "in_progress" -- nothing else does (recruiters don't
// click through these for the candidate-join flow, and the live LLM is only
// ever told to advance state at the *end* of the call), so without this a
// session sits at "scheduled" for the whole call and a refresh after it
// ends just re-offers "Start Interview" (phase is only closed by a
// terminal state, not by "a call already happened").
var preCallChain = []string{"scheduled", "invited", "ready", "dispatched", "in_progress"}

type repo interface {
	JoinByToken(ctx context.Context, token string) (*interview_repo.JoinInfo, error)
	OrgOf(ctx context.Context, sessionID string) (string, error)
}

// stateAdvancer is the consumed contract for driving the pre-call chain.
type stateAdvancer interface {
	Advance(ctx context.Context, orgID, sessionID, toState, reason, actor string) (*interview.Detail, error)
}

// Service resolves join tokens.
type Service struct {
	repo     repo
	sessions stateAdvancer
	aiWsURL  string
	logger   *zap.Logger
	now      func() time.Time
}

// NewService constructs a join Service.
func NewService(r repo, sessions stateAdvancer, aiWsURL string, logger *zap.Logger) *Service {
	return NewServiceWithClock(r, sessions, aiWsURL, logger, time.Now)
}

// NewServiceWithClock constructs a join Service with an injectable clock
// (tests use this to control "now" without needing package-internal access).
func NewServiceWithClock(
	r repo, sessions stateAdvancer, aiWsURL string, logger *zap.Logger, now func() time.Time,
) *Service {
	return &Service{repo: r, sessions: sessions, aiWsURL: aiWsURL, logger: logger, now: now}
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
	JobTitle        string    `json:"job_title"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	ServerNow       time.Time `json:"server_now"`
	Phase           string    `json:"phase"`
	LateBySeconds   int       `json:"late_by_seconds"`
	SessionID       string    `json:"session_id"`
	AIWsURL         string    `json:"ai_ws_url"`
	DurationMinutes int       `json:"duration_minutes"`
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
		JobTitle:        info.JobTitle,
		ScheduledAt:     info.ScheduledAt.UTC(),
		ServerNow:       now,
		SessionID:       info.SessionID,
		AIWsURL:         s.aiWsURL,
		DurationMinutes: info.DurationMinutes,
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

// Start drives the session from wherever it currently sits up to
// "in_progress", one legal hop at a time. Called once by ai-service right
// before it accepts the candidate's WebSocket connection -- i.e. the call is
// actually starting, not just that someone loaded the landing page (Status
// above is read-only and has no side effects; this does).
//
// Idempotent: a session already at or past "in_progress" (including a
// reconnect, or an already-terminal session) is a silent no-op, not an
// error -- Start is best-effort scaffolding, not something a flaky network
// blip should be able to fail the call over.
func (s *Service) Start(ctx context.Context, token string) error {
	info, err := s.repo.JoinByToken(ctx, token)
	if errors.Is(err, postgres.ErrNotFound) {
		return apxerrors.E(apxerrors.NotFound, "invalid or expired interview link")
	}
	if err != nil {
		return err
	}
	if info.IsTerminal {
		return nil
	}

	idx := -1
	for i, name := range preCallChain {
		if name == info.State {
			idx = i
			break
		}
	}
	if idx < 0 {
		// Not in the pre-call chain at all (e.g. already "completed" or
		// "scoring" from a prior attempt) -- nothing for Start to do.
		return nil
	}

	orgID, err := s.repo.OrgOf(ctx, info.SessionID)
	if err != nil {
		return err
	}

	for i := idx; i < len(preCallChain)-1; i++ {
		if _, err := s.sessions.Advance(
			ctx, orgID, info.SessionID, preCallChain[i+1], "candidate joined the call", "ai-service",
		); err != nil {
			s.logger.Warn("join.Start: advance failed",
				zap.String("session_id", info.SessionID),
				zap.String("to", preCallChain[i+1]), zap.Error(err))
			return err
		}
	}
	return nil
}

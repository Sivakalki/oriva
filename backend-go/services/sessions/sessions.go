// Package sessions is the authority for interview-session state transitions.
package sessions

import (
	"context"
	"errors"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/statemachine"

	"go.uber.org/zap"
)

// interviewStateRepo is the consumed store contract.
type interviewStateRepo interface {
	CurrentState(ctx context.Context, orgID, id string) (string, error)
	ApplyTransition(ctx context.Context, orgID, id, from, to, reason, actor string) error
	Get(ctx context.Context, orgID, id string) (*interview.Detail, error)
}

// scoreTrigger kicks off overall scoring once a session enters the
// "scoring" state. A local, consumer-defined interface (services/scoring
// satisfies it structurally) so this package never imports services/scoring
// -- scoring already depends on sessions (to auto-advance scoring->scored),
// so an import the other way would cycle.
type scoreTrigger interface {
	ScoreOverall(ctx context.Context, orgID, sessionID string) error
}

// Service validates and applies state transitions.
type Service struct {
	repo    interviewStateRepo
	machine *statemachine.Machine
	scorer  scoreTrigger // nil until SetScorer is called; Advance skips the hook then
	logger  *zap.Logger
}

// NewService constructs a sessions Service. Scoring is off until SetScorer
// is called (main.go wires it once the scoring service exists, breaking
// what would otherwise be a construction-order cycle).
func NewService(repo interviewStateRepo, m *statemachine.Machine, logger *zap.Logger) *Service {
	return &Service{repo: repo, machine: m, logger: logger}
}

// SetScorer wires the scoring trigger. Advance calls it (in the background)
// whenever a session transitions into the "scoring" state.
func (s *Service) SetScorer(scorer scoreTrigger) { s.scorer = scorer }

// Graph exposes the loaded state graph for the API.
func (s *Service) Graph() statemachine.Graph { return s.machine.Graph() }

// Advance validates from the session's current state to toState, applies the
// transition with an audit event, and returns the refreshed interview.
func (s *Service) Advance(
	ctx context.Context, orgID, sessionID, toState, reason, actor string,
) (*interview.Detail, error) {
	if toState == "" || !s.machine.Has(toState) {
		return nil, apxerrors.E(apxerrors.Invalid, "unknown target state")
	}

	current, err := s.repo.CurrentState(ctx, orgID, sessionID)
	if errors.Is(err, postgres.ErrNotFound) {
		return nil, apxerrors.E(apxerrors.NotFound, "interview not found")
	}
	if err != nil {
		return nil, err
	}

	if err := s.machine.Validate(current, toState); err != nil {
		switch {
		case errors.Is(err, statemachine.ErrUnknownState):
			return nil, apxerrors.E(apxerrors.Invalid, "unknown session state")
		default: // ErrTerminalState, ErrIllegalTransition
			return nil, apxerrors.E(apxerrors.Conflict,
				"cannot move from "+current+" to "+toState)
		}
	}

	switch err := s.repo.ApplyTransition(ctx, orgID, sessionID, current, toState, reason, actor); {
	case errors.Is(err, postgres.ErrNotFound):
		return nil, apxerrors.E(apxerrors.NotFound, "interview not found")
	case errors.Is(err, postgres.ErrConflict):
		return nil, apxerrors.E(apxerrors.Conflict, "session state changed concurrently; retry")
	case err != nil:
		return nil, err
	}

	// Entering "scoring" kicks off overall scoring in the background -- never
	// blocks the caller (e.g. the LLM's advance_state tool call) on an LLM
	// judge round-trip. services/scoring auto-advances scoring -> scored once
	// it has a result.
	if toState == "scoring" && s.scorer != nil {
		go func() {
			if err := s.scorer.ScoreOverall(context.Background(), orgID, sessionID); err != nil {
				s.logger.Warn("overall scoring failed",
					zap.String("session_id", sessionID), zap.Error(err))
			}
		}()
	}

	return s.repo.Get(ctx, orgID, sessionID)
}

// Package sessions is the authority for interview-session state transitions.
package sessions

import (
	"context"
	"errors"

	"oriva/backend-go/db/postgres"
	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/interview"
	"oriva/backend-go/statemachine"
)

// interviewStateRepo is the consumed store contract.
type interviewStateRepo interface {
	CurrentState(ctx context.Context, orgID, id string) (string, error)
	ApplyTransition(ctx context.Context, orgID, id, from, to, reason, actor string) error
	Get(ctx context.Context, orgID, id string) (*interview.Detail, error)
}

// Service validates and applies state transitions.
type Service struct {
	repo    interviewStateRepo
	machine *statemachine.Machine
}

// NewService constructs a sessions Service.
func NewService(repo interviewStateRepo, m *statemachine.Machine) *Service {
	return &Service{repo: repo, machine: m}
}

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

	return s.repo.Get(ctx, orgID, sessionID)
}

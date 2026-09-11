package handlers

import (
	"context"
	"net/http"

	"oriva/backend-go/services/join"

	"github.com/go-chi/chi/v5"
)

type joinService interface {
	Status(ctx context.Context, token string) (*join.Status, error)
	Start(ctx context.Context, token string) error
}

// Join serves the public candidate join endpoints.
type Join struct{ svc joinService }

// NewJoinHandler constructs a Join handler.
func NewJoinHandler(svc joinService) *Join { return &Join{svc: svc} }

// Status handles GET /join/{token} (no auth — the token is the credential).
func (h *Join) Status(w http.ResponseWriter, r *http.Request) (any, int, error) {
	s, err := h.svc.Status(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		return nil, 0, err
	}
	return s, http.StatusOK, nil
}

// Start handles POST /join/{token}/start (no auth — the token is the
// credential). Called by ai-service right before it accepts the candidate's
// /ws connection, to drive the session's state machine up to "in_progress".
func (h *Join) Start(w http.ResponseWriter, r *http.Request) (any, int, error) {
	if err := h.svc.Start(r.Context(), chi.URLParam(r, "token")); err != nil {
		return nil, 0, err
	}
	return map[string]bool{"started": true}, http.StatusOK, nil
}

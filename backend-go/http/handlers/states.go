package handlers

import (
	"net/http"

	"oriva/backend-go/statemachine"
)

type sessionGraphService interface {
	Graph() statemachine.Graph
}

// Sessions serves session-state metadata (the graph React renders from).
type Sessions struct{ svc sessionGraphService }

// NewSessionsHandler constructs a Sessions handler.
func NewSessionsHandler(svc sessionGraphService) *Sessions { return &Sessions{svc: svc} }

// Graph handles GET /session-states.
func (h *Sessions) Graph(w http.ResponseWriter, r *http.Request) (any, int, error) {
	return h.svc.Graph(), http.StatusOK, nil
}

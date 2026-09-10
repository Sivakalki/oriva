// Package handlers holds the HTTP transport layer.
package handlers

import (
	"context"
	"net/http"

	apxresp "oriva/backend-go/http/response"
)

type healthService interface {
	Health(ctx context.Context) bool
}

// Health is the health-check handler.
type Health struct {
	svc healthService
}

// NewHealthHandler constructs a Health handler.
func NewHealthHandler(svc healthService) *Health { return &Health{svc: svc} }

// Check responds 200 when dependencies are reachable, 503 otherwise.
func (h *Health) Check(w http.ResponseWriter, r *http.Request) {
	if !h.svc.Health(r.Context()) {
		apxresp.RespondMessage(w, http.StatusServiceUnavailable, "health check failed")
		return
	}
	apxresp.RespondMessage(w, http.StatusOK, "ok")
}

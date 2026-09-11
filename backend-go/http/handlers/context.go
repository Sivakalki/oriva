package handlers

import (
	"net/http"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/utils/authctx"
)

// listEnvelope wraps a slice in {"data": [...]}, rendering nil as an empty array.
func listEnvelope[T any](items []T) map[string]any {
	if items == nil {
		items = []T{}
	}
	return map[string]any{"data": items}
}

// OrgID returns the caller's organization id from the JWT claims, or an
// Unauthorized error if the request is somehow unauthenticated.
func OrgID(r *http.Request) (string, error) {
	claims, ok := authctx.FromContext(r.Context())
	if !ok || claims.OrgID == "" {
		return "", &apxerrors.Error{Kind: apxerrors.Unauthorized, Message: "unauthorized"}
	}
	return claims.OrgID, nil
}

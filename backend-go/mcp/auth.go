package mcp

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// BearerAuth guards next with a static bearer token. The Go↔Python MCP boundary
// is an internal, trusted connection (docs/ARCHITECTURE.md §2); per-org tokens
// or mTLS are a later hardening slice.
func BearerAuth(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		got, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(got)), want) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

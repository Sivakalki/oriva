package middlewares

import (
	"net/http"
	"strings"

	apxerrors "oriva/backend-go/errors"
	apxresp "oriva/backend-go/http/response"
	"oriva/backend-go/utils/authctx"
	"oriva/backend-go/utils/jwt"
)

// tokenParser is the subset of *jwt.JWT the middleware needs.
type tokenParser interface {
	Parse(token string) (*jwt.Claims, error)
}

var errUnauthorized = &apxerrors.Error{Kind: apxerrors.Unauthorized, Message: "unauthorized"}

// RequireAuth validates the bearer token and stores the claims on the request context.
func RequireAuth(p tokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if raw == "" {
				apxresp.RespondError(w, errUnauthorized)
				return
			}
			claims, err := p.Parse(raw)
			if err != nil {
				apxresp.RespondError(w, errUnauthorized)
				return
			}
			ctx := authctx.WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole rejects requests whose claims role is not in roles. It must be
// mounted after RequireAuth.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := authctx.FromContext(r.Context())
			if !ok {
				apxresp.RespondError(w, errUnauthorized)
				return
			}
			if !contains(roles, claims.Role) {
				apxresp.RespondError(w, &apxerrors.Error{Kind: apxerrors.Forbidden, Message: "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

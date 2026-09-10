package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"oriva/backend-go/utils/authctx"
	"oriva/backend-go/utils/jwt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func okHandler(seen *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = true
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireAuth(t *testing.T) {
	j := jwt.New("secret", time.Hour)
	good, _, err := j.Issue("u1", "o1", jwt.RoleScheduler)
	require.NoError(t, err)

	t.Run("missing header", func(t *testing.T) {
		rr := httptest.NewRecorder()
		RequireAuth(j)(okHandler(nil)).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("bad token", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer garbage")
		RequireAuth(j)(okHandler(nil)).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("valid token injects claims", func(t *testing.T) {
		var gotRole string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := authctx.FromContext(r.Context())
			require.True(t, ok)
			gotRole = c.Role
		})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+good)
		RequireAuth(j)(h).ServeHTTP(rr, req)
		assert.Equal(t, jwt.RoleScheduler, gotRole)
	})
}

func TestRequireRole(t *testing.T) {
	base := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := authctx.WithClaims(base.Context(), &jwt.Claims{Role: jwt.RoleCandidate})
	req := base.WithContext(ctx)

	t.Run("wrong role forbidden", func(t *testing.T) {
		rr := httptest.NewRecorder()
		RequireRole(jwt.RoleScheduler)(okHandler(nil)).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("right role passes", func(t *testing.T) {
		rr := httptest.NewRecorder()
		var seen bool
		RequireRole(jwt.RoleCandidate)(okHandler(&seen)).ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.True(t, seen)
	})
}

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/services/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- health ---

type fakeHealth struct{ ok bool }

func (f fakeHealth) Health(context.Context) bool { return f.ok }

func TestHealthHandler(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		rr := httptest.NewRecorder()
		NewHealthHandler(fakeHealth{true}).Check(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
		assert.Equal(t, http.StatusOK, rr.Code)
	})
	t.Run("down", func(t *testing.T) {
		rr := httptest.NewRecorder()
		NewHealthHandler(fakeHealth{false}).Check(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})
}

// --- auth login ---

type fakeAuth struct {
	res auth.LoginResult
	err error
}

func (f fakeAuth) Login(context.Context, string, string) (auth.LoginResult, error) {
	return f.res, f.err
}

func callLogin(t *testing.T, svc authService, body string) (int, map[string]any) {
	t.Helper()
	h := NewAuthHandler(svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	out, status, err := h.Login(rr, req)
	if err != nil {
		var ae *apxerrors.Error
		require.True(t, apxerrors.As(err, &ae))
		return kindToStatus(ae.Kind), nil
	}
	m := map[string]any{}
	b, _ := json.Marshal(out)
	_ = json.Unmarshal(b, &m)
	return status, m
}

func kindToStatus(k apxerrors.Kind) int {
	switch k {
	case apxerrors.Invalid:
		return http.StatusBadRequest
	case apxerrors.Unauthorized:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func TestLoginHandler(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		svc := fakeAuth{res: auth.LoginResult{AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 3600}}
		status, body := callLogin(t, svc, `{"email":"a@b.com","password":"pw"}`)
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, "tok", body["access_token"])
	})
	t.Run("missing fields", func(t *testing.T) {
		status, _ := callLogin(t, fakeAuth{}, `{"email":"","password":""}`)
		assert.Equal(t, http.StatusBadRequest, status)
	})
	t.Run("bad creds", func(t *testing.T) {
		svc := fakeAuth{err: apxerrors.E(apxerrors.Unauthorized, "invalid credentials")}
		status, _ := callLogin(t, svc, `{"email":"a@b.com","password":"x"}`)
		assert.Equal(t, http.StatusUnauthorized, status)
	})
}

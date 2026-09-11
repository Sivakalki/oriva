package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/http/handlers"
	"oriva/backend-go/services/join"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeJoin struct {
	st  *join.Status
	err error
}

func (f fakeJoin) Status(context.Context, string) (*join.Status, error) { return f.st, f.err }
func (f fakeJoin) Start(context.Context, string) error                  { return f.err }

func TestJoinStatus_OK(t *testing.T) {
	st := &join.Status{JobTitle: "Role", Phase: join.PhaseBefore, ScheduledAt: time.Now().Add(time.Hour)}
	h := handlers.NewJoinHandler(fakeJoin{st: st})
	body, code, err := h.Status(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/join/tok", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "before", body.(*join.Status).Phase)
}

func TestJoinStatus_NotFound(t *testing.T) {
	h := handlers.NewJoinHandler(fakeJoin{err: apxerrors.E(apxerrors.NotFound, "invalid or expired interview link")})
	_, _, err := h.Status(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/join/x", nil))
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	assert.Equal(t, apxerrors.NotFound, ae.Kind)
}

func TestJoinStart_OK(t *testing.T) {
	h := handlers.NewJoinHandler(fakeJoin{})
	body, code, err := h.Start(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/join/tok/start", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, map[string]bool{"started": true}, body)
}

func TestJoinStart_NotFound(t *testing.T) {
	h := handlers.NewJoinHandler(fakeJoin{err: apxerrors.E(apxerrors.NotFound, "invalid or expired interview link")})
	_, _, err := h.Start(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/join/x/start", nil))
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	assert.Equal(t, apxerrors.NotFound, ae.Kind)
}

package auth_test

import (
	"context"
	"errors"
	"testing"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/user"
	"oriva/backend-go/services/auth"
	"oriva/backend-go/utils/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	u   *user.User
	err error
}

func (f fakeRepo) FindByEmail(context.Context, string) (*user.User, error) {
	return f.u, f.err
}

type fakeIssuer struct{ err error }

func (f fakeIssuer) Issue(string, string, string) (string, int, error) {
	if f.err != nil {
		return "", 0, f.err
	}
	return "tok", 3600, nil
}

func mustUser(t *testing.T, password string) *user.User {
	t.Helper()
	h, err := helpers.HashPassword(password, 10)
	require.NoError(t, err)
	return &user.User{ID: "u1", OrgID: "o1", Email: "a@b.com", Role: "scheduler", PasswordHash: h}
}

func TestLogin_Success(t *testing.T) {
	svc := auth.NewService(fakeRepo{u: mustUser(t, "pw")}, fakeIssuer{})
	res, err := svc.Login(context.Background(), "a@b.com", "pw")
	require.NoError(t, err)
	assert.Equal(t, "tok", res.AccessToken)
	assert.Equal(t, "Bearer", res.TokenType)
	assert.Equal(t, "u1", res.User.ID)
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := auth.NewService(fakeRepo{u: mustUser(t, "pw")}, fakeIssuer{})
	_, err := svc.Login(context.Background(), "a@b.com", "nope")
	assertUnauthorized(t, err)
}

func TestLogin_UnknownUser(t *testing.T) {
	svc := auth.NewService(fakeRepo{err: errors.New("not found")}, fakeIssuer{})
	_, err := svc.Login(context.Background(), "x@y.com", "pw")
	assertUnauthorized(t, err)
}

func TestLogin_SameErrorForBothFailures(t *testing.T) {
	a := auth.NewService(fakeRepo{u: mustUser(t, "pw")}, fakeIssuer{})
	_, e1 := a.Login(context.Background(), "a@b.com", "wrong")
	b := auth.NewService(fakeRepo{err: errors.New("not found")}, fakeIssuer{})
	_, e2 := b.Login(context.Background(), "a@b.com", "wrong")
	assert.Equal(t, e1.Error(), e2.Error())
}

func assertUnauthorized(t *testing.T, err error) {
	t.Helper()
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	assert.Equal(t, apxerrors.Unauthorized, ae.Kind)
}

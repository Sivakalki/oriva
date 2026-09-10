package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/services/auth"
	"oriva/backend-go/utils/authctx"
)

type authService interface {
	Login(ctx context.Context, email, password string) (auth.LoginResult, error)
}

// Auth holds the auth handlers.
type Auth struct {
	svc authService
}

// NewAuthHandler constructs an Auth handler.
func NewAuthHandler(svc authService) *Auth { return &Auth{svc: svc} }

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login authenticates a user and returns an access token.
func (a *Auth) Login(w http.ResponseWriter, r *http.Request) (any, int, error) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, 0, apxerrors.InvalidBodyErr(err)
	}
	req.Email = strings.TrimSpace(req.Email)

	ve := apxerrors.ValidationErrs()
	if req.Email == "" {
		ve.Add("email", "cannot be empty")
	}
	if req.Password == "" {
		ve.Add("password", "cannot be empty")
	}
	if err := ve.Err(); err != nil {
		return nil, 0, apxerrors.ValidationFailedErr(err)
	}

	res, err := a.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		return nil, 0, err
	}
	return res, http.StatusOK, nil
}

// Me returns the currently authenticated user.
func (a *Auth) Me(w http.ResponseWriter, r *http.Request) (any, int, error) {
	claims, ok := authctx.FromContext(r.Context())
	if !ok {
		return nil, 0, &apxerrors.Error{Kind: apxerrors.Unauthorized, Message: "unauthorized"}
	}
	return map[string]string{
		"id":     claims.Subject,
		"org_id": claims.OrgID,
		"role":   claims.Role,
	}, http.StatusOK, nil
}

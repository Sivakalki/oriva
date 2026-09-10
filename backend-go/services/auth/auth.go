// Package auth handles credential verification and access-token issuance.
package auth

import (
	"context"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/user"
	"oriva/backend-go/utils/helpers"
	"oriva/backend-go/utils/jwt"
)

// userRepo is the store contract, defined at the point of consumption.
type userRepo interface {
	FindByEmail(ctx context.Context, email string) (*user.User, error)
}

// tokenIssuer is the JWT contract.
type tokenIssuer interface {
	Issue(userID, orgID, role string) (token string, expiresIn int, err error)
}

// Service verifies credentials and issues tokens.
type Service struct {
	repo userRepo
	jwt  tokenIssuer
}

// NewService constructs an auth Service.
func NewService(repo userRepo, j tokenIssuer) *Service {
	return &Service{repo: repo, jwt: j}
}

// LoginResult is returned on a successful login.
type LoginResult struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresIn   int         `json:"expires_in"`
	User        user.Public `json:"user"`
}

var errInvalidCreds = apxerrors.E(apxerrors.Unauthorized, "invalid credentials")

// Login verifies email + password and returns an access token. It returns an
// identical error for unknown users and wrong passwords (no user enumeration).
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	u, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return LoginResult{}, errInvalidCreds
	}
	if !helpers.VerifyPassword(password, u.PasswordHash) {
		return LoginResult{}, errInvalidCreds
	}

	tok, expiresIn, err := s.jwt.Issue(u.ID, u.OrgID, u.Role)
	if err != nil {
		return LoginResult{}, apxerrors.E(apxerrors.Internal, "could not issue token", err)
	}
	return LoginResult{
		AccessToken: tok,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		User:        u.ToPublic(),
	}, nil
}

// Compile-time check that *jwt.JWT satisfies the local tokenIssuer interface.
var _ tokenIssuer = (*jwt.JWT)(nil)

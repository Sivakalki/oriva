// Package jwt issues and verifies the service's access tokens (HS256).
package jwt

import (
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// Roles.
const (
	RoleScheduler = "scheduler"
	RoleCandidate = "candidate"
)

// Claims is the token payload. Subject holds the user id.
type Claims struct {
	gojwt.RegisteredClaims
	OrgID string `json:"org_id"`
	Role  string `json:"role"`
}

// JWT issues and parses tokens with a fixed secret and access-token TTL.
type JWT struct {
	secret []byte
	ttl    time.Duration
}

// New returns a JWT signer/verifier.
func New(secret string, ttl time.Duration) *JWT {
	return &JWT{secret: []byte(secret), ttl: ttl}
}

// Issue returns a signed token for the user plus its lifetime in seconds.
func (j *JWT) Issue(userID, orgID, role string) (token string, expiresIn int, err error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(j.ttl)),
		},
		OrgID: orgID,
		Role:  role,
	}
	t := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(j.secret)
	if err != nil {
		return "", 0, err
	}
	return signed, int(j.ttl.Seconds()), nil
}

// Parse validates the token signature and expiry and returns its claims.
func (j *JWT) Parse(token string) (*Claims, error) {
	var claims Claims
	_, err := gojwt.ParseWithClaims(token, &claims, func(t *gojwt.Token) (any, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

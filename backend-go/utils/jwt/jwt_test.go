package jwt

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueParseRoundTrip(t *testing.T) {
	j := New("test-secret", time.Hour)
	tok, expiresIn, err := j.Issue("user-1", "org-1", RoleScheduler)
	require.NoError(t, err)
	assert.Equal(t, 3600, expiresIn)

	claims, err := j.Parse(tok)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.Subject)
	assert.Equal(t, "org-1", claims.OrgID)
	assert.Equal(t, RoleScheduler, claims.Role)
}

func TestParseExpired(t *testing.T) {
	j := New("test-secret", -time.Minute)
	tok, _, err := j.Issue("u", "o", RoleCandidate)
	require.NoError(t, err)

	_, err = j.Parse(tok)
	assert.Error(t, err)
}

func TestParseWrongSecret(t *testing.T) {
	tok, _, err := New("secret-a", time.Hour).Issue("u", "o", RoleCandidate)
	require.NoError(t, err)

	_, err = New("secret-b", time.Hour).Parse(tok)
	assert.Error(t, err)
}

func TestParseRejectsNonHMAC(t *testing.T) {
	// A token signed with alg=none must be rejected by the keyfunc.
	tok, err := gojwt.NewWithClaims(gojwt.SigningMethodNone, Claims{
		RegisteredClaims: gojwt.RegisteredClaims{Subject: "u"},
	}).SignedString(gojwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = New("secret", time.Hour).Parse(tok)
	assert.Error(t, err)
}

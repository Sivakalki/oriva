package helpers

import (
	"crypto/rand"
	"encoding/base64"
)

// NewJoinToken returns a URL-safe random token (24 chars from 18 bytes) that
// lets a candidate open their interview without an account.
func NewJoinToken() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

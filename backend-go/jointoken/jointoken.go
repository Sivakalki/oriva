// Package jointoken mints the opaque token that lets a candidate open their
// interview without an account.
package jointoken

import (
	"crypto/rand"
	"encoding/base64"
)

// New returns a URL-safe random token (24 chars from 18 bytes).
func New() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

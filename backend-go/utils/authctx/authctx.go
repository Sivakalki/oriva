// Package authctx carries authenticated request identity through context.
// It is a leaf package so both middleware and handlers can import it without a cycle.
package authctx

import (
	"context"

	"oriva/backend-go/utils/jwt"
)

type ctxKey struct{}

// WithClaims returns a copy of ctx carrying the given claims.
func WithClaims(ctx context.Context, c *jwt.Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext returns the claims stored on ctx, if any.
func FromContext(ctx context.Context) (*jwt.Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(*jwt.Claims)
	return c, ok
}

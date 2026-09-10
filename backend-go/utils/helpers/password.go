// Package helpers holds small, dependency-light utilities shared across packages.
package helpers

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash of plain at the given cost.
// Cost is clamped into bcrypt's valid range.
func HashPassword(plain string, cost int) (string, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword reports whether plain matches the bcrypt hash.
func VerifyPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

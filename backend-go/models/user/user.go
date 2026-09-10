// Package user holds the User model shared across the repo, service and handler layers.
package user

import "time"

// User is a platform account. Role is "scheduler" or "candidate".
type User struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Public is the User view safe to return over the API.
type Public struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
	OrgID string `json:"org_id"`
}

// Public returns the API-safe projection of u.
func (u User) ToPublic() Public {
	return Public{ID: u.ID, Email: u.Email, Role: u.Role, OrgID: u.OrgID}
}

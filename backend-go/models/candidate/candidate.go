// Package candidate holds the Candidate model shared across layers.
package candidate

import "time"

// Candidate is a person being interviewed. Email is unique per organization.
type Candidate struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	ResumeText string    `json:"resume_text"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

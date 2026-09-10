// Package job holds the Job model shared across repo, service and handler layers.
package job

import "time"

// Job is a role a candidate is interviewed for.
type Job struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

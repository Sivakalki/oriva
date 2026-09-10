// Package interview holds the interview-session models shared across layers.
package interview

import "time"

// Interview is the raw interview_sessions row.
type Interview struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	JobID       string    `json:"job_id"`
	CandidateID string    `json:"candidate_id"`
	State       string    `json:"state"`
	ScheduledAt time.Time `json:"scheduled_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// JobRef is the embedded job summary in Detail.
type JobRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// CandRef is the embedded candidate summary in Detail.
type CandRef struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Detail is the API projection for GET /{id} and list entries.
type Detail struct {
	ID          string    `json:"id"`
	State       string    `json:"state"`
	StateLabel  string    `json:"state_label"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Job         JobRef    `json:"job"`
	Candidate   CandRef   `json:"candidate"`
	CreatedAt   time.Time `json:"created_at"`

	JoinToken string `json:"join_token"`
	JoinURL   string `json:"join_url,omitempty"`
}

// Filter is the optional set of list constraints. Empty fields are ignored.
type Filter struct {
	State       string
	JobID       string
	CandidateID string
}

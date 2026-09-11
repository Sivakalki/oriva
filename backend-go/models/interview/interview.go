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

// TurnScore is one scored interview turn (services/scoring). Recruiter-only:
// only reachable via GET /interviews/{id}, which sits behind
// RequireRole(RoleScheduler) -- the candidate's public /join/{token} status
// uses the unrelated join.Status type and never sees this.
type TurnScore struct {
	TurnIndex int     `json:"turn_index"`
	Question  string  `json:"question"`
	Answer    string  `json:"answer"`
	Value     float64 `json:"value"`
	Rationale string  `json:"rationale"`
}

// ScoreSummary is the session's overall score, once computed.
type ScoreSummary struct {
	Value     float64   `json:"value"`
	Rationale string    `json:"rationale"`
	Model     string    `json:"model"`
	ScoredAt  time.Time `json:"scored_at"`
}

// Detail is the API projection for GET /{id} and list entries. OverallScore
// and TurnScores are only populated by Get (not List, to keep the dashboard
// table cheap) and only once scoring has actually run -- nil/empty until then.
type Detail struct {
	ID              string    `json:"id"`
	State           string    `json:"state"`
	StateLabel      string    `json:"state_label"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	DurationMinutes int       `json:"duration_minutes"`
	Job             JobRef    `json:"job"`
	Candidate       CandRef   `json:"candidate"`
	CreatedAt       time.Time `json:"created_at"`

	JoinToken string `json:"join_token"`
	JoinURL   string `json:"join_url,omitempty"`

	OverallScore *ScoreSummary `json:"overall_score,omitempty"`
	TurnScores   []TurnScore   `json:"turn_scores,omitempty"`
}

// Filter is the optional set of list constraints. Empty fields are ignored.
type Filter struct {
	State       string
	JobID       string
	CandidateID string
}

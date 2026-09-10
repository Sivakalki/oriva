# Backend-Go — Slice 2 Design (Recruiter CRUD + Interview Scheduling)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** slice 1 (`2026-09-10-backend-go-slice1-design.md`, commit `c9618ee`)
**Scope:** Phase 0 CRUD. Jobs, candidates, and interview scheduling for the
`scheduler` role. No session state machine, no candidate access flow, no email,
no scoring — those are later slices.

## 1. Goal

Give a recruiter (`scheduler`) the API to manage jobs and candidate profiles and
to schedule interviews against them (`docs/PLAN.md` Phase 0). Every route is
authenticated and scoped to the caller's organization via the JWT `org_id` claim.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | All routes require `RequireAuth` + `RequireRole("scheduler")` | Only schedulers do CRUD in v1 |
| D2 | Org scope comes from the JWT `org_id` claim, never from the request body | Prevents cross-org writes |
| D3 | Add `org_id` to `interview_sessions` (migration 0003) | Cross-org isolation is a `WHERE org_id = $1`, not a multi-join |
| D4 | Scheduling inserts the literal state `"scheduled"` | State machine / transition validation is slice 4; the FK to `session_states` still holds |
| D5 | No DELETE endpoints | YAGNI; add soft-delete later if a real need appears |
| D6 | PATCH semantics: only provided fields change; `email` and `role` are immutable via PATCH | Email is the candidate identity key |
| D7 | `scheduled_at` must be RFC3339 and in the future | Cheap guard against obvious mistakes |
| D8 | `interviews` list supports `?state=`, `?job_id=`, `?candidate_id=` filters | Recruiter dashboard needs them; all optional |
| D9 | One repo file per aggregate, one service package per aggregate | Matches slice 1's `services/auth` + `db/postgres/user.go` pattern |

## 3. Migration 0003 — `interview_sessions.org_id`

```sql
-- up
ALTER TABLE interview_sessions
    ADD COLUMN org_id uuid NOT NULL REFERENCES organizations (id);
CREATE INDEX interview_sessions_org_id_idx ON interview_sessions (org_id);
```
```sql
-- down
DROP INDEX IF EXISTS interview_sessions_org_id_idx;
ALTER TABLE interview_sessions DROP COLUMN IF EXISTS org_id;
```
The dev/test databases have no `interview_sessions` rows, so `NOT NULL` with no
default is safe. (If that ever stops being true, the planner adds a backfill.)

## 4. Models (`models/`)

```go
// models/job
type Job struct {
    ID, OrgID, Title, Description string
    CreatedAt, UpdatedAt          time.Time
}

// models/candidate
type Candidate struct {
    ID, OrgID, Email, Name, ResumeText string
    CreatedAt, UpdatedAt               time.Time
}

// models/interview
type Interview struct {
    ID, OrgID, JobID, CandidateID, State string
    ScheduledAt                          time.Time
    CreatedAt, UpdatedAt                 time.Time
}

// Detail is the GET /{id} and list projection.
type Detail struct {
    ID          string     `json:"id"`
    State       string     `json:"state"`
    StateLabel  string     `json:"state_label"`
    ScheduledAt time.Time   `json:"scheduled_at"`
    Job         JobRef     `json:"job"`        // {id, title}
    Candidate   CandRef    `json:"candidate"`  // {id, name, email}
    CreatedAt   time.Time   `json:"created_at"`
}
```

## 5. Repositories (`db/postgres/`)

All queries filter by `org_id`. `ErrNotFound` (existing sentinel) on no rows.

### `job.go` — `JobRepo`
- `Create(ctx, orgID, title, description) (*job.Job, error)`
- `ListByOrg(ctx, orgID) ([]job.Job, error)` — newest first
- `Get(ctx, orgID, id) (*job.Job, error)`
- `Update(ctx, orgID, id string, title, description *string) (*job.Job, error)` —
  `COALESCE`-style partial update; `ErrNotFound` if no row in org
- `ExistsInOrg(ctx, orgID, id) (bool, error)` — used by scheduling

### `candidate.go` — `CandidateRepo`
- `Create(ctx, orgID, email, name, resumeText) (*candidate.Candidate, error)` —
  maps the `(org_id, email)` unique-violation (`23505`) to a `Conflict` sentinel
  `ErrConflict`
- `ListByOrg`, `Get`, `Update(name, resumeText *string)`, `ExistsInOrg` — as jobs

### `interview.go` — `InterviewRepo`
- `Schedule(ctx, orgID, jobID, candidateID string, scheduledAt time.Time) (*interview.Interview, error)`
  — inserts `state = 'scheduled'`
- `Get(ctx, orgID, id) (*interview.Detail, error)` — joins `jobs`, `candidates`,
  `session_states` for the label
- `List(ctx, orgID string, f InterviewFilter) ([]interview.Detail, error)` —
  `f` has optional `State`, `JobID`, `CandidateID`; built with conditional
  `WHERE` fragments and positional args

## 6. Services (`services/`)

Each package mirrors `services/auth`: a struct holding a consumer-defined repo
interface, constructor `NewService`, methods returning `*apxerrors.Error` for
domain failures.

### `services/jobs`
- `Create(ctx, orgID, in CreateInput) (*job.Job, error)` — validates non-empty
  `title` (≤ 200 chars), `description` ≤ 20 000 chars
- `List`, `Get`, `Update` — `Update` requires at least one field present

### `services/candidates`
- `Create` — validates `email` (RFC 5322-ish via `net/mail.ParseAddress`),
  `name` non-empty ≤ 200, `resume_text` ≤ 100 000; passes repo `ErrConflict`
  through as `apxerrors.E(Conflict, "candidate already exists")`
- `List`, `Get`, `Update`

### `services/interviews`
- `Schedule(ctx, orgID, in ScheduleInput) (*interview.Detail, error)`:
  1. validate `scheduled_at` present, RFC3339, `After(now)`
  2. `jobRepo.ExistsInOrg` and `candRepo.ExistsInOrg` — either false → `NotFound`
  3. `repo.Schedule`, then `repo.Get` for the detail projection
  - consumes three interfaces: `interviewRepo`, `jobChecker`, `candChecker`
- `List(ctx, orgID, filter)`, `Get(ctx, orgID, id)`

## 7. HTTP (`http/handlers/`)

New `handlers/context.go` helper:
```go
func orgID(r *http.Request) (string, error)   // authctx.FromContext -> claims.OrgID, or *Error Unauthorized
```

### `jobs.go` — `Jobs` handler
`Create`, `List`, `Get`, `Update` as `(any, int, error)` funcs. Body structs with
JSON tags; decode error → `apxerrors.InvalidBodyErr`; validation via
`apxerrors.ValidationErrs`. `{id}` read with `chi.URLParam`.

### `candidates.go` — `Candidates` handler
Same shape.

### `interviews.go` — `Interviews` handler
- `Schedule` — `POST`, 201 on success
- `List` — reads filters from `r.URL.Query()`
- `Get` — 404 passthrough from service

### Router (`http/server.go`)
```go
r.Route("/jobs", func(r chi.Router) {
    r.Use(mw.RequireAuth(s.jwt), mw.RequireRole(jwt.RoleScheduler))
    r.Post("/", s.toHandlerFunc(s.jobs.Create))
    r.Get("/", s.toHandlerFunc(s.jobs.List))
    r.Get("/{id}", s.toHandlerFunc(s.jobs.Get))
    r.Patch("/{id}", s.toHandlerFunc(s.jobs.Update))
})
// candidates, interviews likewise
```
`Server` gains `jobs *handlers.Jobs`, `candidates *handlers.Candidates`,
`interviews *handlers.Interviews` (alphabetical order kept). `initServer` wires
the three repos + services.

## 8. Response / status mapping

| Situation | Kind | HTTP |
|---|---|---|
| create success | — | 201 (jobs/candidates/interviews POST) |
| get/list/update success | — | 200 |
| bad body / validation | `Invalid` | 400 |
| unknown id (any org) or referenced job/candidate not in org | `NotFound` | 404 |
| duplicate candidate email in org | `Conflict` | 409 |
| missing/rejected token | `Unauthorized` | 401 |
| non-scheduler role | `Forbidden` | 403 |

`response.RespondError` already covers `Conflict` → 409 (added in slice 1). POST
handlers return `http.StatusCreated` explicitly.

## 9. Testing

Unit (`go test ./...`):
- `services/jobs` — create validation (empty title, over-long), update-with-no-fields
  rejected, list/get pass-through, `Get` not-found.
- `services/candidates` — email parse rejection, conflict pass-through, happy path.
- `services/interviews` — past `scheduled_at` rejected, unknown job → 404, unknown
  candidate → 404, happy path calls `Schedule` then `Get`.
- `http/handlers` — one happy + one error path per handler via `httptest` with fake
  services; `orgID` helper returns 401 when claims absent.

Integration (`//go:build integration`, `make test-integration`):
- `JobRepo` CRUD round trip; `Update` partial; cross-org `Get` returns `ErrNotFound`.
- `CandidateRepo` duplicate `(org_id, email)` → `ErrConflict`; different org same
  email → ok.
- `InterviewRepo.Schedule` inserts `scheduled`; `Get` returns `state_label = "Scheduled"`;
  `List` filters by state/job/candidate; scheduling into another org's job is
  blocked at the service layer (covered in a service-level test with real repos).

## 10. Out of scope (later slices)

- Session state machine: loading `session_state_transitions`, `advance_state`,
  any transition past the initial `scheduled` insert.
- Candidate `users` provisioning, magic-link / join-token flow, candidate-role
  endpoints.
- Email / notification sending.
- Scoring, responses, rubric.
- MCP server.
- Pagination on list endpoints (add when a dataset is big enough to need it).
- DELETE / archival.

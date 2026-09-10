# Backend-Go — Slice 3 Design (Session State Machine)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** slice 1 (`c9618ee`), slice 2 (`d152dcf`)
**Scope:** Go becomes the sole authority for validating and applying interview
session state transitions (`docs/ARCHITECTURE.md` §4). No MCP tool yet, no
pipeline-driven auto-transitions.

## 1. Goal

The 12 canonical states and the legal transition graph already live in Postgres
(`session_states`, `session_state_transitions`, seeded in migration 0002). This
slice loads them into memory at startup and puts every state change through one
validated path, with an append-only audit trail.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | Graph loaded once at startup into an immutable in-memory `Machine`; a malformed graph fails startup | `docs/ARCHITECTURE.md` §4 — Go loads the tables at boot and is the only validator |
| D2 | `statemachine` is a pure package (no db, no http) | Trivially and exhaustively unit-testable |
| D3 | Transition applied with optimistic CAS (`WHERE state = $from`) | Two concurrent `advance` calls must not both win |
| D4 | Every applied transition appends a `session_state_events` row in the same tx | Auditable trail (`docs/PROJECT_SCOPE.md` compliance, `ARCHITECTURE.md` §5) |
| D5 | `Schedule` still inserts the literal `"scheduled"`; the machine is not consulted there | Creating a session is not a transition |
| D6 | `GET /session-states` returns the whole graph | React reads `{name,label,is_terminal}` off the API, never bakes the enum (`ARCHITECTURE.md` §4) |
| D7 | Error mapping: unknown target → `Invalid` 400; illegal / terminal / CAS-race → `Conflict` 409; missing session → `NotFound` 404 | Distinguishes "bad request" from "not allowed from here" from "gone" |

## 3. `statemachine` package (`statemachine/`)

```go
type State struct {
    Name       string
    Label      string
    IsTerminal bool
}

type Transition struct{ From, To string }

type Graph struct {
    States      []State      `json:"states"`
    Transitions []Transition `json:"transitions"`
}

type Machine struct {
    labels   map[string]string
    terminal map[string]bool
    edges    map[string]map[string]struct{} // from -> set(to)
    order    []State                        // preserves seed order for Graph()
}

var (
    ErrUnknownState      = errors.New("unknown session state")
    ErrIllegalTransition = errors.New("illegal session state transition")
    ErrTerminalState     = errors.New("session state is terminal")
)

// New validates that every transition references a known state and returns the
// Machine. Returns an error (fails server startup) otherwise.
func New(states []State, transitions []Transition) (*Machine, error)

func (m *Machine) Has(state string) bool
func (m *Machine) IsTerminal(state string) bool
func (m *Machine) Label(state string) string

// Validate returns nil if from -> to is a legal edge. Errors:
//   - ErrUnknownState      if from or to is not a known state
//   - ErrTerminalState     if from is terminal (wraps ErrIllegalTransition too)
//   - ErrIllegalTransition if the edge is not in the graph
func (m *Machine) Validate(from, to string) error

func (m *Machine) Graph() Graph
```

No knowledge of the specific 12 states is compiled in — it is whatever the tables
hold. Tests assert the real seeded graph behaves per `ARCHITECTURE.md` §4.

## 4. Migration 0004 — `session_state_events`

```sql
-- up
CREATE TABLE session_state_events (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES interview_sessions (id),
    org_id     uuid NOT NULL REFERENCES organizations (id),
    from_state text NOT NULL REFERENCES session_states (name),
    to_state   text NOT NULL REFERENCES session_states (name),
    reason     text NOT NULL DEFAULT '',
    actor      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX session_state_events_session_id_idx ON session_state_events (session_id, created_at);
```
```sql
-- down
DROP TABLE IF EXISTS session_state_events;
```

## 5. Repository changes (`db/postgres/`)

### `state.go` (new)
```go
func LoadGraph(ctx context.Context, pool *pgxpool.Pool) ([]statemachine.State, []statemachine.Transition, error)
```
Two queries: `SELECT name, label, is_terminal FROM session_states ORDER BY name`
(seed order is not stored, so order by name — `Graph()` output is sorted, which
is fine for the API) and `SELECT from_state, to_state FROM session_state_transitions`.

*(Note: `statemachine` types are re-exported here; `db/postgres` importing
`statemachine` is a clean direction — no cycle.)*

### `interview.go` (additions)
```go
// CurrentState returns the session's state or ErrNotFound.
func (r *InterviewRepo) CurrentState(ctx context.Context, orgID, id string) (string, error)

// ApplyTransition moves the session from -> to and logs the event, atomically.
// Returns ErrConflict if the row's state is no longer `from` (lost CAS race or
// stale caller), ErrNotFound if the session does not exist in the org.
func (r *InterviewRepo) ApplyTransition(ctx context.Context, orgID, id, from, to, reason, actor string) error
```
`ApplyTransition` uses `pool.Begin` → `UPDATE ... WHERE org_id=$1 AND id=$2 AND state=$3`
(check `CommandTag.RowsAffected()`; 0 → distinguish not-found vs conflict with a
follow-up existence check) → `INSERT INTO session_state_events (...)` → `Commit`.

## 6. `services/sessions` package (new)

```go
type interviewStateRepo interface {
    CurrentState(ctx context.Context, orgID, id string) (string, error)
    ApplyTransition(ctx context.Context, orgID, id, from, to, reason, actor string) error
    Get(ctx context.Context, orgID, id string) (*interview.Detail, error)
}

type Service struct {
    repo    interviewStateRepo
    machine *statemachine.Machine
}

func NewService(repo interviewStateRepo, m *statemachine.Machine) *Service

// Advance validates and applies the transition, then returns the refreshed Detail.
func (s *Service) Advance(ctx context.Context, orgID, sessionID, toState, reason, actor string) (*interview.Detail, error)

// Graph exposes the loaded state graph for the API.
func (s *Service) Graph() statemachine.Graph
```

`Advance`:
1. `toState` non-empty and `machine.Has(toState)` — else `apxerrors.E(Invalid, "unknown target state")`.
2. `cur, err := repo.CurrentState(...)` — `ErrNotFound` → `apxerrors.E(NotFound, "interview not found")`.
3. `machine.Validate(cur, toState)` — map `ErrUnknownState`→`Invalid`, `ErrTerminalState`/`ErrIllegalTransition`→`Conflict` with a message naming `cur` and `toState`.
4. `repo.ApplyTransition(...)` — `ErrConflict` → `apxerrors.E(Conflict, "session state changed concurrently; retry")`.
5. `return repo.Get(...)`.

## 7. HTTP (`http/`)

### `handlers/interviews.go` — add `Advance`
```go
type advanceBody struct {
    ToState string `json:"to_state"`
    Reason  string `json:"reason"`
}
func (h *Interviews) Advance(w, r) (any, int, error) // 200 with updated Detail
```
`actor` from `authctx.FromContext(...).Subject`. The `interviewsService` interface
in the handler gains `Advance(ctx, orgID, id, toState, reason, actor string) (*interview.Detail, error)`.

*Decision:* the `Interviews` handler takes a second dependency (the sessions
service) rather than folding state logic into `services/interviews`. Constructor
becomes `NewInterviewsHandler(interviews interviewsService, sessions sessionsService)`.

### `handlers/states.go` (new) — `Sessions` handler
```go
func (h *Sessions) Graph(w, r) (any, int, error) // 200 with statemachine.Graph
```

### `server.go`
```go
r.Route("/interviews", func(r chi.Router) {
    ...
    r.Post("/{id}/advance", s.toHandlerFunc(s.interviews.Advance))
})
r.Get("/session-states", s.toHandlerFunc(s.sessions.Graph))
```
Both under the existing `RequireAuth` + `RequireRole(scheduler)` group.
`Server`/`Handlers` gain a `Sessions *handlers.Sessions` field.

## 8. Wiring (`cmd/server/main.go`)

```go
states, transitions, err := postgres.LoadGraph(ctx, pool)      // after migrations
machine, err := statemachine.New(states, transitions)          // fatal on error
...
sessionsSvc := sessions.NewService(interviewRepo, machine)
hs := apxhttp.Handlers{
    ...
    Interviews: handlers.NewInterviewsHandler(interviews.NewService(...), sessionsSvc),
    Sessions:   handlers.NewSessionsHandler(sessionsSvc),
}
```
`LoadGraph` runs after `RunUp` (when `auto_migrate`) so the seed rows exist.

## 9. Testing

Unit (`go test ./...`):
- **`statemachine`** — build from the real seeded graph (hard-coded fixture mirroring migration 0002):
  - happy path chain `scheduled→invited→…→scored` each `Validate` == nil
  - `interrupted→dispatched` allowed; `dispatched→interrupted` allowed
  - every state in `{scored,declined,abandoned,failed}` → `IsTerminal` true and
    `Validate(terminal, anything)` returns `ErrTerminalState`
  - a sampling of illegal edges (`scheduled→completed`, `ready→scored`,
    `invited→in_progress`) → `ErrIllegalTransition`
  - `Validate("bogus", "invited")` and `Validate("ready","bogus")` → `ErrUnknownState`
  - `New` with a transition referencing an undefined state → error
  - `Graph()` returns all states + transitions
- **`services/sessions`** — fake repo + a machine built from the fixture:
  - `scheduled`→`invited` success calls `ApplyTransition` then `Get`
  - illegal (`ready`→`scored`) → `Conflict`, `ApplyTransition` not called
  - unknown target → `Invalid`
  - `CurrentState` returns `ErrNotFound` → `NotFound`
  - `ApplyTransition` returns `ErrConflict` → `Conflict`
- **`http/handlers`** — `Advance` happy (200 + Detail), bad body → 400, illegal → 409
  via `httptest` + fake sessions service; `Graph` returns the graph JSON shape.

Integration (`//go:build integration`, `-p 1`):
- migrate up (0004 present); seed org/job/candidate; `Schedule`.
- `LoadGraph` returns 12 states, 21 transitions (matches migration 0002).
- `ApplyTransition` `scheduled→invited` succeeds; a second `ApplyTransition`
  `scheduled→invited` on the same row → `ErrConflict` (state already moved).
- `session_state_events` has one row with the right `from`/`to`/`actor`.
- `ApplyTransition` on an unknown session id → `ErrNotFound`.

## 10. Out of scope

- MCP `advance_state` tool (a later slice; it will call `sessions.Service.Advance`).
- Auto-transitions from the Python pipeline (`dispatched`/`in_progress`/`interrupted`
  are driven by the live call later).
- `scoring`/`scored` automation (no scoring engine yet — `ARCHITECTURE.md` §4:
  "code paths for scoring/scored don't need to be wired up until scoring exists").
- Rejoin orchestration beyond allowing the `interrupted→dispatched` edge.
- Notifying React of label changes (it re-fetches `/session-states`).

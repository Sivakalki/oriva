# Backend-Go — Slice 4 Design (MCP Server Skeleton)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** slice 1 (`c9618ee`), slice 2 (`d152dcf`), slice 3 (`479b069`)
**Scope:** The Go↔Python tool boundary (`docs/ARCHITECTURE.md` §2). Go implements
an MCP server over Streamable HTTP exposing four tools with owned schemas
(`oriva.tools.v1`). No dynamic question generation, no pgvector retrieval, no
Python-side wiring.

## 1. Goal

`docs/ARCHITECTURE.md` §2: "Go implements an MCP server and owns the tool schemas
(JSON Schema, versioned as `oriva.tools.v1`). Python's Pipecat pipeline runs an
`MCPClient` that connects to Go's MCP server, discovers the available tools, and
hands their schemas to the LLM so it can call them mid-conversation — e.g.
`get_interview_plan`, `retrieve_context`, `record_turn`, `advance_state`."

This slice stands up that server so the contract is testable independently of the
Python side (`docs/PLAN.md` Phase 0: "expose the tool surface ... even before the
Python side is built").

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | `github.com/modelcontextprotocol/go-sdk` v1.8.0 | Official SDK; typed tools → generated JSON Schema; ships a Streamable HTTP handler |
| D2 | `/mcp` mounted at the **root**, not under `/api/v1` | It is not REST; Python's `mcp.server_url` is `http://…:8080/mcp`; different auth |
| D3 | Static bearer token (`mcp.auth_token`), checked by `mcp/auth.go` | Python is a trusted internal service; per-org tokens / mTLS is a later hardening slice |
| D4 | Each tool resolves `org_id` from `session_id` via `InterviewRepo.OrgOf` | The AI service operates on one session; the session carries the org — no JWT needed on this boundary |
| D5 | `record_turn` and `advance_state` are real; `get_interview_plan` is a real read with canned questions; `retrieve_context` is a stub | Matches what already exists (responses table, sessions service) vs Phase 2 work |
| D6 | Tools only — no MCP resources or prompts | Nothing needs them yet |
| D7 | Schema version string `oriva.tools.v1` exposed via the server's instructions and a per-tool annotation | `ARCHITECTURE.md` names the version; consumers can pin it |

## 3. Dependency

`go get github.com/modelcontextprotocol/go-sdk@v1.8.0`. Used APIs: `mcp.NewServer`,
`mcp.AddTool[In, Out]`, `mcp.NewStreamableHTTPHandler`, `mcp.NewInMemoryTransports`
+ `mcp.NewClient` (tests).

## 4. Layout

```
backend-go/
  mcp/
    server.go     # NewServer(Deps) -> *Server; Handler() http.Handler
    tools.go      # In/Out structs (jsonschema tags) + the four handlers
    auth.go       # BearerAuth(token) middleware
    server_test.go
    auth_test.go
  db/postgres/
    response.go   # ResponseRepo (new)
    interview.go  # + OrgOf(ctx, sessionID) (string, error)
  config/config.go        # + MCP section
  http/server.go          # mount the MCP handler when enabled
  cmd/server/main.go      # wire it
```

## 5. Config (`config/config.go`)

```yaml
mcp:
  enabled: true
  path: "/mcp"
  auth_token: "dev-mcp-token"
```
```go
type MCP struct {
    Enabled   bool   `koanf:"enabled"`
    Path      string `koanf:"path"`
    AuthToken string `koanf:"auth_token"`
}
```
`Validate()`: when `Enabled`, `path` must start with `/` (default `/mcp`),
`auth_token` non-empty, and ≥16 chars when `IsProdMode`.

## 6. Repository changes

### `db/postgres/interview.go` — `OrgOf`
```go
// OrgOf returns the org that owns the session, or ErrNotFound.
func (r *InterviewRepo) OrgOf(ctx context.Context, sessionID string) (string, error)
```
`SELECT org_id FROM interview_sessions WHERE id = $1`.

### `db/postgres/response.go` — `ResponseRepo` (new)
```go
type ResponseRepo struct{ pool *pgxpool.Pool }
func NewResponseRepo(pool *pgxpool.Pool) *ResponseRepo

// Record inserts a turn. If turnIndex <= 0 it uses max(turn_index)+1 for the
// session (starting at 1). Returns the stored turn_index. A unique (session_id,
// turn_index) collision returns ErrConflict.
func (r *ResponseRepo) Record(ctx context.Context, sessionID string, turnIndex int, question, answer string) (int, error)
```
Implementation: one tx — `SELECT coalesce(max(turn_index),0)+1 ... FOR UPDATE`
when `turnIndex <= 0`, then `INSERT ... RETURNING turn_index`; map SQLSTATE
`23505` → `ErrConflict`.

## 7. MCP tools (`mcp/tools.go`)

`oriva.tools.v1`. Each `In`/`Out` is a plain struct with `json` + `jsonschema`
tags; the SDK generates the schema.

### `get_interview_plan`
```go
type PlanIn  struct { SessionID string `json:"session_id" jsonschema:"the interview session id"` }
type PlanOut struct {
    SessionID        string   `json:"session_id"`
    State            string   `json:"state"`
    JobTitle         string   `json:"job_title"`
    JobDescription   string   `json:"job_description"`
    CandidateName    string   `json:"candidate_name"`
    CandidateResume  string   `json:"candidate_resume"`
    Questions        []string `json:"questions"`
    SchemaVersion    string   `json:"schema_version"` // "oriva.tools.v1"
}
```
Handler: `OrgOf` → a new `InterviewRepo.PlanData(ctx, orgID, sessionID)` returning
the joined job/candidate/state fields → attach three canned opener questions
(`"Walk me through a recent project you're proud of."`, etc.).

### `retrieve_context` (stub)
```go
type RetrieveIn  struct {
    SessionID string `json:"session_id"`
    Query     string `json:"query" jsonschema:"what to search the resume/JD for"`
    K         int    `json:"k,omitempty" jsonschema:"max chunks, default 5"`
}
type RetrieveOut struct {
    Chunks []string `json:"chunks"`
    Note   string   `json:"note"`
}
```
Handler: validate the session exists (`OrgOf`), return `{Chunks: [], Note:
"pgvector retrieval not implemented in this slice"}`.

### `record_turn`
```go
type RecordIn struct {
    SessionID string `json:"session_id"`
    Question  string `json:"question"`
    Answer    string `json:"answer"`
    TurnIndex int    `json:"turn_index,omitempty" jsonschema:"omit to auto-append"`
}
type RecordOut struct { Recorded bool `json:"recorded"`; TurnIndex int `json:"turn_index"` }
```
Handler: `OrgOf` (existence check) → `ResponseRepo.Record` → `{true, idx}`.
`ErrConflict` → MCP tool error "turn_index already recorded".

### `advance_state`
```go
type AdvanceIn  struct {
    SessionID string `json:"session_id"`
    ToState   string `json:"to_state" jsonschema:"target session state"`
    Reason    string `json:"reason,omitempty"`
}
type AdvanceOut struct { State string `json:"state"`; StateLabel string `json:"state_label"` }
```
Handler: `OrgOf` → `sessions.Service.Advance(ctx, org, sessionID, toState, reason, "ai-service")`
→ `{d.State, d.StateLabel}`. A `*apxerrors.Error` becomes an MCP tool error whose
text is the message (the LLM sees "cannot move from ready to invited" and can
react).

### Error convention
Handlers return `(*mcp.CallToolResult, Out, error)`. Domain failures →
`return nil, Out{}, fmt.Errorf(...)` (SDK marks the tool call `isError`);
`ErrNotFound` → `"session not found"`. Infra failures propagate as Go errors and
are logged.

## 8. Server (`mcp/server.go`)

```go
type Deps struct {
    Interviews  interviewReader   // OrgOf, PlanData
    Responses   responseRecorder  // Record
    Sessions    stateAdvancer     // Advance
    Logger      *zap.Logger
}

func NewServer(d Deps) *Server               // builds *mcp.Server, AddTool x4
func (s *Server) Handler() http.Handler      // mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcp })
```
The `mcp.Server` is created with `&mcp.Implementation{Name: "oriva-backend",
Version: buildinfo.Version}` and `Instructions` mentioning `oriva.tools.v1`.
Consumer-defined interfaces (`interviewReader`, `responseRecorder`,
`stateAdvancer`) keep `mcp` testable without a DB.

## 9. Auth (`mcp/auth.go`)

```go
func BearerAuth(token string, next http.Handler) http.Handler
```
Reads `Authorization: Bearer <t>`; constant-time compare against `token`; 401
`{"error":"unauthorized"}` on mismatch. Applied in `http/server.go` around the
MCP handler.

## 10. HTTP mount (`http/server.go`)

`Server`/`Handlers` gain `MCP http.Handler` (already auth-wrapped) and `MCPPath string`.
```go
if s.mcp != nil {
    r.Handle(s.mcpPath, s.mcp)
    r.Handle(s.mcpPath+"/*", s.mcp)
}
```
Mounted on the root router, before the `/api/v1` route group. The logger/metrics
middleware still applies (one `http_requests_total{route="/mcp"}` series).

## 11. Wiring (`cmd/server/main.go`)

```go
if cfg.MCP.Enabled {
    mcpSrv := mcp.NewServer(mcp.Deps{
        Interviews: interviewRepo,
        Responses:  postgres.NewResponseRepo(pool),
        Sessions:   sessionsSvc,
        Logger:     logger,
    })
    hs.MCP = mcpmw.BearerAuth(cfg.MCP.AuthToken, mcpSrv.Handler())
    hs.MCPPath = cfg.MCP.Path
}
```

## 12. Testing

Unit (`go test ./...`):
- **`mcp`** — `NewServer` with fakes; connect via `mcp.NewInMemoryTransports` + `mcp.NewClient`:
  - `ListTools` returns exactly `get_interview_plan`, `retrieve_context`,
    `record_turn`, `advance_state`; each has a non-empty input schema.
  - `CallTool("advance_state", {session_id, to_state})` → fake advancer sees the
    args, `actor == "ai-service"`; result carries `{state,state_label}`.
  - `CallTool("record_turn", …)` → fake recorder sees `{question,answer}`, result
    `{recorded:true, turn_index}`.
  - `CallTool("get_interview_plan", …)` → fake reader returns fields + 3 questions.
  - unknown session (`OrgOf` → `ErrNotFound`) → tool call `IsError`, text
    "session not found".
- **`mcp/auth`** — no header → 401; `Bearer wrong` → 401; `Bearer <token>` → 200
  (wraps a stub `http.HandlerFunc`).
- **`config`** — `MCP` validation: enabled + empty token → error; prod + short
  token → error; disabled + empty token → ok.

Integration (`//go:build integration`, `-p 1`):
- `ResponseRepo.Record` — first call → `turn_index == 1`; second → `2`; explicit
  `turn_index: 1` again → `ErrConflict`.
- `InterviewRepo.OrgOf` — returns the org; unknown id → `ErrNotFound`.
- `InterviewRepo.PlanData` — returns the joined job title / candidate name for a
  scheduled session.

## 13. Out of scope (later slices)

- Dynamic, JD/resume-informed question generation (`get_interview_plan` real impl) — Phase 2.
- `retrieve_context` real implementation: pgvector embedding columns, embedding
  pipeline, similarity search.
- Python-side `MCPClient` wiring into the Pipecat pipeline (an ai-service slice).
- Per-org MCP tokens, mTLS, request signing.
- MCP resources / prompts / sampling.
- Rate limiting the MCP endpoint.

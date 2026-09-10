# Backend-Go — Slice 5 Design (Candidate Join + Invite Email)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** slices 1–4 (`c9618ee`, `d152dcf`, `479b069`, `f295dbd`)
**Scope:** The candidate side of scheduling — a join token, a public status
endpoint the candidate landing page polls, and an invite email (log transport
for dev). No call/WebRTC, no candidate `users` / password.

## 1. Goal

After a recruiter schedules an interview, the candidate must receive a URL and
the scheduled time and be able to open a page that tells them whether the
interview has started, is open, or they are late. The URL carries an opaque
token — no login for the candidate in v1.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | `interview_sessions.join_token` — URL-safe random, minted at schedule time | The token is the candidate's only credential; no candidate account in v1 |
| D2 | `GET /api/v1/join/{token}` is public (outside the JWT group) | The candidate has no JWT |
| D3 | `phase` computed server-side from the DB clock | The server's clock is authoritative; the browser's isn't |
| D4 | `open` window = `[scheduled_at, scheduled_at + 15m]`; past that is `late` | A candidate a few minutes early sees `before`; the grace covers normal lateness before it's flagged |
| D5 | Email via a `notify.Sender` interface; `log` transport is the dev default, `smtp` is opt-in | `docs/CLAUDE.md` dev-mode: prefer free/OSS, defer paid tiers; a real inbox isn't needed to build the flow |
| D6 | Invite send failure is logged, not fatal to scheduling | The interview exists regardless; the recruiter can resend / copy the link |
| D7 | `join_token` and a derived `join_url` are returned on schedule + `GET /interviews/{id}` | The recruiter UI shows/copies the link and confirms the email target |

## 3. Config (`config/config.go`)

```yaml
app:
  base_url: "http://localhost:5173"   # frontend origin, for building join URLs

notify:
  transport: "log"        # log | smtp
  from: "interviews@oriva.dev"
  smtp:
    host: ""
    port: 587
    username: ""
    password: ""
```
```go
type App struct { BaseURL string `koanf:"base_url"` }   // new top-level "app" block
type Notify struct {
    Transport string     `koanf:"transport"`   // "log" | "smtp"
    From      string     `koanf:"from"`
    SMTP      SMTPConfig  `koanf:"smtp"`
}
type SMTPConfig struct { Host string; Port int; Username string; Password string }
```
`Validate()`: `app.base_url` non-empty and parseable; `notify.transport` in
{`log`,`smtp`}; when `smtp`, `notify.smtp.host` non-empty and `notify.from` non-empty.
(The existing `Application` string field stays; the new `App` struct is `app:` in
YAML — rename the field to `AppName`/keep `application` key to avoid a clash, or
nest. Chosen: keep `application` as-is, add `app: { base_url }` as a sibling —
call the struct `WebApp` to avoid confusion.)

## 4. Migration 0005 — `join_token`

```sql
-- up
ALTER TABLE interview_sessions ADD COLUMN join_token text;
UPDATE interview_sessions SET join_token = encode(gen_random_bytes(18), 'base64')
  WHERE join_token IS NULL;                       -- no rows in dev, harmless
ALTER TABLE interview_sessions ALTER COLUMN join_token SET NOT NULL;
ALTER TABLE interview_sessions ADD CONSTRAINT interview_sessions_join_token_key UNIQUE (join_token);
```
```sql
-- down
ALTER TABLE interview_sessions DROP COLUMN IF EXISTS join_token;
```
Application-generated tokens use `base64.RawURLEncoding` of 18 random bytes
(24 chars, URL-safe); the migration's `base64` fallback is only for pre-existing
rows (none in practice).

## 5. Repository (`db/postgres/interview.go`)

- `Schedule` gains a `joinToken string` parameter and inserts it; still returns the id.
- `detailSelect` + `interview.Detail` gain `join_token`.
- New `JoinInfo`:
  ```go
  type JoinInfo struct {
      JobTitle    string
      ScheduledAt time.Time
      State       string
      IsTerminal  bool     // from session_states.is_terminal
  }
  func (r *InterviewRepo) JoinByToken(ctx context.Context, token string) (*JoinInfo, error) // ErrNotFound
  ```
  `SELECT j.title, s.scheduled_at, s.state, ss.is_terminal
   FROM interview_sessions s JOIN jobs j ON j.id=s.job_id
   JOIN session_states ss ON ss.name=s.state
   WHERE s.join_token = $1`.

## 6. `notify` package (`notify/`)

```go
type Email struct { To, Subject, Text string }

type Sender interface {
    Send(ctx context.Context, e Email) error
}

func NewSender(cfg config.Notify, logger *zap.Logger) (Sender, error)  // dispatches on cfg.Transport

// logSender: logs the rendered email at info level (dev default).
// smtpSender: net/smtp PLAIN auth; STARTTLS on :587.

// InviteEmail renders the candidate invite.
func InviteEmail(from, to, candidateName, jobTitle, joinURL string, scheduledAt time.Time) Email
```
`InviteEmail` body (plain text): greeting, "You've been invited to an interview
for **<job title>**", the scheduled time formatted with timezone, the join URL,
and "the interview page will let you in at the scheduled time."

## 7. Scheduling wiring (`services/interviews`)

`Service` gains `notifier notify.Sender`, `baseURL string`, and a `candLookup`
(email + name for the candidate — extend the existing `candChecker` to a
`candReader` with `Get`). `Schedule`:
1. validate (unchanged) + org-ownership checks (unchanged)
2. `token := jointoken.New()` (24-char RawURLEncoding); `repo.Schedule(..., token)`
3. `d := repo.Get(...)`; attach `d.JoinURL = baseURL + "/join/" + token`
4. best-effort: `c := candRepo.Get(...)`; `notifier.Send(ctx, notify.InviteEmail(from, c.Email, c.Name, d.Job.Title, d.JoinURL, at))`; on error `logger.Warn`, continue
5. return `d`

`interview.Detail` gains `JoinToken string` and `JoinURL string` (json
`join_token`, `join_url`); `Get`/`List` populate `JoinURL` from the configured
base URL (pass base URL into the interviews service; `List` items can omit
`join_url` to keep the query simple — decision: include it, it's a cheap concat).

## 8. Join service + handler

### `services/join` (new)
```go
type repo interface {
    JoinByToken(ctx context.Context, token string) (*postgres.JoinInfo, error)
}
type Service struct { repo repo; grace time.Duration; now func() time.Time }

type Status struct {
    JobTitle      string    `json:"job_title"`
    ScheduledAt   time.Time `json:"scheduled_at"`
    ServerNow     time.Time `json:"server_now"`
    Phase         string    `json:"phase"`             // before | open | late | closed
    LateBySeconds int       `json:"late_by_seconds"`
}
func (s *Service) Status(ctx context.Context, token string) (*Status, error)
```
Phase logic:
- `info.IsTerminal` → `closed`
- `now < scheduled_at` → `before`
- `scheduled_at ≤ now ≤ scheduled_at + grace` (grace = 15m) → `open`
- else → `late`, `LateBySeconds = int(now.Sub(scheduled_at).Seconds())`
`ErrNotFound` → `apxerrors.E(NotFound, "invalid or expired interview link")`.

### `http/handlers/join.go`
```go
func (h *Join) Status(w, r) (any, int, error)  // token from chi.URLParam
```

### `http/server.go`
`GET /api/v1/join/{token}` registered **before/outside** the `RequireAuth` group,
alongside `/health` and `/metrics`.

## 9. Wiring (`cmd/server/main.go`)

```go
notifier, err := notify.NewSender(cfg.Notify, logger)          // fatal on bad config
joinSvc := join.NewService(postgres.NewInterviewRepo(pool))
interviewsSvc := interviews.NewService(interviewRepo, jobRepo, candRepo, notifier, cfg.WebApp.BaseURL)
hs.Join = handlers.NewJoinHandler(joinSvc)
```

## 10. Testing

Unit (`go test ./...`):
- **`jointoken`** — `New()` returns 24 URL-safe chars, distinct across calls.
- **`notify`** — `InviteEmail` contains the job title, the join URL, and the
  formatted time; `logSender.Send` never errors; `NewSender` rejects
  `transport: "smtp"` with an empty host.
- **`services/join`** — table over `(scheduledAt offset, isTerminal) -> phase`:
  terminal → `closed`; now-1m → `before`; now → `open`; now+10m (within grace) →
  `open`; now+20m → `late` with `late_by_seconds ≈ 1200`; unknown token →
  `NotFound`.
- **`services/interviews`** — `Schedule` calls `notifier.Send` with the
  candidate's email and a URL containing the token; a `notifier` that errors does
  **not** fail `Schedule` (result still returned).
- **`http/handlers/join`** — 200 body shape via `httptest` + fake service; unknown
  token → 404.

Integration (`//go:build integration`, `-p 1`):
- migrate 0005 present; `Schedule` stores a `join_token`; `JoinByToken` returns
  the job title + scheduled_at; unknown token → `ErrNotFound`.
- `join_token` is unique (two schedules → different tokens).

## 11. Out of scope

- Candidate `users` rows / password / OAuth.
- Rate-limiting the public `/join` endpoint.
- Token expiry / rotation / single-use.
- Real SMTP delivery testing (the `smtp` transport is built but not exercised in CI).
- Resend-invite endpoint (recruiter copies the URL from the detail page for now).
- The call experience the "Start interview" button eventually opens.

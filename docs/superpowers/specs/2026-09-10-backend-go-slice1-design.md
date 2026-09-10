# Backend-Go — Slice 1 Design (Skeleton + Config + Postgres + Migrations + Health/Metrics + JWT Auth)

**Date:** 2026-09-10
**Status:** Approved for planning
**Scope:** Phase 0, slice 1 of the Go backend (`docs/PLAN.md`). Foundations only — no CRUD
handlers, no session state machine code paths, no MCP server. Those are later slices.

## 1. Goal

Stand up a Go HTTP service (`oriva/backend-go`) that boots cleanly, connects to Postgres,
applies its schema via migrations, serves health + metrics, and authenticates users with
JWT (roles `scheduler` and `candidate`). This is the parallel-safe foundation that does not
depend on the STT/LLM/TTS bake-off (`docs/CLAUDE.md` "What's safe to build now").

The service keeps the *shape* of `~/Practice/Golang/go-service-template-v2` (package layout,
consumer-defined interfaces, `(any, int, error)` handler wrapper, zap-logfmt logging, chi
router, Prometheus middleware, typed `errors` package) but swaps persistence to Postgres and
drops MongoDB, Redis, and the Apxor-specific slack/apxmetrics/apxcache packages.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | New `backend-go/` at repo root; module `oriva/backend-go`, local-only; Go 1.23 | `docs/CLAUDE.md` repo structure; no remote yet |
| D2 | Flat top-level packages mirroring the template (no `internal/`) | Match the reference template |
| D3 | `pgx`/`pgxpool` for Postgres; `golang-migrate` as a **library** with embedded `.sql` files | User directive; no external `migrate` binary required |
| D4 | Drop Mongo, Redis-sentinel, Apxor slack/apxmetrics/apxcache | User directive; not needed for slice 1 |
| D5 | `koanf` config, embedded default YAML + `dev.yaml` override via `APX_CONFIG_FILE` | Keep template pattern |
| D6 | Routes namespaced `/api/v1/...` | User directive (template uses `/service/v1`) |
| D7 | Health check pings the DB pool | User directive |
| D8 | First migration seeds the draft schema + the 12 session states + transition graph | User directive; `docs/ARCHITECTURE.md` §4 |
| D9 | JWT HS256, typed claims, `golang-jwt/jwt/v5`; access token only, no refresh tokens | YAGNI until the frontend needs session persistence |
| D10 | No signup endpoint; users provisioned by `cmd/seed` | Self-serve org onboarding is a non-goal (`docs/PROJECT_SCOPE.md`) |
| D11 | Plain `bcrypt` for passwords (drop template's MD5+pepper wrapper) | Legacy wrapper has no callers here |
| D12 | Server auto-runs migrations at boot only when `postgres.auto_migrate: true` | Safe default off; convenient for dev |

## 3. Directory layout

```
backend-go/
  go.mod  go.sum
  Makefile                     # run, build, test, test-integration, db-up, db-down,
                               # migrate-up, migrate-down, migrate-create name=..., seed
  docker-compose.yml           # pgvector/pgvector:pg16, port 5432, named volume, healthcheck
  .air.toml  .gitignore
  config/
    config.go                  # Config struct, embedded DefaultConfig YAML, Validate()
    dev.yaml                   # local override, loaded via APX_CONFIG_FILE (default dev.yaml)
  cmd/
    server/main.go             # load config -> logger -> InitializeServer -> graceful shutdown
    migrate/main.go            # golang-migrate runner: up | down | version | force <v>
    seed/main.go               # create dev org + one scheduler from env
  db/
    postgres/
      connect.go               # pgxpool.New(cfg) + initial Ping
      user.go                  # UserRepo (pgx impl of services/auth userRepo)
      migrate.go               # embed migrations/*.sql; RunUp(dsn) using golang-migrate lib
  migrations/
    0001_init.up.sql            0001_init.down.sql
    0002_session_states.up.sql  0002_session_states.down.sql
    0003_users.up.sql           0003_users.down.sql
  http/
    server.go                  # chi router, middleware chain, routes, ToHTTPHandlerFunc, Listen
    handlers/
      health.go                # health handler
      auth.go                  # login, me
    middlewares/
      logger.go                # zap-logfmt request log + Prometheus histogram/counter
      auth.go                  # RequireAuth, RequireRole
      cors.go                  # permissive dev CORS (rs/cors)
    response/
      response.go              # RespondJSON, RespondError, RespondMessage
  services/
    health/health.go           # Service{db pinger}; Health(ctx) bool
    auth/auth.go               # Service{repo userRepo, jwt *apxjwt.JWT, cost int}; Login()
  models/
    user/user.go               # User struct (shared: repo <-> service <-> handler)
  errors/
    errors.go  common.go  validation.go   # kept from template (verbatim shape)
  utils/
    jwt/jwt.go                 # Issue(claims) / Parse(token) -> *Claims; typed claims
    helpers/password.go        # HashPassword(plain,cost) / VerifyPassword(plain,hash)
    buildinfo/buildinfo.go     # Version/Commit/Date via -ldflags
```

## 4. Configuration

`config/config.go` — embedded default YAML, overridden by `config/<APX_CONFIG_FILE>`
(default `dev.yaml`). `Validate()` returns the template's accumulated `ValidationErrs`.

```yaml
application: "oriva-backend"
listen: ":8080"
is_prod_mode: false
logger:
  level: "debug"
postgres:
  dsn: "postgres://oriva:oriva@localhost:5432/oriva?sslmode=disable"
  max_conns: 10
  min_conns: 2
  max_conn_lifetime: "1h"
  auto_migrate: false
auth:
  jwt_secret: "dev-insecure-secret-change-me"
  access_ttl: "1h"
  bcrypt_cost: 12
```

Config structs:

```go
type Config struct {
    Application string   `koanf:"application"`
    Listen      string   `koanf:"listen"`
    IsProdMode  bool     `koanf:"is_prod_mode"`
    Logger      Logger   `koanf:"logger"`
    Postgres    Postgres `koanf:"postgres"`
    Auth        Auth     `koanf:"auth"`
}
type Logger   struct { Level string `koanf:"level"`; HostName string }
type Postgres struct {
    DSN             string `koanf:"dsn"`
    MaxConns        int32  `koanf:"max_conns"`
    MinConns        int32  `koanf:"min_conns"`
    MaxConnLifetime string `koanf:"max_conn_lifetime"`
    MaxConnLifetimeDur time.Duration
    AutoMigrate     bool   `koanf:"auto_migrate"`
}
type Auth struct {
    JWTSecret  string `koanf:"jwt_secret"`
    AccessTTL  string `koanf:"access_ttl"`
    AccessTTLDur time.Duration
    BcryptCost int    `koanf:"bcrypt_cost"`
}
```

`Validate()` rules:
- `listen` non-empty; `logger.level` non-empty and a valid zap level.
- `postgres.dsn` non-empty and parseable by `pgxpool.ParseConfig`.
- `max_conn_lifetime` / `auth.access_ttl` parse as `time.Duration` (populate the `*Dur` fields).
- `auth.jwt_secret` non-empty; when `is_prod_mode`, length ≥ 32.
- `auth.bcrypt_cost` in `[10, 14]` (default 12 if zero).
- Sets `logger.host_name` from `os.Hostname()`.

## 5. Database

### 5.1 Connection
`db/postgres/connect.go`: build `*pgxpool.Pool` from `pgxpool.ParseConfig(dsn)` + apply
`MaxConns` / `MinConns` / `MaxConnLifetime`; `pool.Ping(ctx)` once at startup, return error on
failure. `pgxpool.Pool` satisfies the health service's `pinger` interface via a thin adapter
(`Ping(ctx) error`).

### 5.2 Migrations
`db/postgres/migrate.go`: `//go:embed ../../migrations/*.sql` → `iofs` source →
`golang-migrate`. Exposes `RunUp(dsn) error`, `RunDown(dsn) error`, `Version(dsn)`.
- `cmd/migrate` is the CLI entry (`up`, `down`, `version`, `force <v>`).
- Server calls `RunUp` during `InitializeServer` only if `postgres.auto_migrate` is true.

### 5.3 Migration 0001 — `init` (draft schema)
All tables: `id uuid primary key default gen_random_uuid()`,
`created_at timestamptz not null default now()`,
`updated_at timestamptz not null default now()`.

- `CREATE EXTENSION IF NOT EXISTS vector;`
- `organizations(name text not null)`
- `jobs(org_id uuid not null references organizations, title text not null, description text not null default '')`
- `candidates(org_id uuid not null references organizations, email citext not null, name text not null default '', resume_text text not null default '', unique(org_id, email))`
  *(note: `citext` extension is created in 0003; reorder so 0001 creates it, or keep
  `candidates.email` as `text` in 0001 and tighten later — planner picks. Recommended: move
  `CREATE EXTENSION citext` into 0001.)*
- `interview_sessions(job_id uuid not null references jobs, candidate_id uuid not null references candidates, state text not null references session_states(name), scheduled_at timestamptz)`
- `responses(session_id uuid not null references interview_sessions, turn_index int not null, question text not null, answer text not null default '', unique(session_id, turn_index))`
- `scores(session_id uuid not null references interview_sessions, rubric_item text not null, value numeric, rationale text not null default '')`

`interview_sessions.state` FKs `session_states`, so **0002 must create + seed `session_states`
before 0001's `interview_sessions`** — therefore the state tables move into 0001, or
`interview_sessions` moves into a migration after 0002. Recommended: create **all** tables
including `session_states` / `session_state_transitions` in 0001, and seed the state rows in
0002. (Planner: keep DDL and seed data in separate migrations but respect FK ordering.)

### 5.4 Migration 0002 — `session_states` (seed data)
Tables (created in 0001):
- `session_states(name text primary key, label text not null, is_terminal boolean not null default false)`
- `session_state_transitions(from_state text not null references session_states(name), to_state text not null references session_states(name), primary key(from_state, to_state))`

Seed the 12 canonical states (`docs/ARCHITECTURE.md` §4):

| name | label | is_terminal |
|---|---|---|
| scheduled | Scheduled | false |
| invited | Invited | false |
| ready | Ready | false |
| dispatched | Dispatched | false |
| in_progress | In progress | false |
| completed | Completed | false |
| scoring | Scoring | false |
| scored | Scored | **true** |
| declined | Declined | **true** |
| abandoned | Abandoned | **true** |
| interrupted | Interrupted | false |
| failed | Failed | **true** |

Seed transitions:
- Happy path: `scheduled→invited`, `invited→ready`, `ready→dispatched`, `dispatched→in_progress`,
  `in_progress→completed`, `completed→scoring`, `scoring→scored`.
- `ready→declined`
- `invited→abandoned`, `ready→abandoned`
- `dispatched→interrupted`, `in_progress→interrupted`
- `interrupted→dispatched` (the one rejoin edge)
- `failed` from every non-terminal active state:
  `scheduled→failed`, `invited→failed`, `ready→failed`, `dispatched→failed`,
  `in_progress→failed`, `completed→failed`, `scoring→failed`, `interrupted→failed`.
- No rows with `from_state` in {`scored`,`declined`,`abandoned`,`failed`}.

Slice 1 does **not** load these into memory or enforce them in code — that is the session
state machine slice. 0002 only establishes the data.

### 5.5 Migration 0003 — `users`
- `CREATE EXTENSION IF NOT EXISTS citext;` (if not already in 0001)
- `users(id uuid pk default gen_random_uuid(), org_id uuid not null references organizations,
  email citext not null unique, password_hash text not null,
  role text not null check (role in ('scheduler','candidate')),
  created_at/updated_at timestamptz not null default now())`
- `create index on users(org_id)`

No password data in migrations — seeding is `cmd/seed`.

## 6. HTTP layer

### 6.1 Router (`http/server.go`)
chi v5. Middleware chain (order matches the template):
`middleware.RequestID` → `middleware.RealIP` → `middlewares.LoggerWithMetrics` →
`middleware.Recoverer`.

Routes:
```
GET  /api/v1/health                 -> Server.HealthCheckHandler          (public)
GET  /api/v1/metrics                 -> promhttp.Handler                   (public)
POST /api/v1/auth/login              -> ToHTTPHandlerFunc(auth.Login)      (public)
GET  /api/v1/auth/me                 -> RequireAuth( ToHTTPHandlerFunc(auth.Me) )
```

Keep the template's `ToHTTPHandlerFunc(func(w,r)(any,int,error)) http.HandlerFunc` wrapper:
`*errors.Error` → `RespondError` (structured JSON, mapped status); any other error → logged,
`500 "internal error"`; non-nil response → `RespondJSON(status, body)`.

`Listen(ctx, addr)`: start `http.Server` in a goroutine; on `ctx.Done()` (SIGINT/SIGTERM
via `signal.NotifyContext` in `main`) call `server.Shutdown` with a 5s timeout.

### 6.2 Logging + metrics middleware (`http/middlewares/logger.go`)
Adapted from the template's `mlogger`. Per request: log `method`, `path`, `route pattern`,
`status`, `bytes`, `duration_ms`, `request_id`. Record with `promauto`:
- `http_request_duration_seconds` histogram, labels `method`, `route`, `status`.
- `http_requests_total` counter, same labels.
Route label uses `chi.RouteContext(r.Context()).RoutePattern()` to avoid cardinality blowup.

### 6.3 Response package (`http/response/response.go`)
`RespondJSON(w, status, v any)`, `RespondError(w, *errors.Error)` →
`{"error":{"kind":..., "message":..., "fields":{...}}}`, `RespondMessage(w, status, msg)` →
`{"message":msg}`. Kept from the template, trimmed to what's used.

### 6.4 Health (`services/health/health.go` + `http/handlers/health.go`)
```go
type pinger interface { Ping(ctx context.Context) error }
type Service struct { db pinger; logger *zap.Logger }
func NewService(logger *zap.Logger, db pinger) *Service
func (s *Service) Health(ctx context.Context) bool   // 1s timeout, s.db.Ping
```
Handler: `503 {"message":"health check failed"}` when `Health` is false, else
`200 {"message":"ok"}`.

## 7. Authentication

### 7.1 Tokens (`utils/jwt/jwt.go`)
```go
type Claims struct {
    jwt.RegisteredClaims                 // Subject = user id, IssuedAt, ExpiresAt
    OrgID string `json:"org_id"`
    Role  string `json:"role"`
}
type JWT struct { secret []byte; ttl time.Duration }
func New(secret string, ttl time.Duration) *JWT
func (j *JWT) Issue(userID, orgID, role string) (token string, expiresIn int, err error)  // HS256
func (j *JWT) Parse(token string) (*Claims, error)   // rejects non-HMAC alg, expired, bad sig
```

### 7.2 Passwords (`utils/helpers/password.go`)
```go
func HashPassword(plain string, cost int) (string, error)   // bcrypt.GenerateFromPassword
func VerifyPassword(plain, hash string) bool                // bcrypt.CompareHashAndPassword == nil
```

### 7.3 Service (`services/auth/auth.go`)
```go
type userRepo interface {
    FindByEmail(ctx context.Context, email string) (*user.User, error)   // ErrNotFound sentinel
}
type Service struct { repo userRepo; jwt *apxjwt.JWT }
func NewService(repo userRepo, j *apxjwt.JWT) *Service

// Login returns errors.Error{Kind: Unauthorized, "invalid credentials"} for
// unknown user OR bad password (no user-enumeration).
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error)

type LoginResult struct {
    AccessToken string
    TokenType   string   // "Bearer"
    ExpiresIn   int       // seconds
    User        user.User // id, email, role, org_id (no hash)
}
```

### 7.4 Repo (`db/postgres/user.go`)
`UserRepo{pool *pgxpool.Pool}` implementing `FindByEmail` (and `Create` used by `cmd/seed`).
`FindByEmail` returns a package-level `ErrNotFound` when `pgx.ErrNoRows`.

### 7.5 Middleware (`http/middlewares/auth.go`)
```go
func RequireAuth(j *apxjwt.JWT) func(http.Handler) http.Handler
// reads "Authorization: Bearer <t>", j.Parse, on failure -> RespondError 401;
// on success -> context.WithValue(ctx, claimsKey, *Claims)

func RequireRole(roles ...string) func(http.Handler) http.Handler
// reads claims from ctx; 403 if Role not in roles

// in auth package (avoid import cycle): FromContext(ctx) (*Claims, bool)
```
Context key lives in a small shared package (`auth` model or `utils/authctx`) that both the
middleware and handlers import, so there is no `middlewares` ↔ `handlers` cycle.

### 7.6 Handlers (`http/handlers/auth.go`)
- `Login(w,r) (any,int,error)` — decode `{email,password}`, validate non-empty (→
  `errors.ValidationErrs`), call `svc.Login`, return `LoginResult` + 200.
- `Me(w,r) (any,int,error)` — `authctx.FromContext`, return `{id,email,role,org_id}` + 200.

### 7.7 Seed (`cmd/seed/main.go`)
Reads `POSTGRES_DSN` (or config), `SEED_ORG_NAME` (default "Dev Org"),
`SEED_SCHEDULER_EMAIL`, `SEED_SCHEDULER_PASSWORD`. Upserts the org, upserts the scheduler
user with a bcrypt hash. Idempotent. Invoked by `make seed`.

## 8. Wiring (`cmd/server/main.go` + `InitializeServer`)

```
load config (koanf) -> Config.Validate()
build zap logger (logfmt, stdout, level, initial fields host+service+version)
signal.NotifyContext(SIGINT, SIGTERM)
InitializeServer(ctx, cfg, logger):
    pool := postgres.Connect(ctx, cfg.Postgres)          // pings
    if cfg.Postgres.AutoMigrate { postgres.RunUp(dsn) }
    userRepo  := postgres.NewUserRepo(pool)
    jwt       := apxjwt.New(cfg.Auth.JWTSecret, cfg.Auth.AccessTTLDur)
    healthSvc := health.NewService(logger, pingAdapter{pool})
    authSvc   := auth.NewService(userRepo, jwt)
    authH     := handlers.NewAuthHandler(authSvc)
    server    := http.NewServer(logger, healthSvc, authH, jwt)
server.Listen(ctx, cfg.Listen)
```

## 9. Build info

`utils/buildinfo`: `var Version, Commit, Date = "dev","none","unknown"` set via
`-ldflags "-X 'oriva/backend-go/utils/buildinfo.Version=...'"` in the Makefile `build`
target. Logged once at boot; **not** exposed on `/health` in this slice.

## 10. Testing

Unit (`go test ./...`, `testify`, table-driven):
- `config` — `Validate()`: empty listen, invalid logger level, empty/invalid DSN, missing
  jwt_secret, short secret under prod mode, bad bcrypt cost, happy path; `*Dur` fields
  populated.
- `utils/jwt` — issue→parse round trip; expired token rejected; wrong secret rejected;
  `alg=none` / RS256 header rejected.
- `utils/helpers` — hash then verify true; wrong password verify false.
- `services/health` — fake `pinger` nil → true; error → false; respects timeout.
- `services/auth` — fake `userRepo`: success returns token + sanitized user; unknown user →
  `Unauthorized`; bad password → `Unauthorized`; identical error value/message for the last
  two.
- `http/middlewares/auth` — no header → 401; malformed token → 401; valid → 200 and claims
  present in ctx; `RequireRole` mismatch → 403.
- `http/handlers` — health 200/503 with fake health service; auth login 200 + body shape,
  400 on missing fields, 401 on bad creds, all via `httptest` with a fake auth service.

Integration (`//go:build integration`, `make test-integration`, needs
`TEST_DATABASE_URL` + `make db-up`):
- Apply `up` → assert `session_states` has exactly 12 rows and `is_terminal` set on
  {scored, declined, abandoned, failed}; assert transition rows include `interrupted→dispatched`
  and exclude any `from_state` in the terminal set.
- `down` → `up` again cleanly (idempotency).
- `UserRepo`: seed a user, `FindByEmail` returns it, unknown email → `ErrNotFound`.

`make test` = unit only. CI wiring is out of scope for this slice.

## 11. Out of scope (explicitly deferred to later slices)

- Candidate/recruiter/job CRUD handlers and services.
- Interview scheduling endpoints and the candidate invite-token flow.
- Session state machine: in-memory load of `session_states` / `session_state_transitions`,
  `advance_state` validation, transition enforcement.
- MCP server skeleton (`get_interview_plan`, `retrieve_context`, `record_turn`,
  `advance_state`) over Streamable HTTP.
- Refresh tokens / token revocation / logout.
- pgvector embedding columns and the Python read-only retrieval path.
- Deployment target, Dockerfile for the app image, CI pipeline.
- Rate limiting, production CORS allowlist, request body size limits.

## 12. Open questions (non-blocking; planner may resolve)

- Exact FK ordering across migrations 0001/0002 (see §5.3) — recommended: all DDL in 0001,
  seed data in 0002/0003.
- Whether `cmd/seed` reads the koanf config directly or only env vars — recommended: reuse
  the config loader, allow env override of the two seed-specific values.

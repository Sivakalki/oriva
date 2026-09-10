# oriva/backend-go

Go backend for the AI interview platform. **Slice 1** (foundations): config, Postgres +
migrations, health/metrics, and JWT auth. See
[`../docs/superpowers/specs/2026-09-10-backend-go-slice1-design.md`](../docs/superpowers/specs/2026-09-10-backend-go-slice1-design.md).

CRUD, the session state machine, and the MCP server are later slices.

## Quick start

```bash
make db-up                 # start Postgres (pgvector/pgvector:pg16) on :5432
make migrate-up            # apply migrations
SEED_SCHEDULER_EMAIL=rec@oriva.dev SEED_SCHEDULER_PASSWORD=pass1234 make seed
make run                   # serve on :8080  (config/dev.yaml has auto_migrate: true)
```

```bash
curl localhost:8080/api/v1/health
curl -XPOST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"rec@oriva.dev","password":"pass1234"}'
curl localhost:8080/api/v1/auth/me -H "Authorization: Bearer <token>"
```

## Layout

| Path | Purpose |
|---|---|
| `cmd/server` | HTTP service entrypoint |
| `cmd/migrate` | `up` / `down` / `version` / `force <v>` |
| `cmd/seed` | provision a dev org + one scheduler |
| `config` | koanf config + `Validate()` |
| `db/postgres` | pgxpool, embedded `golang-migrate`, repos |
| `migrations` | `*.sql` (embedded) |
| `http` | chi router, `(any,int,error)` handler wrapper, middleware |
| `services/*` | business logic (consumer-defined interfaces) |
| `errors` | typed `*Error` + validation builder |
| `utils/jwt` | HS256 issue/parse, typed claims |

## Config

`config/dev.yaml` overrides `config.DefaultConfig`. Point elsewhere with
`APX_CONFIG_FILE=<name>` (resolved under `config/`). `POSTGRES_DSN` overrides the DSN for
`cmd/migrate` and `cmd/seed`.

## Tests

```bash
make test                  # unit only
make db-up && docker exec <pg> psql -U oriva -d oriva -c 'CREATE DATABASE oriva_test;'
make test-integration      # needs TEST_DATABASE_URL (defaults to the oriva_test DB)
```

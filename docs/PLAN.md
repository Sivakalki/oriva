# Plan — AI interview platform

This is a working roadmap, not a rigid spec. Update it as decisions get made.

## Status — 2026-09-10

Legend: ✅ done · 🟡 partial · ⬜ not started

Two services exist, built slice by slice; each slice has a design spec in
`docs/superpowers/specs/` and a matching implementation commit.

- **`backend-go/`** (module `oriva/backend-go`, Go 1.25, pgx + Postgres):
  slices 1–5 done — config/logging/health/metrics, migrations, JWT auth,
  recruiter CRUD + scheduling, the session state machine, the MCP server, and
  the candidate join token + invite email.
- **`ai-service-python/`** (package `oriva_ai`, uv, Python 3.12, Pipecat 1.8):
  slices 1–4 done — scaffold/config/logging, the pluggable STT→LLM→TTS pipeline
  skeleton with a mock provider per stage, the MCP client, and the bake-off
  harness + Prometheus/Grafana compose.
- **`frontend/`** (Vite + React 19 + Tailwind v4 + shadcn): slice 1 done —
  recruiter shell (auth, dashboard, jobs, candidates, schedule, interview detail)
  and the public candidate landing page.

**What's left before Phase 1 can conclude:** install real STT/LLM/TTS providers
and record the real clip set, then run the matrix and pick a combo. Phase 2/3
are not started.

## Phase 0 — Foundations (can start immediately, doesn't depend on AI-stack bake-off)

- ✅ **Go backend: JWT auth** (scheduler/candidate roles, `POST /auth/login`,
  `RequireAuth`/`RequireRole`). *(backend-go slice 1)*
- ✅ **Candidate/recruiter CRUD + interview scheduling** — jobs, candidates,
  `POST /interviews`, all scheduler-only and org-scoped. *(backend-go slice 2)*
- ✅ **Session state machine** backed by `session_states` /
  `session_state_transitions` (docs/ARCHITECTURE.md §4): the 12-state graph is
  loaded at startup into an immutable validator; transitions go through
  `POST /interviews/{id}/advance` with optimistic CAS + a `session_state_events`
  audit log; `GET /session-states` exposes the graph for React. *(backend-go slice 3)*
- ✅ **React frontend shell** (`frontend/`, Vite + React + Tailwind v4 + shadcn):
  recruiter auth, interviews dashboard, jobs/candidates lists + create, schedule
  flow, interview detail with the state-advance control. Plus the **public
  candidate landing page** (`/join/:token`) that shows a countdown before the
  interview, "start" when open, and "you're N minutes late" after — backed by
  `GET /api/v1/join/{token}` and the invite email sent on scheduling (backend
  slice 5). Call UI is deferred. *(frontend slice 1, backend-go slice 5)*
- 🟡 **Postgres schema:** organizations → jobs → candidates → interview_sessions
  → responses → scores, plus the state tables — all created (migrations
  0001–0004). Still draft; `responses`/`scores` will be refined as scoring lands.
- ✅ **Go's MCP server skeleton:** all four tools exposed over Streamable HTTP at
  `/mcp` (static bearer token), schema version `oriva.tools.v1`. `advance_state`
  and `record_turn` are real; `get_interview_plan` is a real read with canned
  opener questions; `retrieve_context` is a stub. Org resolved from `session_id`.
  *(backend-go slice 4)*

## Phase 1 — Voice pipeline bake-off (highest-leverage, do this before building real
interview logic)

Goal: pick a concrete STT/LLM/TTS combination empirically, not from research alone, before
committing to it.

1. ✅ **Build a pluggable pipeline skeleton.** Pipecat 1.8, STT/LLM/TTS chosen
   per stage in `config.yaml`; a `mock` provider per stage is the dev default
   (no keys/models), real `whisper` / `openai`(=LiteLLM) / `piper` factories are
   wired but lazily imported. Local `/ws` WebSocket transport — no telephony.
   *(ai-service slice 2)*
2. ✅ **Instrument every stage boundary.** `telemetry/observer.py` timestamps
   each handoff (user-stop → transcript → LLM first token → TTS first audio →
   bot-speaking) into Prometheus histograms labelled by provider/model.
   `docker-compose.yml` + `ops/` stand up Prometheus + Grafana + Pushgateway,
   with a provisioned pipeline dashboard. *(ai-service slices 2 & 4)*
3. 🟡 **Build a fixed test set.** The harness, `bakeoff/clips/manifest.yaml`
   (reference transcript + tags per clip), and the replay machinery exist; the
   committed clips are 4 synthetic tone-sweeps. **TODO:** record 10–15 real
   candidate-answer clips (pauses, fillers, noise, accents, short/long) and
   replace them — no code change, just files + manifest rows. *(ai-service slice 4)*
4. ⬜ **Run the STT bake-off first.** Blocked on step 3's real clips + installing
   `pipecat-ai[whisper]`. The matrix runner (`python -m oriva_ai.bakeoff`) scores
   latency (p50/p95) and word error rate per combo and writes a CSV/JSON report;
   it just needs real providers and clips fed in.
5. ⬜ **Run the LLM × TTS matrix against the winning STT.** Same runner. LLM
   config already points at a LiteLLM `base_url`; running the proxy is pending.
   Still need to filter LLM candidates to ones with solid tool-calling first.
6. ⬜ **Pick a finalist, then validate over real telephony.** Swap the transport
   to Twilio and re-run. Not started.

Use free/open-source options throughout this phase (self-hosted Whisper, Piper/Kokoro TTS,
local WebRTC transport, self-hosted LiteLLM, self-hosted Prometheus/Grafana) — save Twilio
and paid provider tiers for the final validation step.

## Between Phase 1 and Phase 2 — MCP boundary (done early, in parallel)

Not a numbered Phase-1 step, but built alongside it because the contract is
testable independently:

- ✅ **Go MCP server** — see Phase 0.
- ✅ **Python MCP client** — each `/ws` connection opens a Pipecat `MCPClient`
  to the Go server and loads the four tools into the LLM context. `session_id`
  (the `/ws` query param) is bound to every tool call and hidden from the LLM.
  A live cross-service test connects the two. *(ai-service slice 3)*

## Phase 2 — Interview logic
- ⬜ Dynamic follow-up generation: LLM analyzes the JD, the resume, and the running
  conversation to decide the next question, not a fixed script.
  (Today `get_interview_plan` returns three canned openers.)
- ⬜ Rubric-based scoring: run on the transcript post-call, with a separate (can be
  higher-quality/slower) model than the live conversational one.
- ⬜ Guardrails so the AI only answers candidate questions from approved content
  (prompt-injection resistance for the live conversation).
- ⬜ `retrieve_context` real implementation: pgvector embedding columns, an
  embedding step, similarity search (Go side; Python also holds its own
  read-only path per ARCHITECTURE §2).
- ⬜ Candidate join flow: candidate `users` provisioning + magic-link / invite
  token + the email that carries it.

## Phase 3 — Production hardening
- ⬜ Deployment target (deferred until now on purpose).
- ⬜ Full telephony integration and call-quality monitoring in production.
- ⬜ Bias-audit and compliance review of the scoring pipeline (see docs/PROJECT_SCOPE.md).
- ⬜ Final scoring rubric schema, signed off by stakeholders.
- ⬜ MCP boundary hardening: per-org tokens / mTLS instead of the shared bearer token.

## Open questions / not yet decided
- Final STT/LLM/TTS vendor choice — pending Phase 1 results (steps 4–5).
- Production telephony provider beyond "likely Twilio."
- Final scoring rubric schema.
- Deployment target.
- How `session_id` reaches the `/ws` URL (frontend join flow vs telephony webhook).

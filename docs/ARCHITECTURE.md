# Architecture — AI interview platform

## 1. Voice pipeline: cascaded, not speech-to-speech

**Decision:** STT → LLM → TTS as separate, swappable stages, not an end-to-end
speech-to-speech model.

**Why:** The interview needs an auditable text transcript (for scoring, dispute handling,
and bias-audit compliance), component-level control over each stage, and a tool surface
(dynamic question generation, rubric scoring) — all of which a cascaded pipeline supports
and end-to-end speech-to-speech models currently do not.

**Orchestrator:** Pipecat (Python, open source). Chosen because it supports pluggable
STT/LLM/TTS services, has built-in Twilio/SIP telephony support, and handles barge-in and
interruption out of the box.

**Latency target:** ~800ms median voice-to-voice for the live pipeline (loosen to ~1,500ms
acceptable for an early prototype). Rough budget:

| Stage | Budget |
|---|---|
| Network/transport | 50–150ms |
| Turn detection / endpointing | 150–300ms |
| STT (incremental, at end of turn) | 50–150ms |
| LLM time-to-first-token | 150–400ms |
| TTS time-to-first-audio | 40–150ms |

Stages overlap in practice (STT streams while the candidate talks, TTS starts before the
LLM finishes generating), so the realized total is smaller than the naive sum.

## 2. Service boundaries

Three distinct connection types exist in the system — they are not interchangeable and
should not be confused with each other:

### Frontend ↔ Go backend
Plain REST + JWT. Covers auth, scheduling, candidate/recruiter CRUD, listing interviews.
No LLM or tool concept is involved here — this is ordinary CRUD traffic and is the majority
of the app's requests.

### Go ↔ Python, during a live call only ("the tool boundary")
**Mechanism: MCP over Streamable HTTP** (not gRPC).

Go implements an MCP server and owns the tool schemas (JSON Schema, versioned as
`oriva.tools.v1`): `get_interview_plan`, `record_turn`, `advance_state`. Originally these
were meant to be called by the live LLM mid-conversation; in practice the LLM proved
unreliable at pacing itself live (see docs/PLAN.md), so Python now drives the interview
deterministically (a pre-generated question queue) and calls these tools directly itself
rather than leaving that decision to the LLM. Either way, Go owns the schemas and Python
is the only consumer — the tool boundary itself is unchanged.

No retrieval/RAG tool exists. `get_interview_plan` returns the full job description and
resume text directly; both are short enough to use as-is, so there's no corpus to search
and no embedding/similarity-search layer to build.

Why MCP over gRPC: gRPC would require maintaining two schemas — the `.proto` definition for
the Go↔Python wire format, and a separately hand-written JSON Schema for the LLM's
function-calling interface (which speaks JSON Schema, not protobuf). That's two sources of
truth that can drift. MCP's tool definition serves both purposes at once — one schema,
defined once, consumed by both the wire protocol and the LLM. There is no extra LLM cost to
using MCP over gRPC; the LLM call (deciding to invoke a tool) happens either way. MCP is
just the transport/schema format for the tool underneath it — it does not run its own model
and is not a hosted/paid service in this context (it's a Go program you run yourself).

**Tools live in Go, are consumed by Python.** Python holds no direct Postgres connection
of its own and writes nothing directly — every read and write to interview data goes
through Go, via the tool boundary above.

## 3. LLM, STT, TTS: pluggable, vendor-undecided

None of the three pipeline stages are locked to a specific vendor. This is deliberate —
the actual choice is being made empirically (see docs/PLAN.md's bake-off), not from
research alone.

- **LLM:** routed through a proxy layer (self-hosted LiteLLM for dev/bake-off; OpenRouter as
  a hosted alternative once a model is chosen) so the model is a config value, not a code
  dependency. Any candidate model must have solid function/tool-calling support before it's
  compared on latency — the MCP tool boundary depends on reliable tool calls, and not every
  model behind a router handles this equally well.
- **STT:** candidates include self-hosted Whisper/faster-whisper and cloud providers
  (Deepgram, AssemblyAI) — bake-off first, since STT has an outsized effect on interview
  quality (a fast but inaccurate transcript is worse than a slightly slower accurate one).
- **TTS:** candidates include open-source options (Piper, Kokoro) for dev, and Cartesia/
  ElevenLabs for production-quality voice once a finalist is picked.

## 4. Session state

### Canonical states (12 total, snake_case)

**Happy path:** `scheduled` → `invited` → `ready` → `dispatched` → `in_progress` →
`completed` → `scoring` → `scored`

**Exception states:**
- `declined` — candidate refused consent (from `ready`). Terminal.
- `abandoned` — no-show, or join link lapsed unused (from `invited`/`ready`). Terminal.
- `interrupted` — connection dropped mid-call (from `dispatched`/`in_progress`). The only
  exception state with an exit: it can transition back to `dispatched` so the candidate can
  rejoin the same session.
- `failed` — terminal technical failure, not the candidate's fault, not rejoinable (from any
  active state).

**Load-bearing rule:** no transitions out of `scored`, `declined`, `abandoned`, or `failed`.
`interrupted → dispatched` is the one rejoin edge.

Build the full 12-state set now (the enum itself is cheap); the code paths for `scoring`/
`scored` don't need to be wired up until scoring logic actually exists.

### Source of truth and drift prevention

Session state is modeled as **data in Postgres**, not generated code across three
languages:

- `session_states` and `session_state_transitions` tables hold the canonical list and the
  legal transition graph.
- **Go is the sole authority.** It loads these tables into memory at startup and is the only
  service that validates and applies a transition.
- **Python never validates a transition itself.** It calls `advance_state(...)` via the MCP
  tool boundary and handles success/rejection. It only needs the bare state names for its
  own branching logic (e.g. "if `interrupted`, attempt rejoin"), not the full transition
  graph.
- **React never bakes in the enum.** It reads `{state, label}` off the API response it
  already gets. Display-label changes need no frontend deploy.

This keeps a single source of truth without the codegen/CI-diff machinery a three-language
generator would need. Adding a transition is a migration; adding brand-new behavior on a
new state still requires real code (that part is unavoidable, and unrelated to the
sync-mechanism choice).

## 5. Compliance implications of the above

The cascaded pipeline and per-turn transcript exist specifically so that: (a) scoring
rationale is inspectable per question, not an opaque black-box score, and (b) a full
transcript exists for dispute handling and any required bias audit. This should be treated
as a hard constraint on future changes, not an incidental side effect — do not replace the
transcript-based scoring flow with an audio-only or end-to-end scoring shortcut later
without revisiting this document.

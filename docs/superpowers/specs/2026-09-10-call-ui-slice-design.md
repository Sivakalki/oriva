# Call UI Slice Design (candidate real-time voice)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** backend-go slice 5 (`98e189b`), ai-service slice 3 (`1d0c4c0`),
frontend slice 1 (`e345135`)
**Scope:** The candidate joins their interview and talks to the pipeline in the
browser. Cross-cutting: a small backend-go change, a small ai-service change, and
the frontend call page. No recruiter observe mode, no recording, no real audio
(providers are still `mock`).

## 1. Goal

`docs/PLAN.md` Phase 0: "basic call UI shell." The candidate opens the join
link, and when the interview is open, starts a live voice session against the
ai-service `/ws` Pipecat pipeline: mic in, transcript + (eventually) audio back.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | `/ws?token=<join_token>` — the ai-service resolves `session_id` from Go | The candidate holds a token, not a session id; keeps the id server-resolved |
| D2 | `GET /join/{token}` also returns `session_id` + `ai_ws_url` | The frontend needs the WS URL; the token holder is already authorized for the session |
| D3 | ai-service rejects `phase == "closed"` on connect (close 1008); allows `before`/`open`/`late` | The join page gates `before`; defense in depth blocks a finished interview |
| D4 | `@pipecat-ai/client-js` + `@pipecat-ai/websocket-transport` | Speaks the `ProtobufFrameSerializer` the `/ws` already uses; no WebRTC/TURN needed |
| D5 | Transcript via RTVI events (`onUserTranscript` / `onBotTranscript`) | `PipelineTask(enable_rtvi=True)` is already the default |
| D6 | Candidate-only; recruiter observe is a later slice | Keep the surface small |
| D7 | The page states plainly (in dev) that audio-back is silent until a real TTS is installed | Honest skeleton |

## 3. backend-go changes

### Config (`config.go`)
`WebApp` gains `AIWsURL string` (`app.ai_ws_url`, default `ws://localhost:8090/ws`).
`Validate()`: non-empty, scheme `ws`/`wss`.

### `services/join` — `Status` gains fields
```go
type Status struct {
    JobTitle      string
    ScheduledAt   time.Time
    ServerNow     time.Time
    Phase         string
    LateBySeconds int
    SessionID     string `json:"session_id"`   // NEW
    AIWsURL       string `json:"ai_ws_url"`     // NEW
}
```
`join.NewService(repo, aiWsURL)`; `JoinByToken` (repo) also selects
`interview_sessions.id`. Handler/wiring pass `cfg.WebApp.AIWsURL`.

### `db/postgres/interview.go`
`JoinInfo` gains `SessionID string`; `JoinByToken` selects `s.id`.

## 4. ai-service changes

### Config (`config.py`)
`MCPConfig` unchanged. New top-level:
```python
class BackendConfig(BaseModel):
    base_url: str = "http://localhost:8080"   # backend-go origin
Settings.backend: BackendConfig
```
`config.yaml` gains `backend: { base_url: http://localhost:8080 }`.

### `pipeline/join_lookup.py` (new)
```python
@dataclass
class Resolved:
    session_id: str
    phase: str

async def resolve_token(backend_base_url: str, token: str) -> Resolved
```
`GET {base_url}/api/v1/join/{token}` via `httpx.AsyncClient` (add `httpx` — it is
already a transitive dep, pin it). 404 → raises `TokenNotFound`.

### `transport/websocket.py`
```python
@app.websocket("/ws")
async def ws(websocket):
    token = websocket.query_params.get("token")
    session_id = websocket.query_params.get("session_id")   # test/back-compat
    if token:
        try:
            r = await resolve_token(settings.backend.base_url, token)
        except TokenNotFound:
            await websocket.close(code=1008, reason="invalid interview link"); return
        if r.phase == "closed":
            await websocket.close(code=1008, reason="interview has ended"); return
        session_id = r.session_id
    if settings.mcp.enabled and not session_id:
        await websocket.close(code=1008, reason="session_id or token required"); return
    await websocket.accept()
    build = await build_session_pipeline(settings, session_id or "")
    ...
```

## 5. frontend changes

### Deps
`@pipecat-ai/client-js`, `@pipecat-ai/websocket-transport`.

### Types (`api/types.ts`)
`JoinStatus` gains `session_id: string`, `ai_ws_url: string`.

### Routing (`routes.tsx`)
`/interview/:token` → `Call` (public, standalone, no AppShell).
`pages/Join.tsx`: the two `StartButton`s become `<Link to={/interview/${token}}>`.

### `pages/Call.tsx`
- `useJoinStatus(token)` for `phase` + `ai_ws_url`.
- `phase === "closed"` → "This interview has ended."
- else → a **Lobby** view: job title, "Start interview" button.
- On Start:
  1. `await navigator.mediaDevices.getUserMedia({ audio: true })` — on
     `NotAllowedError` show "Microphone access is required."
  2. build the client:
     ```ts
     const client = new PipecatClient({
       transport: new WebSocketTransport(),
       enableMic: true,
       enableCam: false,
       callbacks: { ... },
     })
     await client.connect({ wsUrl: `${aiWsUrl}?token=${encodeURIComponent(token)}` })
     ```
     (exact `connect` param shape verified against the installed client version at
     build time — 1.13 may use `{ endpoint }` / a connect helper; adapt.)
  3. Live view: a status pill (`onConnected`/`onDisconnected`/transport state), a
     large pulsing indicator toggled by `onUserStartedSpeaking` /
     `onUserStoppedSpeaking` / `onBotStartedSpeaking` / `onBotStoppedSpeaking`, a
     scrolling transcript list appended from `onUserTranscript(final)` and
     `onBotTranscript`, and an **End interview** button → `client.disconnect()`.
  4. On disconnect → a "Thanks, the interview has ended." screen.
- A dev-only note: "You may not hear audio back yet — the voice models aren't
  wired in this environment."
- Clean up the client + mic stream on unmount.

### `components/` additions
`CallStatusPill`, `SpeakingIndicator`, `TranscriptList` — small presentational
pieces so `Call.tsx` stays readable.

## 6. Testing

backend-go (`go test ./...`, `-tags=integration -p 1`):
- `services/join` — `Status` includes `session_id` and the configured `ai_ws_url`;
  phase logic unchanged.
- integration — `JoinByToken` returns the session id; `/join/{token}` JSON has
  both new fields.
- `config` — `app.ai_ws_url` validation (empty, bad scheme).

ai-service (`pytest`):
- `test_join_lookup.py` — `resolve_token` parses the Go response (httpx mocked);
  404 → `TokenNotFound`.
- `test_app.py` — `/ws?token=…` with a mocked `resolve_token`: `closed` phase →
  socket closed 1008; open phase → `build_session_pipeline` called with the
  resolved `session_id`. `/ws` with neither token nor session_id → 1008.

frontend (`vitest`):
- `Call` renders the lobby for `phase: open` (job title + Start button) and the
  ended screen for `phase: closed`.
- With `@pipecat-ai/client-js` mocked: clicking Start calls `getUserMedia`, then
  `client.connect` with a URL containing `?token=`; a mocked `onBotTranscript`
  callback appends a line to the transcript; End calls `client.disconnect()`.
- mic-denied path: `getUserMedia` rejects → the permission message shows.

## 7. Out of scope

- Real audio playback (needs a real TTS provider + model).
- Recruiter observe / listen-in mode.
- Call recording / transcript persistence view.
- Network-drop reconnection UX beyond a status pill.
- The `interrupted → dispatched` rejoin edge in the UI.
- WebRTC transport / TURN / STUN (WebSocket only).
- Auth on `/ws` beyond the join token.

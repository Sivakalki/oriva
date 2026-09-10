# AI Service — Slice 2 Design (Pipecat Pipeline Skeleton)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** ai-service slice 1 (`2026-09-10-ai-service-slice1-design.md`, commit `502aab9`)
**Scope:** PLAN.md Phase 1 items 1–2 plus a stub of item 3 — a config-driven,
provider-pluggable Pipecat STT→LLM→TTS pipeline over a local WebSocket transport,
with every stage boundary instrumented into the slice 1 Prometheus histograms,
and an offline test-clip harness.

## 1. Goal

Make the voice pipeline real enough to run and measure, without committing to any
STT/LLM/TTS vendor and without requiring external services. The dev default is an
all-mock pipeline (`uv run oriva-ai` works with no API keys, no model downloads).
Real providers (`whisper`, OpenAI-compatible incl. LiteLLM, `piper`) are wired but
lazily imported; selecting one whose extra is not installed fails with a clear
message. The bake-off slice adds real extras and the LiteLLM/Prometheus compose.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | `mock` is a first-class provider for every stage, and the committed `config.yaml` default | Skeleton must run and be fully testable with zero external deps |
| D2 | Real providers are lazy-imported factories in a registry | Missing `pipecat-ai[whisper]` etc. must not break `import oriva_ai` |
| D3 | Metrics come from Pipecat's own `MetricsFrame` (TTFB per processor), captured by a `BaseObserver` | Pipecat already computes per-service TTFB; don't reinvent timing |
| D4 | `voice_to_voice` measured observer-side: `UserStoppedSpeakingFrame` → first `BotStartedSpeakingFrame` | Matches ARCHITECTURE.md §1 definition |
| D5 | Local transport = `FastAPIWebsocketTransport` on `@app.websocket("/ws")` | Pipecat-idiomatic, no separate port, no telephony |
| D6 | Silero VAD (`pipecat-ai[silero]`) added now | Turn detection is required for `voice_to_voice` and barge-in later |
| D7 | Pipeline built once at app startup (fail-fast on bad provider config), reused per connection | Config errors surface at boot, not on first call |
| D8 | Harness runs offline via an in-memory transport, not the websocket | Reusable bake-off measuring tool; deterministic in CI |
| D9 | Pipecat 1.8 API: `LLMContext` / `LLMContextFrame` (not `OpenAILLMContext`) | That's what 1.8.1 ships |

## 3. Structure (new, under `src/oriva_ai/`)

```
pipeline/
  __init__.py            # re-exports build_pipeline, PipelineBuild
  providers/
    __init__.py
    base.py              # STTConfig/LLMConfig/TTSConfig -> service; error types
    registry.py          # STT_PROVIDERS / LLM_PROVIDERS / TTS_PROVIDERS: name -> factory
    mock.py              # MockSTTService, MockLLMService, MockTTSService
    real.py              # whisper_stt(), openai_llm(), piper_tts()  (lazy imports)
  context.py             # interview_context(settings) -> LLMContext + greeting
  assembly.py            # build_pipeline(settings) -> PipelineBuild
  runner.py              # PipelineSession(build, transport): run() / stop()
transport/
  __init__.py
  websocket.py           # register_ws_route(app)
telemetry/
  observer.py            # MetricsObserver(BaseObserver)
harness/
  __init__.py
  run_clip.py            # python -m oriva_ai.harness.run_clip <wav> [--config path]
  memory_transport.py    # in-memory transport: WAV -> InputAudioRawFrame; collect OutputAudioRawFrame
  fixtures/
    short_answer.wav     # ~1s, synthetic, mono 16k, checked in (<50KB)
tools/
  gen_fixtures.py        # regenerates harness/fixtures/*.wav (not run in CI)
```

## 4. Config additions (`config.py` + `config.yaml`)

```python
class PipelineConfig(BaseModel):
    greeting: str = "Hi, thanks for joining. Let's begin when you're ready."
    sample_rate: int = 16000
    vad: Literal["silero", "none"] = "silero"
```
`Settings` gains `pipeline: PipelineConfig`. `config.yaml` gains the block and its
`stt.provider` / `llm.provider` / `tts.provider` change to `mock`. Provider-specific
fields (whisper `model`, llm `base_url`/`api_key`/`model`, piper `voice`) stay — a
mock ignores the ones it doesn't need.

`MockConfig` extras on the stt/llm/tts models (optional, defaulted):
- `stt.mock_transcript: str = "I have about five years of backend experience."`
- `llm.mock_reply_template: str = "You said: {user}. Can you tell me more?"`

## 5. Providers

### `base.py`
```python
class ProviderError(RuntimeError): ...
class UnknownProvider(ProviderError): ...        # name not in registry
class ProviderNotInstalled(ProviderError): ...   # lazy import failed; message names the extra

def build_stt(cfg: STTConfig) -> STTService
def build_llm(cfg: LLMConfig) -> LLMService
def build_tts(cfg: TTSConfig) -> TTSService
```
Each looks up `cfg.provider` in the stage registry, calls the factory with `cfg`,
wraps `ImportError`/`ModuleNotFoundError` as `ProviderNotInstalled`.

### `mock.py`
- **`MockSTTService(STTService)`** — override `run_stt(audio)`:
  `self.start_ttfb_metrics()`, `await asyncio.sleep(0)`, `self.stop_ttfb_metrics()`,
  `yield TranscriptionFrame(cfg.mock_transcript, user_id="", timestamp=now_iso())`.
- **`MockLLMService(LLMService)`** — override `process_frame`; on `LLMContextFrame`
  (or `LLMRunFrame`): read the latest user message from the context, then push
  `LLMFullResponseStartFrame`, `start_ttfb_metrics()`, stream the templated reply as
  `LLMTextFrame` per word with `stop_ttfb_metrics()` after the first, then
  `LLMFullResponseEndFrame`. Bracket with `start/stop_processing_metrics()`.
- **`MockTTSService(TTSService)`** — override `run_tts(text, context_id)`:
  `yield TTSStartedFrame()`, `start_ttfb_metrics()`, then N `TTSAudioRawFrame`
  chunks of zero bytes sized `~ len(text)` at `sample_rate` (`stop_ttfb_metrics()`
  after the first), then `TTSStoppedFrame()`.

Exact base-class hooks verified against pipecat 1.8.1 during build; fall back to a
minimal `FrameProcessor` subclass if a base signature differs.

### `real.py` (lazy)
```python
def whisper_stt(cfg):   from pipecat.services.whisper.stt import WhisperSTTService; return WhisperSTTService(model=cfg.model, language=cfg.language)
def openai_llm(cfg):    from pipecat.services.openai.llm import OpenAILLMService;   return OpenAILLMService(model=cfg.model, base_url=cfg.base_url, api_key=cfg.api_key)
def piper_tts(cfg):     from pipecat.services.piper.tts import PiperTTSService;     return PiperTTSService(voice=cfg.voice, ...)   # exact kwargs checked at build
```
`openai_llm` pointed at `base_url` is exactly how Pipecat talks to a LiteLLM proxy.

### `registry.py`
```python
STT_PROVIDERS = {"mock": mock.build_stt, "whisper": real.whisper_stt}
LLM_PROVIDERS = {"mock": mock.build_llm, "openai": real.openai_llm, "litellm": real.openai_llm}
TTS_PROVIDERS = {"mock": mock.build_tts, "piper": real.piper_tts}
```

## 6. Assembly (`assembly.py`)

```python
@dataclass
class PipelineBuild:
    stt: STTService
    llm: LLMService
    tts: TTSService
    context: LLMContext
    settings: Settings
    def make_task(self, transport) -> PipelineTask: ...

def build_pipeline(settings: Settings) -> PipelineBuild:
    stt = build_stt(settings.stt)
    llm = build_llm(settings.llm)
    tts = build_tts(settings.tts)
    context = interview_context(settings)          # system + greeting
    return PipelineBuild(stt, llm, tts, context, settings)
```

`make_task(transport)` assembles:
```
Pipeline([
    transport.input(),
    stt,
    ctx_aggregator.user(),
    llm,
    tts,
    transport.output(),
    ctx_aggregator.assistant(),
])
```
`PipelineTask(pipeline, params=PipelineParams(enable_metrics=True,
enable_usage_metrics=True, audio_in_sample_rate=settings.pipeline.sample_rate),
observers=[MetricsObserver(settings), TurnTrackingObserver(), UserBotLatencyObserver()])`.

Context-aggregator construction uses the pipecat 1.8 helper (`LLMContextAggregatorPair`
or `llm.create_context_aggregator(context)` — whichever 1.8.1 exposes; verified at build).

## 7. Transport (`transport/websocket.py`)

```python
def register_ws_route(app: FastAPI) -> None:
    @app.websocket("/ws")
    async def ws(websocket: WebSocket):
        await websocket.accept()
        build: PipelineBuild = app.state.pipeline
        params = FastAPIWebsocketParams(
            audio_in_enabled=True, audio_out_enabled=True, add_wav_header=False,
            vad_analyzer=SileroVADAnalyzer() if build.settings.pipeline.vad == "silero" else None,
            serializer=ProtobufFrameSerializer(),
        )
        transport = FastAPIWebsocketTransport(websocket, params)
        session = PipelineSession(build, transport)
        await session.run()          # returns when the socket closes or EndFrame
```
`PipelineSession.run()` = `PipelineRunner().run(build.make_task(transport))`, with the
greeting queued as the first assistant turn on client connect.

If `SileroVADAnalyzer` import path differs in 1.8.1, adjust; `vad: none` is the
escape hatch and what the harness uses.

## 8. Metrics observer (`telemetry/observer.py`)

```python
class MetricsObserver(BaseObserver):
    def __init__(self, settings: Settings): ...   # keeps provider/model labels

    async def on_push_frame(self, data: FramePushed) -> None:
        frame = data.frame
        if isinstance(frame, MetricsFrame):
            for m in frame.data:
                if isinstance(m, TTFBMetricsData):
                    self._record_ttfb(m.processor, m.value)
        elif isinstance(frame, UserStoppedSpeakingFrame):
            self._user_stopped = data.timestamp
        elif isinstance(frame, BotStartedSpeakingFrame) and self._user_stopped:
            VOICE_TO_VOICE.labels(self.settings.llm.model).observe(data.timestamp - self._user_stopped)
            self._user_stopped = None
```
`_record_ttfb` matches the processor name against the built STT/LLM/TTS instances
(by `id()` or `.name`) and routes to `STT_LATENCY` / `LLM_TTFT` / `TTS_FIRST_AUDIO`
with `(provider, model|voice)` labels from settings.

`BaseObserver` hook name/signature (`on_push_frame` vs `on_frame_pushed`) confirmed
against 1.8.1 at build time.

## 9. Harness (`harness/`)

- **`memory_transport.py`** — a minimal `BaseTransport` (or a pair of
  `FrameProcessor`s) that: on start, reads the WAV, emits `StartFrame` then
  `InputAudioRawFrame` chunks (20 ms) at the file's rate, then
  `UserStoppedSpeakingFrame`; collects every `OutputAudioRawFrame` and the
  `TranscriptionFrame` / `LLMTextFrame`s it sees; ends on `EndFrame` or input
  exhaustion + a short drain timeout.
- **`run_clip.py`** — `argparse` for `<wav>` and `--config`; loads settings (forcing
  `pipeline.vad=none`), `build_pipeline`, runs it through the memory transport with
  a fresh `CollectorRegistry` so metrics are per-run, then prints:
  ```
  clip=short_answer.wav  provider=mock/mock/mock
  stt_latency_ms      ...
  llm_ttft_ms         ...
  tts_first_audio_ms  ...
  voice_to_voice_ms   ...
  total_ms            ...
  ```
  Exit 0 on success, 1 if the pipeline produced no bot audio.
- **`fixtures/short_answer.wav`** — generated by `tools/gen_fixtures.py` (a 1 s
  200–400 Hz sweep at 16 kHz mono). Small, deterministic, committed.

## 10. App wiring (`app.py`, `__main__.py`)

- `lifespan`: `app.state.pipeline = build_pipeline(settings)` (raises → uvicorn
  exits non-zero, message printed); log `pipeline ready | stt=… llm=… tts=…`.
- `create_app`: call `register_ws_route(app)` after route setup.
- No change to `/health` or `/metrics` (the pipeline feeds the same default
  registry the `/metrics` endpoint already serializes).

## 11. Testing (`tests/`)

- **`test_providers.py`** — `build_stt/llm/tts` with `provider=mock` return the mock
  classes; `provider="nope"` → `UnknownProvider`; a real factory with its import
  monkeypatched to raise `ModuleNotFoundError` → `ProviderNotInstalled` and the
  message names a `uv add` target.
- **`test_pipeline.py`** — `build_pipeline(Settings())` succeeds; a `PipelineTask`
  with a tail capture processor, fed synthetic `InputAudioRawFrame`s +
  `UserStoppedSpeakingFrame`, yields at least one `TranscriptionFrame`, one
  `LLMTextFrame`, one `TTSAudioRawFrame` (or `OutputAudioRawFrame`).
- **`test_observer.py`** — feed `MetricsObserver` a synthetic `MetricsFrame` with a
  `TTFBMetricsData` for each stage processor → each histogram `_count` == 1;
  `UserStoppedSpeakingFrame` then `BotStartedSpeakingFrame` 0.9 s later →
  `VOICE_TO_VOICE` observes ≈0.9.
- **`test_harness.py`** — `run_clip.main(["harness/fixtures/short_answer.wav"])`
  returns 0 and its stdout contains all five metric lines.
- **`test_app.py`** (extend) — `create_app(Settings())` registers a `/ws` route;
  lifespan sets `app.state.pipeline`.

All tests use mock providers; no network, no models. `make check` stays green
(ruff + mypy --strict + pytest).

## 12. Out of scope (later slices)

- MCP client (`pipecat.services.mcp_service`) → Go's MCP server.
- Read-only Postgres / pgvector speculative retrieval.
- Real STT/LLM/TTS extras, LiteLLM proxy, Prometheus/Grafana `docker-compose`.
- The actual 10–15 recorded bake-off clips and the comparison runs.
- Barge-in / interruption tuning, dynamic question generation, scoring.
- Telephony (Twilio), deployment.

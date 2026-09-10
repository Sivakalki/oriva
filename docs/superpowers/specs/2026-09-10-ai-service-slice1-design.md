# AI Service — Slice 1 Design (Foundations: uv, config, logging, HTTP)

**Date:** 2026-09-10
**Status:** Approved for build
**Scope:** Project scaffold for the Python AI voice service. `uv` setup, one
config file, loguru logging, a FastAPI health/metrics server, and the
stage-boundary metric declarations. **No Pipecat pipeline, transport, MCP client,
or Postgres client** — those are slice 2+.

## 1. Goal

Stand up `ai-service-python/` as a runnable, tested, `uv`-managed project that
loads a single config, logs through one pipe, and serves `/health` + `/metrics`.
This is the foundation the Phase 1 bake-off pipeline (`docs/PLAN.md`) is built on.

The service owns the real-time voice pipeline (STT → LLM → TTS) per
`docs/ARCHITECTURE.md` §1. All three stages are pluggable and vendor-undecided;
config is provider-keyed from day one so the bake-off is a config change.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | `ai-service-python/` at repo root, separate from `backend-go/` | `docs/CLAUDE.md` repo structure |
| D2 | `uv` project, Python 3.12, `src/` layout, package `oriva_ai` | User directive; `src/` avoids import-shadowing in tests |
| D3 | One `config.yaml`, committed directly (dev defaults, no secrets) | User directive; secrets come from env overrides |
| D4 | `pydantic-settings` model, YAML source + `ORIVA_AI__*` env overrides (`__` nested delimiter) | Single typed config object; 12-factor overrides |
| D5 | `loguru` for logging, with stdlib-logging interception | Pipecat already logs via loguru — one pipe |
| D6 | FastAPI + `uvicorn[standard]` for HTTP | Pipecat-idiomatic; later hosts WebRTC/WebSocket signaling |
| D7 | Declare stage-boundary Prometheus histograms now, feed them in slice 2 | `docs/PLAN.md` Phase 1 step 2 wants instrumentation from the start |
| D8 | Provider SDK extras (`pipecat-ai[whisper,...]`) deferred to slice 2 | Shortlist not known until bake-off planning |

## 3. Layout

```
ai-service-python/
  pyproject.toml            # uv-managed; project + deps + tool config (ruff, mypy, pytest)
  uv.lock
  .python-version           # 3.12
  .gitignore
  README.md
  config.yaml               # the single config, committed
  Makefile                  # run, dev, test, lint, typecheck, lock
  src/oriva_ai/
    __init__.py             # __version__
    __main__.py             # entrypoint: loads config, configures logging, runs uvicorn
    config.py               # Settings model + get_settings()
    logging.py              # setup_logging(settings)
    app.py                  # create_app(settings) -> FastAPI; /health, /metrics, lifespan
    telemetry/
      __init__.py
      metrics.py            # Prometheus histogram/counter declarations
    pipeline/
      __init__.py           # empty stub; slice 2 fills this
  tests/
    __init__.py
    conftest.py             # settings fixture
    test_config.py
    test_logging.py
    test_app.py
```

## 4. `pyproject.toml`

- `[project]` name `oriva-ai`, `requires-python = ">=3.12,<3.13"`, version `0.1.0`.
- `[project.scripts]` `oriva-ai = "oriva_ai.__main__:main"`.
- Dependencies: `pipecat-ai`, `pydantic>=2`, `pydantic-settings>=2`, `pyyaml`,
  `loguru`, `fastapi`, `uvicorn[standard]`, `prometheus-client`.
- `[dependency-groups] dev`: `pytest`, `pytest-asyncio`, `httpx` (FastAPI test
  client), `ruff`, `mypy`, `types-pyyaml`.
- `[tool.hatchling]` (or uv's default build backend) packages = `src/oriva_ai`.
- `[tool.ruff]` line-length 100, target `py312`; `[tool.pytest.ini_options]`
  `asyncio_mode = "auto"`, `testpaths = ["tests"]`.

## 5. Configuration (`config.py`)

One `Settings` object built from `pydantic-settings`:

```python
class STTConfig(BaseModel):
    provider: str = "whisper"
    model: str = "base.en"
    language: str = "en"

class LLMConfig(BaseModel):
    provider: str = "litellm"
    model: str = "openai/gpt-4o-mini"
    base_url: str = "http://localhost:4000"
    api_key: str = "not-needed-for-local-litellm"

class TTSConfig(BaseModel):
    provider: str = "piper"
    voice: str = "en_US-lessac-medium"

class TransportConfig(BaseModel):
    type: Literal["websocket", "webrtc"] = "websocket"

class PostgresConfig(BaseModel):
    dsn: str = "postgresql://oriva:oriva@localhost:5432/oriva"
    readonly: bool = True            # Python only ever reads (ARCHITECTURE.md §2)

class MCPConfig(BaseModel):
    server_url: str = "http://localhost:8080/mcp"

class ServerConfig(BaseModel):
    host: str = "0.0.0.0"
    port: int = 8090

class LoggingConfig(BaseModel):
    level: str = "DEBUG"
    format: Literal["console", "json"] = "console"   # "json" field name shadows BaseModel

class AppConfig(BaseModel):
    name: str = "oriva-ai"
    env: Literal["dev", "staging", "prod"] = "dev"

class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_prefix="ORIVA_AI__",
        env_nested_delimiter="__",
        extra="forbid",
    )
    app: AppConfig = AppConfig()
    logging: LoggingConfig = LoggingConfig()
    server: ServerConfig = ServerConfig()
    transport: TransportConfig = TransportConfig()
    stt: STTConfig = STTConfig()
    llm: LLMConfig = LLMConfig()
    tts: TTSConfig = TTSConfig()
    postgres: PostgresConfig = PostgresConfig()
    mcp: MCPConfig = MCPConfig()
    metrics_enabled: bool = True
```

`get_settings(path: str | Path = "config.yaml") -> Settings`:
1. read the YAML file if it exists (path overridable via `ORIVA_AI_CONFIG_FILE`),
2. construct `Settings(**yaml_data)`,
3. pydantic-settings layers env-var overrides on top,
4. `@lru_cache` the result.

Validation errors surface as a `pydantic.ValidationError` — `__main__` catches it,
prints a readable summary, exits non-zero.

`config.yaml` mirrors the defaults above verbatim so the file is the source of
truth in dev and the Python defaults are just a safety net.

## 6. Logging (`logging.py`)

`setup_logging(settings: LoggingConfig) -> None`:
- remove loguru's default handler,
- add a `stderr` sink: colorized human format when `format="console"`,
  `serialize=True` (JSON lines) when `format="json"`, level from `settings.level`,
- install an `InterceptHandler` on the root stdlib logger so `logging`-based
  output (uvicorn, pipecat sub-deps) is redirected into loguru,
- set uvicorn/`uvicorn.access` loggers to propagate.

Called once from `__main__.main()` before the app starts.

## 7. HTTP app (`app.py`)

`create_app(settings: Settings) -> FastAPI`:
- `GET /health` → `{"status": "ok", "service": settings.app.name, "version": __version__}`.
- `GET /metrics` → `prometheus_client.generate_latest()` with the right content type
  (only when `settings.metrics_enabled`, else 404).
- `lifespan`: logs a startup line with resolved provider choices
  (`stt=whisper/base.en llm=litellm/openai/gpt-4o-mini tts=piper/...`); a
  `# TODO(slice 2): start pipeline runner` marker. Nothing else yet.
- `app.state.settings = settings` so later routers can reach config.

`__main__.main()`: `get_settings()` → `setup_logging()` → `uvicorn.run(create_app(...),
host, port, log_config=None)` (loguru owns logging).

## 8. Telemetry (`telemetry/metrics.py`)

Module-level Prometheus objects, registered on import:

```python
STT_LATENCY   = Histogram("oriva_stt_latency_seconds",
                          "STT finalize latency", ["provider", "model"])
LLM_TTFT      = Histogram("oriva_llm_ttft_seconds",
                          "LLM time to first token", ["provider", "model"])
TTS_FIRST_AUDIO = Histogram("oriva_tts_first_audio_seconds",
                            "TTS time to first audio", ["provider", "voice"])
VOICE_TO_VOICE  = Histogram("oriva_voice_to_voice_seconds",
                            "End of user speech to start of bot audio", ["llm_model"])
PIPELINE_ERRORS = Counter("oriva_pipeline_errors_total",
                          "Pipeline errors", ["stage"])
```

Buckets tuned to the `docs/ARCHITECTURE.md` §1 latency budget (e.g. TTFT
`[.05,.1,.15,.2,.3,.4,.6,1,2]`). No code feeds these in slice 1.

## 9. Testing

`pytest` (`asyncio_mode=auto`), `httpx` client:
- `test_config.py` — defaults load; a YAML file overrides a nested value; an
  `ORIVA_AI__LLM__MODEL` env var overrides the YAML; unknown key → `ValidationError`
  (`extra="forbid"`); `get_settings` is cached.
- `test_logging.py` — `setup_logging` with `format="json"` emits parseable JSON to the
  sink; stdlib `logging.getLogger("x").info(...)` reaches the loguru sink
  (capture via a list sink).
- `test_app.py` — `/health` returns 200 with the expected keys; `/metrics` returns
  200 and `text/plain; version=0.0.4` and contains `oriva_stt_latency_seconds`;
  `/metrics` 404s when `metrics_enabled=False`.

`make test` runs all of it; no external services required.

## 10. Makefile targets

`run` (`uv run oriva-ai`), `dev` (uvicorn `--reload`), `test` (`uv run pytest`),
`lint` (`uv run ruff check`), `fmt` (`uv run ruff format`), `typecheck`
(`uv run mypy src`), `lock` (`uv lock`), `sync` (`uv sync`).

## 11. Out of scope (later slices)

- **Slice 2:** Pipecat pipeline skeleton — STT/LLM/TTS services as config-driven
  swappable Pipecat processors, local WebSocket transport, wiring the metrics,
  a fixed test-clip harness.
- MCP client (`pipecat` `MCPClient` → Go's MCP server).
- Read-only Postgres / pgvector speculative retrieval path.
- LiteLLM proxy setup + `docker-compose` for Prometheus/Grafana.
- Telephony (Twilio), barge-in tuning, real STT/LLM/TTS provider selection.
- Dockerfile, deployment.

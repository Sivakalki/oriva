# oriva/ai-service-python

Python voice service for the AI interview platform — owns the real-time
Pipecat **STT → LLM → TTS** pipeline (`docs/ARCHITECTURE.md` §1).

- **Slice 1** — `uv` scaffold, single `config.yaml`, loguru logging,
  FastAPI `/health` + `/metrics`, stage-boundary metric declarations.
  [spec](../docs/superpowers/specs/2026-09-10-ai-service-slice1-design.md)
- **Slice 2** — Pipecat pipeline skeleton: provider-pluggable STT/LLM/TTS
  (`mock` by default), `/ws` WebSocket transport, stage-boundary metrics
  observer, offline clip harness.
  [spec](../docs/superpowers/specs/2026-09-10-ai-service-slice2-design.md)
- **Slice 3** — MCP client: connects to the backend-go MCP server, loads
  `oriva.tools.v1` into the LLM context per `/ws` connection.
  [spec](../docs/superpowers/specs/2026-09-10-ai-service-slice3-design.md)
- **Slice 4** — bake-off harness: fixed clip set + combo-matrix runner scoring
  stage latency and WER; Prometheus/Grafana compose.
  [spec](../docs/superpowers/specs/2026-09-10-ai-service-slice4-design.md)

## Pipeline

STT → LLM → TTS. Providers are chosen per stage in `config.yaml`; `mock` (the
dev default) needs no API keys or models. Real providers (`whisper`, `openai` /
`litellm`, `piper`) are wired but lazily imported — selecting one whose
`pipecat-ai` extra is not installed raises a clear error.

```bash
uv run oriva-ai                       # /ws?session_id=<id> pipeline endpoint on :8090
# offline: run one clip through the pipeline and print stage latencies
uv run python -m harness.run_clip src/harness/fixtures/short_answer.wav
```

## MCP tools

When `mcp.enabled`, each `/ws` connection opens an MCP client to the Go server
(`mcp.server_url`, bearer `mcp.auth_token`) and hands the LLM the four
`oriva.tools.v1` tools. `session_id` (the `/ws` query param) is bound to every
tool call via `tools_arguments` — the LLM never sees it.

```bash
# live cross-service test (needs backend-go running)
ORIVA_GO_MCP_URL=http://localhost:8080/mcp make test-integration
```

## Bake-off

Score STT/LLM/TTS combinations (`bakeoff/combos.yaml`) over the fixed clip set
(`bakeoff/clips/`) on stage latency and word error rate (`docs/PLAN.md` Phase 1).

```bash
make bakeoff                              # writes bakeoff/out/{results.csv,results.json}
make obs-up                               # Prometheus :9090, Grafana :3000, Pushgateway :9091
uv run python -m bakeoff --push           # push aggregates to Grafana
```

The committed clips are synthetic; see `bakeoff/clips/README.md` to drop in real
recordings. Real provider combos need `uv add "pipecat-ai[whisper,openai,piper]"`.

## Quick start

```bash
cd ai-service-python
uv sync
uv run oriva-ai          # serves on :8090
curl localhost:8090/health
curl localhost:8090/metrics
```

## Config

One file: [`config.yaml`](config.yaml), committed with dev defaults (no secrets).
Override anything with env vars using `__` as the nesting delimiter:

```bash
ORIVA_AI__LLM__MODEL=anthropic/claude-sonnet-5 uv run oriva-ai
ORIVA_AI__LOGGING__LEVEL=INFO ORIVA_AI__LOGGING__JSON=true uv run oriva-ai
```

Point at a different file with `ORIVA_AI_CONFIG_FILE=/path/to/other.yaml`.

## Layout

Flat `src/` (no wrapping package): every top-level dir under `src/` is
directly importable (`import app`, `import pipelines`, ...), mirroring
`backend-go`'s handler → service → repo split — `api/` are the handlers,
`pipelines/` is the service layer, `pipelines/providers/` is the repo layer
(one subpackage per STT/LLM/TTS engine, swapped via `config.yaml`).

| Path | Purpose |
|---|---|
| `src/config.py` | `Settings` (pydantic-settings) + `load_settings()` |
| `src/log_setup.py` | loguru sink + stdlib-logging interception |
| `src/app.py` | FastAPI app factory (mounts `api/v1`, `/ws`, lifespan) |
| `src/api/v1/` | HTTP handlers: `health.py`, `metrics.py`, combined in `routes.py` |
| `src/pipelines/assembly.py` | `build_pipeline(settings)` → services + task factory |
| `src/pipelines/providers/` | one subpackage per provider (`mock/`, `whisper/`, `piper/`, `openai/`), each exposing `build_stt`/`build_llm`/`build_tts`; `registry.py` maps `config.yaml`'s `provider:` string to one |
| `src/transport/websocket.py` | `/ws` FastAPI WebSocket transport |
| `src/telemetry/metrics.py` | Prometheus histograms for pipeline stages |
| `src/telemetry/observer.py` | times each stage boundary → the histograms |
| `src/harness/` | offline clip runner + synthetic fixtures |
| `src/bakeoff/` | provider combo-matrix runner (see Bake-off above) |

Adding a new STT/LLM/TTS provider: drop a new subpackage under
`pipelines/providers/` exposing a `build_stt`/`build_llm`/`build_tts`
factory (import the SDK lazily inside the function, like the existing ones),
add one line to the matching dict in `pipelines/providers/registry.py`, then
select it via `config.yaml`'s `provider:` key — no other code changes.

## Dev

```bash
make check      # ruff + mypy + pytest
make dev        # uvicorn --reload
```

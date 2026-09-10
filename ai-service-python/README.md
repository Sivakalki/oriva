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

## Pipeline

STT → LLM → TTS. Providers are chosen per stage in `config.yaml`; `mock` (the
dev default) needs no API keys or models. Real providers (`whisper`, `openai` /
`litellm`, `piper`) are wired but lazily imported — selecting one whose
`pipecat-ai` extra is not installed raises a clear error.

```bash
uv run oriva-ai                       # /ws?session_id=<id> pipeline endpoint on :8090
# offline: run one clip through the pipeline and print stage latencies
uv run python -m oriva_ai.harness.run_clip src/oriva_ai/harness/fixtures/short_answer.wav
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

| Path | Purpose |
|---|---|
| `src/oriva_ai/config.py` | `Settings` (pydantic-settings) + `load_settings()` |
| `src/oriva_ai/logging.py` | loguru sink + stdlib-logging interception |
| `src/oriva_ai/app.py` | FastAPI app factory (`/health`, `/metrics`, lifespan) |
| `src/oriva_ai/telemetry/metrics.py` | Prometheus histograms for pipeline stages |
| `src/oriva_ai/telemetry/observer.py` | times each stage boundary → the histograms |
| `src/oriva_ai/pipeline/providers/` | provider registry: `mock` + lazy `whisper`/`openai`/`piper` |
| `src/oriva_ai/pipeline/assembly.py` | `build_pipeline(settings)` → services + task factory |
| `src/oriva_ai/transport/websocket.py` | `/ws` FastAPI WebSocket transport |
| `src/oriva_ai/harness/` | offline clip runner + synthetic fixtures |

## Dev

```bash
make check      # ruff + mypy + pytest
make dev        # uvicorn --reload
```

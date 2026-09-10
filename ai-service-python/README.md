# oriva/ai-service-python

Python voice service for the AI interview platform — owns the real-time
Pipecat **STT → LLM → TTS** pipeline (`docs/ARCHITECTURE.md` §1).

- **Slice 1** (this) — `uv` scaffold, single `config.yaml`, loguru logging,
  FastAPI `/health` + `/metrics`, stage-boundary metric declarations.
  [spec](../docs/superpowers/specs/2026-09-10-ai-service-slice1-design.md)
- **Slice 2** (next) — Pipecat pipeline skeleton: config-driven swappable
  STT/LLM/TTS, local WebSocket transport, wired metrics, test-clip harness.

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
| `src/oriva_ai/pipeline/` | the pipeline (slice 2) |

## Dev

```bash
make check      # ruff + mypy + pytest
make dev        # uvicorn --reload
```

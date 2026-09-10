# AI Service — Slice 4 Design (STT/LLM/TTS Bake-off Harness)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** ai-service slice 2 (`6cffe0c`), slice 3 (`1d0c4c0`)
**Scope:** `docs/PLAN.md` Phase 1 steps 2–5 — a fixed clip set, a combo-matrix
runner that scores STT/LLM/TTS combinations on stage latency and STT accuracy,
a comparison report, and the Prometheus/Grafana compose. No real recorded clips,
no provider-selection decision, no PSTN re-validation.

## 1. Goal

`docs/PLAN.md` Phase 1: "pick a concrete STT/LLM/TTS combination empirically."
This slice builds the measuring apparatus:

- **step 2** — instrument every stage boundary (done in slice 2) + stand up
  Prometheus + Grafana via docker-compose.
- **step 3** — a fixed test set of candidate-answer clips, reused across combos.
- **step 4** — run the STT bake-off: compare on latency *and* transcription
  accuracy (word error rate).
- **step 5** — run the LLM × TTS matrix against the winning STT.

The harness is provider-agnostic. With only the `mock` providers installed the
numbers are synthetic; they become real when the `whisper` / `openai` (LiteLLM) /
`piper` extras and recorded clips are added.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | Extract slice 2's `run_clip._run` into a reusable `harness/clip_run.py::run_clip(settings, wav) -> ClipResult` | The bake-off runner and the single-clip CLI share one code path |
| D2 | Clip set = a directory of WAVs + `manifest.yaml` (reference transcript + tags) | Real recordings drop in by adding files + manifest rows; no code change |
| D3 | WER implemented in-repo (Levenshtein on normalized tokens), no `jiwer` dep | ~20 lines; keeps the dependency surface small |
| D4 | Combos declared in `bakeoff/combos.yaml`; each is a full provider triple | The bake-off is a config artifact, reviewable and diffable |
| D5 | A combo whose provider extra is missing → caught, marked `unavailable`, matrix continues | Running the mock combo must not require installing whisper/piper |
| D6 | Report = console table + `results.csv` + `results.json` | Table for humans, CSV for spreadsheets, JSON for later tooling / CI gates |
| D7 | `docker-compose.yml` runs Prometheus + Grafana + Pushgateway; Prometheus scrapes the live `/metrics` and the pushgateway | Live `/ws` calls and offline harness runs both land in Grafana |
| D8 | Harness → pushgateway is opt-in (`--push`) | The default run just writes report files; no container needed |

## 3. Refactor: `harness/clip_run.py` (new)

```python
@dataclass
class ClipResult:
    clip_id: str
    transcript: str
    reply: str
    audio_chunks: int
    stt_ms: float | None
    llm_ttft_ms: float | None
    tts_ttfa_ms: float | None
    v2v_ms: float | None

async def run_clip(settings: Settings, wav: Path, clip_id: str = "") -> ClipResult
```
Body is slice 2's `_run`: `build_pipeline(settings)` → `build_offline_pipeline`
with two `Collector`s → `run_test` with `wav_to_frames` → derive timings from the
collectors' timestamped events. `harness/run_clip.py` becomes a thin CLI that
calls `run_clip` and prints the one-clip block it prints today.

## 4. Clip set (`bakeoff/clips/`)

`manifest.yaml`:
```yaml
clips:
  - id: clean_short
    file: clean_short.wav
    reference_transcript: "I have about five years of backend experience."
    tags: [clean, short]
  - id: filler_medium
    file: filler_medium.wav
    reference_transcript: "Um, so, I mostly worked on, like, payment systems and, uh, some infra."
    tags: [fillers, medium]
  - id: accent_long
    file: accent_long.wav
    reference_transcript: "..."
    tags: [accent, long]
  - id: noisy_short
    file: noisy_short.wav
    reference_transcript: "..."
    tags: [noise, short]
```
`tools/gen_bakeoff_clips.py` synthesizes 4–6 mono 16 kHz WAVs (tone sweeps of
varying length + a noise-mixed one) and writes the manifest. Small, committed,
deterministic. A `README.md` in the dir explains how to replace them with real
recordings (drop `*.wav`, add manifest rows with the true transcript + tags).

## 5. Combos (`bakeoff/combos.yaml`)

```yaml
combos:
  - name: all-mock
    stt: { provider: mock,   model: base.en }
    llm: { provider: mock,   model: mock }
    tts: { provider: mock,   voice: mock }
  # --- examples that need extras (uv add "pipecat-ai[whisper,openai,piper]") ---
  # - name: whisper-litellm-piper
  #   stt: { provider: whisper, model: base.en }
  #   llm: { provider: litellm, model: openai/gpt-4o-mini, base_url: http://localhost:4000 }
  #   tts: { provider: piper,   voice: en_US-lessac-medium }
```
A combo is turned into `Settings` by `Settings().model_copy` with the `stt`/`llm`/
`tts` sub-models replaced (VAD forced `none`, MCP forced disabled — the harness is
offline).

## 6. Metrics (`bakeoff/metrics.py`)

```python
def normalize(text: str) -> list[str]      # lowercase, strip punctuation, split
def word_error_rate(reference: str, hypothesis: str) -> float
    # Levenshtein(ref_tokens, hyp_tokens) / max(len(ref_tokens), 1)
    # empty hypothesis -> 1.0; empty reference & empty hyp -> 0.0

@dataclass
class ComboAggregate:
    combo: str
    available: bool
    n: int
    stt_ms_p50: float | None
    stt_ms_p95: float | None
    llm_ttft_ms_p50: float | None
    llm_ttft_ms_p95: float | None
    tts_ttfa_ms_p50: float | None
    tts_ttfa_ms_p95: float | None
    v2v_ms_p50: float | None
    v2v_ms_p95: float | None
    wer_mean: float | None
    error: str | None                       # set when available is False

def aggregate(combo: str, results: list[tuple[ClipResult, float]]) -> ComboAggregate
    # results is (clip_result, wer) per clip; percentiles via statistics.quantiles
```

## 7. Runner (`bakeoff/runner.py`)

```python
@dataclass
class MatrixResult:
    combos: list[ComboAggregate]
    per_clip: list[dict]      # flat rows: {combo, clip_id, stt_ms, ..., wer}

async def run_matrix(
    combos: list[dict], clips: list[dict], base: Settings | None = None
) -> MatrixResult

def main(argv: list[str] | None = None) -> int
    # `python -m oriva_ai.bakeoff [--combos path] [--clips path] [--out dir] [--push]`
```
For each combo:
- build `Settings`; `try: build_pipeline(settings)` to fail fast on a missing
  extra → `ProviderNotInstalled`/`ProviderError` → `ComboAggregate(available=False,
  error=str(exc))`, continue.
- for each clip: `run_clip(settings, clips_dir/clip.file, clip.id)`, then
  `word_error_rate(clip.reference_transcript, result.transcript)`.
- `aggregate(...)`.

`main` loads the yaml files, runs the matrix, calls `report.write(...)`, and if
`--push` calls `push.push(...)`. Returns non-zero if every combo is unavailable.

## 8. Report (`bakeoff/report.py`)

```python
def render_table(m: MatrixResult) -> str          # aligned text table, one row per combo
def write(m: MatrixResult, out_dir: Path) -> None  # prints render_table; writes results.csv + results.json
```
CSV columns: `combo, available, n, stt_ms_p50, stt_ms_p95, llm_ttft_ms_p50,
llm_ttft_ms_p95, tts_ttfa_ms_p50, tts_ttfa_ms_p95, v2v_ms_p50, v2v_ms_p95,
wer_mean, error`. JSON: `{combos: [...], per_clip: [...]}` (dataclasses via
`dataclasses.asdict`).

## 9. Pushgateway (`bakeoff/push.py`)

```python
def push(m: MatrixResult, gateway: str = "localhost:9091", job: str = "oriva_bakeoff") -> None
```
Uses `prometheus_client.CollectorRegistry` + `push_to_gateway`. One `Gauge` per
metric, labelled `combo`; skips unavailable combos. Import of
`prometheus_client.exposition.push_to_gateway` is already available (dep exists).

## 10. Observability compose (`docker-compose.yml` + `ops/`)

`ai-service-python/docker-compose.yml`:
```yaml
services:
  prometheus:
    image: prom/prometheus:v3.1.0
    volumes: [./ops/prometheus.yml:/etc/prometheus/prometheus.yml:ro]
    ports: ["9090:9090"]
    extra_hosts: ["host.docker.internal:host-gateway"]
  pushgateway:
    image: prom/pushgateway:v1.11.0
    ports: ["9091:9091"]
  grafana:
    image: grafana/grafana:11.5.0
    environment: [GF_AUTH_ANONYMOUS_ENABLED=true, GF_AUTH_ANONYMOUS_ORG_ROLE=Admin]
    volumes: [./ops/grafana/provisioning:/etc/grafana/provisioning:ro]
    ports: ["3000:3000"]
```
`ops/prometheus.yml` — scrape `host.docker.internal:8090` (`/metrics`, the live
service) every 5s and `pushgateway:9091` (with `honor_labels: true`).
`ops/grafana/provisioning/datasources/prometheus.yml` — the Prometheus datasource.
`ops/grafana/provisioning/dashboards/` — a provider + one `oriva-pipeline.json`
dashboard: four panels, `histogram_quantile(0.5|0.95, sum by (le, provider, model)
(rate(oriva_*_seconds_bucket[5m])))` for STT / LLM TTFT / TTS TTFA / voice-to-voice.

Makefile: `obs-up` (`docker compose up -d`), `obs-down`.

## 11. Makefile / entrypoints

- `bakeoff` — `uv run python -m oriva_ai.bakeoff` (writes `bakeoff/out/`).
- `gen-clips` — `uv run python tools/gen_bakeoff_clips.py`.
- `obs-up` / `obs-down`.
- `bakeoff/out/` is gitignored.

## 12. Testing (`tests/`)

- **`test_bakeoff_metrics.py`** — `word_error_rate("a b c d e", "a b c d e") == 0`;
  one substitution → `0.2`; `word_error_rate("a b", "") == 1.0`;
  `word_error_rate("", "") == 0.0`; `normalize` strips punctuation/case.
  `aggregate` — percentiles from a known list; `n` correct.
- **`test_bakeoff_runner.py`** — `run_matrix([all-mock], <2 synthetic clips>)` →
  one `ComboAggregate`, `available`, `n == 2`, latency percentiles not None,
  `per_clip` has 2 rows. A combo `{stt:{provider:whisper}}` with the whisper
  factory monkeypatched to raise `ModuleNotFoundError` → `available is False`,
  `error` set, matrix still returns.
- **`test_bakeoff_report.py`** — `write(m, tmp)` creates `results.csv` (header +
  one row per combo) and `results.json` (`json.load` round-trips, `combos` key).
  `render_table` contains each combo name.
- **`test_clip_run.py`** — `run_clip(Settings(), fixture)` → `ClipResult` with
  `transcript` non-empty, `reply` non-empty, `audio_chunks > 0`, all four `*_ms`
  populated.
- `test_harness.py` (slice 2) updated to import from `clip_run` if its internals
  moved.

`make test` stays fast (mock only, no containers). The compose is not exercised
in CI.

## 13. Out of scope (later)

- Real recorded candidate clips (this ships 4–6 synthetic ones + the structure).
- Installing/running the real STT/LLM/TTS providers and LiteLLM proxy.
- Picking the winning combo / writing it back to `config.yaml`.
- PSTN/Twilio re-validation (`docs/PLAN.md` Phase 1 step 6).
- CI gates on latency/WER regression.
- Barge-in / interruption measurement.

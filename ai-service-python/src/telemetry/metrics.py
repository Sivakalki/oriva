"""Stage-boundary metrics for the voice pipeline.

Declared now (docs/PLAN.md Phase 1 step 2); fed once the pipeline exists (slice 2).
Buckets follow the latency budget in docs/ARCHITECTURE.md section 1.
"""

from __future__ import annotations

from prometheus_client import Counter, Histogram

_FAST_BUCKETS = (0.02, 0.05, 0.1, 0.15, 0.2, 0.3, 0.4, 0.6, 1.0, 2.0)
_V2V_BUCKETS = (0.3, 0.5, 0.8, 1.0, 1.2, 1.5, 2.0, 3.0, 5.0)

STT_LATENCY = Histogram(
    "oriva_stt_latency_seconds",
    "STT finalize latency (end of utterance to final transcript).",
    ["provider", "model"],
    buckets=_FAST_BUCKETS,
)

LLM_TTFT = Histogram(
    "oriva_llm_ttft_seconds",
    "LLM time to first token.",
    ["provider", "model"],
    buckets=_FAST_BUCKETS,
)

TTS_FIRST_AUDIO = Histogram(
    "oriva_tts_first_audio_seconds",
    "TTS time to first audio chunk.",
    ["provider", "voice"],
    buckets=_FAST_BUCKETS,
)

VOICE_TO_VOICE = Histogram(
    "oriva_voice_to_voice_seconds",
    "End of user speech to start of bot audio playback.",
    ["llm_model"],
    buckets=_V2V_BUCKETS,
)

PIPELINE_ERRORS = Counter(
    "oriva_pipeline_errors_total",
    "Pipeline errors by stage.",
    ["stage"],
)

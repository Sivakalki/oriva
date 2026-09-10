"""Timestamp each pipeline stage boundary and record it to Prometheus.

This follows docs/PLAN.md Phase 1 step 2 ("timestamp each handoff") directly,
watching the frames that cross each boundary rather than relying on Pipecat's
internal metrics frames. Timestamps are the pipeline clock, in nanoseconds.
"""

from __future__ import annotations

from pipecat.frames.frames import (
    BotStartedSpeakingFrame,
    LLMTextFrame,
    TranscriptionFrame,
    TTSAudioRawFrame,
    UserStoppedSpeakingFrame,
)
from pipecat.observers.base_observer import BaseObserver, FramePushed

from oriva_ai.config import Settings
from oriva_ai.telemetry.metrics import (
    LLM_TTFT,
    STT_LATENCY,
    TTS_FIRST_AUDIO,
    VOICE_TO_VOICE,
)

_NS = 1e9


class MetricsObserver(BaseObserver):
    """Records STT / LLM TTFT / TTS TTFA / voice-to-voice per turn."""

    def __init__(self, settings: Settings, stt_name: str, llm_name: str, tts_name: str) -> None:
        super().__init__()
        self._settings = settings
        self._stt_name = stt_name
        self._llm_name = llm_name
        self._tts_name = tts_name
        self._reset()

    def _reset(self) -> None:
        self._user_stopped_ns: float | None = None
        self._transcript_ns: float | None = None
        self._first_llm_text_ns: float | None = None
        self._turn_open = False

    async def on_push_frame(self, data: FramePushed) -> None:
        frame = data.frame
        ts = float(data.timestamp)
        s = self._settings

        if isinstance(frame, UserStoppedSpeakingFrame):
            self._reset()
            self._user_stopped_ns = ts
            self._turn_open = True

        elif isinstance(frame, TranscriptionFrame) and self._user_stopped_ns is not None:
            STT_LATENCY.labels(s.stt.provider, s.stt.model).observe(
                (ts - self._user_stopped_ns) / _NS
            )
            self._transcript_ns = ts

        elif isinstance(frame, LLMTextFrame) and self._first_llm_text_ns is None:
            self._first_llm_text_ns = ts
            if self._transcript_ns is not None:
                LLM_TTFT.labels(s.llm.provider, s.llm.model).observe(
                    (ts - self._transcript_ns) / _NS
                )

        elif isinstance(frame, TTSAudioRawFrame) and self._turn_open:
            if self._first_llm_text_ns is not None:
                TTS_FIRST_AUDIO.labels(s.tts.provider, s.tts.voice).observe(
                    (ts - self._first_llm_text_ns) / _NS
                )
            self._maybe_voice_to_voice(ts)

        elif isinstance(frame, BotStartedSpeakingFrame):
            self._maybe_voice_to_voice(ts)

    def _maybe_voice_to_voice(self, ts: float) -> None:
        if self._turn_open and self._user_stopped_ns is not None:
            delta = (ts - self._user_stopped_ns) / _NS
            if delta >= 0:
                VOICE_TO_VOICE.labels(self._settings.llm.model).observe(delta)
            self._turn_open = False

"""Run one audio clip through the offline pipeline and measure each stage.

Shared by the single-clip CLI (`harness/run_clip.py`) and the bake-off matrix
runner (`bakeoff/runner.py`).
"""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from pipecat.frames.frames import (
    Frame,
    LLMTextFrame,
    TranscriptionFrame,
    TTSAudioRawFrame,
    UserStoppedSpeakingFrame,
)
from pipecat.pipeline.task import PipelineParams
from pipecat.tests.utils import run_test

from config import Settings
from harness.memory_transport import wav_to_frames
from pipelines.assembly import build_pipeline
from pipelines.offline import Collector, build_offline_pipeline
from telemetry.observer import MetricsObserver


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


def _first(events: list[tuple[float, Frame]], kind: type) -> float | None:
    for ts, frame in events:
        if isinstance(frame, kind):
            return ts
    return None


def _ms(start: float | None, end: float | None) -> float | None:
    if start is None or end is None:
        return None
    return (end - start) * 1000


async def run_clip(settings: Settings, wav: Path, clip_id: str = "") -> ClipResult:
    build = build_pipeline(settings)
    post_llm, post_tts = Collector(), Collector()
    pipeline = build_offline_pipeline(build, post_llm=post_llm, post_tts=post_tts)
    observer = MetricsObserver(settings, build.stt.name, build.llm.name, build.tts.name)

    await run_test(
        pipeline,
        frames_to_send=wav_to_frames(wav),
        expected_down_frames=None,
        observers=[observer],
        pipeline_params=PipelineParams(enable_metrics=True, enable_usage_metrics=True),
    )

    ev = sorted(post_llm.events + post_tts.events, key=lambda e: e[0])
    t_user_stop = _first(ev, UserStoppedSpeakingFrame)
    t_transcript = _first(ev, TranscriptionFrame)
    t_llm = _first(ev, LLMTextFrame)
    t_audio = _first(ev, TTSAudioRawFrame)

    return ClipResult(
        clip_id=clip_id or wav.stem,
        transcript=next((f.text for _, f in ev if isinstance(f, TranscriptionFrame)), ""),
        reply="".join(f.text for _, f in ev if isinstance(f, LLMTextFrame)).strip(),
        audio_chunks=sum(1 for _, f in ev if isinstance(f, TTSAudioRawFrame)),
        stt_ms=_ms(t_user_stop, t_transcript),
        llm_ttft_ms=_ms(t_transcript, t_llm),
        tts_ttfa_ms=_ms(t_llm, t_audio),
        v2v_ms=_ms(t_user_stop, t_audio),
    )

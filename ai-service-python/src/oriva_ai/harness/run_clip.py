"""`python -m oriva_ai.harness.run_clip <clip.wav> [--config path]`

Runs one audio clip through the configured pipeline offline (no websocket) and
prints the stage-boundary latencies, measured from the frames crossing each
boundary. The bake-off runs this over a fixed clip set per STT/LLM/TTS combo
(docs/PLAN.md Phase 1).
"""

from __future__ import annotations

import argparse
import asyncio
import sys
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

from oriva_ai.config import load_settings
from oriva_ai.harness.memory_transport import wav_to_frames
from oriva_ai.pipeline.assembly import build_pipeline
from oriva_ai.pipeline.offline import Collector, build_offline_pipeline
from oriva_ai.telemetry.observer import MetricsObserver


def _first(events: list[tuple[float, Frame]], kind: type) -> float | None:
    for ts, frame in events:
        if isinstance(frame, kind):
            return ts
    return None


async def _run(wav: Path, config: str | None) -> int:
    settings = load_settings(config)
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

    transcript = next((f.text for _, f in ev if isinstance(f, TranscriptionFrame)), "")
    reply = "".join(f.text for _, f in ev if isinstance(f, LLMTextFrame)).strip()
    audio_chunks = sum(1 for _, f in ev if isinstance(f, TTSAudioRawFrame))

    providers = f"{settings.stt.provider}/{settings.llm.provider}/{settings.tts.provider}"
    print(f"clip={wav.name}  providers={providers}")
    print(f"  transcript        : {transcript or '(none)'}")
    print(f"  llm_reply         : {reply or '(none)'}")
    print(f"  bot_audio_chunks  : {audio_chunks}")
    print(f"  stt_latency_ms    : {_ms(t_user_stop, t_transcript)}")
    print(f"  llm_ttft_ms       : {_ms(t_transcript, t_llm)}")
    print(f"  tts_first_audio_ms: {_ms(t_llm, t_audio)}")
    print(f"  voice_to_voice_ms : {_ms(t_user_stop, t_audio)}")

    return 0 if audio_chunks > 0 else 1


def _ms(start: float | None, end: float | None) -> str:
    if start is None or end is None:
        return "(n/a)"
    return f"{(end - start) * 1000:.1f}"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="oriva-ai.harness.run_clip")
    parser.add_argument("wav", type=Path)
    parser.add_argument("--config", default=None)
    args = parser.parse_args(argv)
    return asyncio.run(_run(args.wav, args.config))


if __name__ == "__main__":
    sys.exit(main())

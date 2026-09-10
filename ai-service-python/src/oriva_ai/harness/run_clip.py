"""`python -m oriva_ai.harness.run_clip <clip.wav> [--config path]`

Runs one audio clip through the offline pipeline and prints the stage-boundary
latencies. For comparing provider combinations across a clip set, use
`python -m oriva_ai.bakeoff` instead.
"""

from __future__ import annotations

import argparse
import asyncio
import sys
from pathlib import Path

from oriva_ai.config import load_settings
from oriva_ai.harness.clip_run import ClipResult, run_clip


def _fmt(v: float | None) -> str:
    return "(n/a)" if v is None else f"{v:.1f}"


def _print(settings_providers: str, wav: Path, r: ClipResult) -> None:
    print(f"clip={wav.name}  providers={settings_providers}")
    print(f"  transcript        : {r.transcript or '(none)'}")
    print(f"  llm_reply         : {r.reply or '(none)'}")
    print(f"  bot_audio_chunks  : {r.audio_chunks}")
    print(f"  stt_latency_ms    : {_fmt(r.stt_ms)}")
    print(f"  llm_ttft_ms       : {_fmt(r.llm_ttft_ms)}")
    print(f"  tts_first_audio_ms: {_fmt(r.tts_ttfa_ms)}")
    print(f"  voice_to_voice_ms : {_fmt(r.v2v_ms)}")


async def _run(wav: Path, config: str | None) -> int:
    settings = load_settings(config)
    r = await run_clip(settings, wav)
    providers = f"{settings.stt.provider}/{settings.llm.provider}/{settings.tts.provider}"
    _print(providers, wav, r)
    return 0 if r.audio_chunks > 0 else 1


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="oriva-ai.harness.run_clip")
    parser.add_argument("wav", type=Path)
    parser.add_argument("--config", default=None)
    args = parser.parse_args(argv)
    return asyncio.run(_run(args.wav, args.config))


if __name__ == "__main__":
    sys.exit(main())

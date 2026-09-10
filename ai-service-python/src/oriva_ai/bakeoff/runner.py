"""`python -m oriva_ai.bakeoff` — score STT/LLM/TTS combos over a clip set."""

from __future__ import annotations

import argparse
import asyncio
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml

from oriva_ai.bakeoff import report
from oriva_ai.bakeoff.metrics import ComboAggregate, aggregate, unavailable, word_error_rate
from oriva_ai.config import LLMConfig, Settings, STTConfig, TTSConfig
from oriva_ai.harness.clip_run import run_clip
from oriva_ai.pipeline.assembly import build_pipeline
from oriva_ai.pipeline.providers.base import ProviderError

_HERE = Path(__file__).parent
DEFAULT_COMBOS = _HERE / "combos.yaml"
DEFAULT_CLIPS = _HERE / "clips" / "manifest.yaml"


@dataclass
class MatrixResult:
    combos: list[ComboAggregate]
    per_clip: list[dict[str, Any]]


def _settings_for(base: Settings, combo: dict[str, Any]) -> Settings:
    s = base.model_copy(deep=True)
    s.stt = STTConfig(**{**s.stt.model_dump(), **combo.get("stt", {})})
    s.llm = LLMConfig(**{**s.llm.model_dump(), **combo.get("llm", {})})
    s.tts = TTSConfig(**{**s.tts.model_dump(), **combo.get("tts", {})})
    s.pipeline.vad = "none"
    s.mcp.enabled = False
    return s


async def run_matrix(
    combos: list[dict[str, Any]],
    clips: list[dict[str, Any]],
    clips_dir: Path,
    base: Settings | None = None,
) -> MatrixResult:
    base = base or Settings()
    aggregates: list[ComboAggregate] = []
    per_clip: list[dict[str, Any]] = []

    for combo in combos:
        name = combo["name"]
        settings = _settings_for(base, combo)
        try:
            build_pipeline(settings)  # fail fast on a missing provider extra
        except (ProviderError, ImportError, ModuleNotFoundError) as exc:
            aggregates.append(unavailable(name, str(exc)))
            continue

        scored = []
        for clip in clips:
            result = await run_clip(settings, clips_dir / clip["file"], clip["id"])
            wer = word_error_rate(clip.get("reference_transcript", ""), result.transcript)
            scored.append((result, wer))
            per_clip.append(
                {
                    "combo": name,
                    "clip_id": clip["id"],
                    "stt_ms": result.stt_ms,
                    "llm_ttft_ms": result.llm_ttft_ms,
                    "tts_ttfa_ms": result.tts_ttfa_ms,
                    "v2v_ms": result.v2v_ms,
                    "wer": wer,
                }
            )
        aggregates.append(aggregate(name, scored))

    return MatrixResult(combos=aggregates, per_clip=per_clip)


def _load(path: Path, key: str) -> list[dict[str, Any]]:
    data = yaml.safe_load(path.read_text()) or {}
    return list(data.get(key, []))


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="oriva-ai.bakeoff")
    parser.add_argument("--combos", type=Path, default=DEFAULT_COMBOS)
    parser.add_argument("--clips", type=Path, default=DEFAULT_CLIPS)
    parser.add_argument("--out", type=Path, default=_HERE / "out")
    parser.add_argument("--push", action="store_true", help="push aggregates to a Pushgateway")
    parser.add_argument("--gateway", default="localhost:9091")
    args = parser.parse_args(argv)

    combos = _load(args.combos, "combos")
    clips = _load(args.clips, "clips")
    clips_dir = args.clips.parent

    result = asyncio.run(run_matrix(combos, clips, clips_dir))
    report.write(result, args.out)

    if args.push:
        from oriva_ai.bakeoff import push

        push.push(result, gateway=args.gateway)

    if not any(c.available for c in result.combos):
        print("no combos were runnable", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

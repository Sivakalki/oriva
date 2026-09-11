"""Regenerate the synthetic bake-off clip set + manifest.

`python tools/gen_bakeoff_clips.py`

These tone-sweep clips exist so the harness runs end to end with no recordings.
Replace them with real candidate-answer recordings: drop `*.wav` into
`src/bakeoff/clips/` and add manifest rows with the true transcript and
tags (accent / noise / length / fillers) — no code change needed.
"""

from __future__ import annotations

import math
import random
import struct
import wave
from pathlib import Path

import yaml

OUT = Path(__file__).parent.parent / "src" / "bakeoff" / "clips"
RATE = 16000

CLIPS = [
    {
        "id": "clean_short",
        "file": "clean_short.wav",
        "seconds": 1.2,
        "noise": 0.0,
        "reference_transcript": "I have about five years of backend experience.",
        "tags": ["clean", "short"],
    },
    {
        "id": "filler_medium",
        "file": "filler_medium.wav",
        "seconds": 3.0,
        "noise": 0.02,
        "reference_transcript": (
            "Um so I mostly worked on like payment systems and uh some infra tooling."
        ),
        "tags": ["fillers", "medium"],
    },
    {
        "id": "accent_long",
        "file": "accent_long.wav",
        "seconds": 6.0,
        "noise": 0.01,
        "reference_transcript": (
            "In my previous role I led the migration of a monolith to services "
            "and owned the observability stack across three teams."
        ),
        "tags": ["accent", "long"],
    },
    {
        "id": "noisy_short",
        "file": "noisy_short.wav",
        "seconds": 1.5,
        "noise": 0.15,
        "reference_transcript": "I prefer Go for services and Python for the data pipelines.",
        "tags": ["noise", "short"],
    },
]


def _tone(seconds: float, noise: float) -> bytes:
    rng = random.Random(42)
    n = int(RATE * seconds)
    out = bytearray()
    for i in range(n):
        t = i / RATE
        freq = 200 + 180 * (i / n)
        sample = 0.3 * math.sin(2 * math.pi * freq * t)
        if noise:
            sample += noise * (rng.random() * 2 - 1)
        out += struct.pack("<h", max(-32768, min(32767, int(sample * 32767))))
    return bytes(out)


def _write_wav(path: Path, pcm: bytes) -> None:
    with wave.open(str(path), "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(RATE)
        wf.writeframes(pcm)


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for clip in CLIPS:
        _write_wav(OUT / str(clip["file"]), _tone(float(clip["seconds"]), float(clip["noise"])))
        print(f"wrote {OUT / str(clip['file'])}")

    manifest = {
        "clips": [
            {
                "id": c["id"],
                "file": c["file"],
                "reference_transcript": c["reference_transcript"],
                "tags": c["tags"],
            }
            for c in CLIPS
        ]
    }
    (OUT / "manifest.yaml").write_text(yaml.safe_dump(manifest, sort_keys=False))
    print(f"wrote {OUT / 'manifest.yaml'}")


if __name__ == "__main__":
    main()

"""Regenerate harness/fixtures/*.wav — small synthetic clips for the offline harness.

Not run in CI. `python tools/gen_fixtures.py`
"""

from __future__ import annotations

import math
import struct
import wave
from pathlib import Path

OUT = Path(__file__).parent.parent / "src" / "oriva_ai" / "harness" / "fixtures"
RATE = 16000


def _sweep(seconds: float, f0: float, f1: float) -> bytes:
    n = int(RATE * seconds)
    out = bytearray()
    for i in range(n):
        t = i / RATE
        freq = f0 + (f1 - f0) * (i / n)
        sample = int(0.3 * 32767 * math.sin(2 * math.pi * freq * t))
        out += struct.pack("<h", sample)
    return bytes(out)


def _write(name: str, pcm: bytes) -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    with wave.open(str(OUT / name), "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(RATE)
        wf.writeframes(pcm)
    print(f"wrote {OUT / name} ({len(pcm)} bytes)")


if __name__ == "__main__":
    _write("short_answer.wav", _sweep(1.0, 220.0, 380.0))

"""Helpers to drive the pipeline from a WAV file, with no network transport.

`wav_to_frames` turns a mono 16-bit PCM WAV into the frame sequence that a real
VAD-driven transport would emit for one candidate answer.
"""

from __future__ import annotations

import wave
from pathlib import Path

from pipecat.frames.frames import (
    Frame,
    InputAudioRawFrame,
    UserStartedSpeakingFrame,
    UserStoppedSpeakingFrame,
    VADUserStartedSpeakingFrame,
    VADUserStoppedSpeakingFrame,
)

_CHUNK_MS = 20


def read_wav(path: str | Path) -> tuple[bytes, int]:
    with wave.open(str(path), "rb") as wf:
        if wf.getnchannels() != 1 or wf.getsampwidth() != 2:
            raise ValueError(f"{path}: expected mono 16-bit PCM WAV")
        return wf.readframes(wf.getnframes()), wf.getframerate()


def wav_to_frames(path: str | Path) -> list[Frame]:
    """One speech segment: VAD start, audio chunks, VAD stop, user-turn stop."""
    pcm, rate = read_wav(path)
    chunk_bytes = int(rate * _CHUNK_MS / 1000) * 2

    frames: list[Frame] = [VADUserStartedSpeakingFrame(), UserStartedSpeakingFrame()]
    for off in range(0, len(pcm), chunk_bytes):
        frames.append(InputAudioRawFrame(pcm[off : off + chunk_bytes], rate, 1))
    frames.append(VADUserStoppedSpeakingFrame())
    frames.append(UserStoppedSpeakingFrame())
    return frames

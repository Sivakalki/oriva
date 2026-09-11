"""Piper TTS (local, fully offline). Needs `uv add "pipecat-ai[piper]"`."""

from pipelines.providers.piper.tts import build_tts

__all__ = ["build_tts"]

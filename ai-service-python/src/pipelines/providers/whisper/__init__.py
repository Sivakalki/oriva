"""Whisper STT (faster-whisper, local). Needs `uv add "pipecat-ai[whisper]"`."""

from pipelines.providers.whisper.stt import build_stt

__all__ = ["build_stt"]

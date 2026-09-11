"""Local Whisper STT via faster-whisper. Imports pipecat's whisper service
lazily so a missing `pipecat-ai[whisper]` extra only fails when this
provider is actually selected.
"""

from __future__ import annotations

from pipecat.services.stt_service import STTService

from config import STTConfig


def build_stt(cfg: STTConfig) -> STTService:
    from pipecat.services.whisper.stt import WhisperSTTService
    from pipecat.transcriptions.language import Language

    # CPU + int8: keeps the GPU free for the LLM, and faster-whisper's int8
    # CPU path is fast enough for live use at small/base model sizes.
    return WhisperSTTService(
        model=cfg.model,
        device="cpu",
        compute_type="int8",
        language=Language(cfg.language),
    )

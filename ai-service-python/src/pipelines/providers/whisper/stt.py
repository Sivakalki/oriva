"""Local Whisper STT via faster-whisper. Imports pipecat's whisper service
lazily so a missing `pipecat-ai[whisper]` extra only fails when this
provider is actually selected.
"""

from __future__ import annotations

from loguru import logger
from pipecat.services.stt_service import STTService

from config import STTConfig


def _resolve_device(cfg: STTConfig) -> tuple[str, str]:
    """Pick a (device, compute_type) pair that's actually valid together.

    CTranslate2 compute types are device-specific -- "int8_float16" (the
    memory-conscious GPU option) errors outright on CPU, so a naive
    device="auto" + a fixed compute_type would crash the moment CUDA isn't
    available. Do the CUDA check ourselves instead: on CPU, always force
    plain "int8" (fast, broadly supported) regardless of cfg.compute_type,
    which only applies to the GPU path.
    """
    if cfg.device == "cpu":
        return "cpu", "int8"

    import ctranslate2  # type: ignore[import-untyped]  # no stubs/py.typed published

    has_cuda = ctranslate2.get_cuda_device_count() > 0
    if cfg.device == "cuda" and not has_cuda:
        logger.warning("stt.device=cuda requested but no CUDA device found -- using CPU")

    if has_cuda and cfg.device in ("cuda", "auto"):
        return "cuda", cfg.compute_type
    return "cpu", "int8"


def build_stt(cfg: STTConfig) -> STTService:
    from pipecat.services.whisper.stt import WhisperSTTService
    from pipecat.transcriptions.language import Language

    device, compute_type = _resolve_device(cfg)
    logger.info("whisper stt: model={} device={} compute_type={}", cfg.model, device, compute_type)
    return WhisperSTTService(
        model=cfg.model,
        device=device,
        compute_type=compute_type,
        language=Language(cfg.language),
    )

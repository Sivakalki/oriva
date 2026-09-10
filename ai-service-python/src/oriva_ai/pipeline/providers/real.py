"""Real provider factories. Each imports its Pipecat service lazily so a missing
optional dependency only fails when that provider is actually selected.
"""

from __future__ import annotations

from pathlib import Path
from typing import cast

from pipecat.services.llm_service import LLMService
from pipecat.services.stt_service import STTService
from pipecat.services.tts_service import TTSService

from oriva_ai.config import LLMConfig, STTConfig, TTSConfig

# Where lazily-downloaded models land (Piper voices, etc.).
_MODEL_DIR = Path.home() / ".cache" / "oriva-ai" / "models"


def whisper_stt(cfg: STTConfig) -> STTService:
    from pipecat.services.whisper.stt import WhisperSTTService

    return WhisperSTTService(model=cfg.model)


def openai_llm(cfg: LLMConfig) -> LLMService:
    from pipecat.services.openai.llm import OpenAILLMService

    return cast(
        LLMService,
        OpenAILLMService(model=cfg.model, base_url=cfg.base_url, api_key=cfg.api_key),
    )


def piper_tts(cfg: TTSConfig) -> TTSService:
    from pipecat.services.piper.tts import PiperTTSService

    _MODEL_DIR.mkdir(parents=True, exist_ok=True)
    # The voice model (~60MB) is downloaded into download_dir on first use.
    return PiperTTSService(
        settings=PiperTTSService.Settings(voice=cfg.voice),
        download_dir=_MODEL_DIR,
    )

"""Real provider factories. Each imports its Pipecat service lazily so a missing
optional dependency only fails when that provider is actually selected.
"""

from __future__ import annotations

from typing import cast

from pipecat.services.llm_service import LLMService
from pipecat.services.stt_service import STTService
from pipecat.services.tts_service import TTSService

from oriva_ai.config import LLMConfig, STTConfig, TTSConfig


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

    return PiperTTSService(voice=cfg.voice)

"""Provider registries: stage -> {name -> factory}.

Real factories live in ``real`` and import their SDK lazily; importing this
module does not import any optional dependency.
"""

from __future__ import annotations

from collections.abc import Callable

from pipecat.services.llm_service import LLMService
from pipecat.services.stt_service import STTService
from pipecat.services.tts_service import TTSService

from oriva_ai.config import LLMConfig, STTConfig, TTSConfig
from oriva_ai.pipeline.providers import mock, real

STTFactory = Callable[[STTConfig], STTService]
LLMFactory = Callable[[LLMConfig], LLMService]
TTSFactory = Callable[[TTSConfig], TTSService]

STT_PROVIDERS: dict[str, STTFactory] = {
    "mock": mock.build_stt,
    "whisper": real.whisper_stt,
}

LLM_PROVIDERS: dict[str, LLMFactory] = {
    "mock": mock.build_llm,
    "openai": real.openai_llm,
    "litellm": real.openai_llm,
}

TTS_PROVIDERS: dict[str, TTSFactory] = {
    "mock": mock.build_tts,
    "piper": real.piper_tts,
}

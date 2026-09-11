"""Provider registries: stage -> {name -> factory}.

Each provider lives in its own subpackage (mock/, whisper/, piper/, openai/)
and imports its SDK lazily inside its factory function, so importing this
module (and therefore every provider subpackage) does not pull in any
optional dependency. Adding a new provider is: drop a new subpackage next to
these, exposing a `build_stt`/`build_llm`/`build_tts` factory, and add one
line to the matching dict below — no other code changes.
"""

from __future__ import annotations

from collections.abc import Callable

from pipecat.services.llm_service import LLMService
from pipecat.services.stt_service import STTService
from pipecat.services.tts_service import TTSService

from config import LLMConfig, STTConfig, TTSConfig
from pipelines.providers import mock, openai, piper, whisper

STTFactory = Callable[[STTConfig], STTService]
LLMFactory = Callable[[LLMConfig], LLMService]
TTSFactory = Callable[[TTSConfig], TTSService]

STT_PROVIDERS: dict[str, STTFactory] = {
    "mock": mock.build_stt,
    "whisper": whisper.build_stt,
}

LLM_PROVIDERS: dict[str, LLMFactory] = {
    "mock": mock.build_llm,
    "openai": openai.build_llm,
    "litellm": openai.build_llm,
}

TTS_PROVIDERS: dict[str, TTSFactory] = {
    "mock": mock.build_tts,
    "piper": piper.build_tts,
}

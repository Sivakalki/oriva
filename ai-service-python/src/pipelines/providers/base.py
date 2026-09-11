"""Resolve a config section to a Pipecat service via the stage registry."""

from __future__ import annotations

from pipecat.services.llm_service import LLMService
from pipecat.services.stt_service import STTService
from pipecat.services.tts_service import TTSService

from config import LLMConfig, STTConfig, TTSConfig
from pipelines.providers.registry import (
    LLM_PROVIDERS,
    STT_PROVIDERS,
    TTS_PROVIDERS,
)


class ProviderError(RuntimeError):
    """Base class for provider resolution failures."""


class UnknownProvider(ProviderError):
    """The configured provider name is not in the registry."""


class ProviderNotInstalled(ProviderError):
    """The provider is known but its optional dependency is not installed."""


def _resolve[T](stage: str, registry: dict[str, T], name: str) -> T:
    try:
        return registry[name]
    except KeyError:
        known = ", ".join(sorted(registry)) or "(none)"
        raise UnknownProvider(f"unknown {stage} provider {name!r}; known: {known}") from None


def build_stt(cfg: STTConfig) -> STTService:
    factory = _resolve("stt", STT_PROVIDERS, cfg.provider)
    try:
        return factory(cfg)
    except (ImportError, ModuleNotFoundError) as exc:
        raise ProviderNotInstalled(
            f'{exc}. Install it with: uv add "pipecat-ai[{cfg.provider}]"'
        ) from exc


def build_llm(cfg: LLMConfig) -> LLMService:
    factory = _resolve("llm", LLM_PROVIDERS, cfg.provider)
    try:
        return factory(cfg)
    except (ImportError, ModuleNotFoundError) as exc:
        raise ProviderNotInstalled(f'{exc}. Install it with: uv add "pipecat-ai[openai]"') from exc


def build_tts(cfg: TTSConfig) -> TTSService:
    factory = _resolve("tts", TTS_PROVIDERS, cfg.provider)
    try:
        return factory(cfg)
    except (ImportError, ModuleNotFoundError) as exc:
        raise ProviderNotInstalled(
            f'{exc}. Install it with: uv add "pipecat-ai[{cfg.provider}]"'
        ) from exc

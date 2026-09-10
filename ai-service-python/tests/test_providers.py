from __future__ import annotations

import pytest

from oriva_ai.config import LLMConfig, STTConfig, TTSConfig
from oriva_ai.pipeline.providers import (
    ProviderNotInstalled,
    UnknownProvider,
    build_llm,
    build_stt,
    build_tts,
)
from oriva_ai.pipeline.providers import mock as mock_mod
from oriva_ai.pipeline.providers.mock import MockLLMService, MockSTTService, MockTTSService


def test_mock_resolves() -> None:
    assert isinstance(build_stt(STTConfig(provider="mock")), MockSTTService)
    assert isinstance(build_llm(LLMConfig(provider="mock")), MockLLMService)
    assert isinstance(build_tts(TTSConfig(provider="mock")), MockTTSService)


def test_unknown_provider() -> None:
    with pytest.raises(UnknownProvider, match="unknown stt provider 'nope'"):
        build_stt(STTConfig(provider="nope"))


def test_missing_extra_becomes_provider_not_installed(monkeypatch: pytest.MonkeyPatch) -> None:
    def boom(_cfg: object) -> object:
        raise ModuleNotFoundError("No module named 'faster_whisper'")

    monkeypatch.setitem(
        __import__(
            "oriva_ai.pipeline.providers.registry", fromlist=["STT_PROVIDERS"]
        ).STT_PROVIDERS,
        "whisper",
        boom,
    )
    with pytest.raises(ProviderNotInstalled, match="pipecat-ai\\[whisper\\]"):
        build_stt(STTConfig(provider="whisper"))


def test_mock_llm_last_user_text() -> None:
    from pipecat.processors.aggregators.llm_context import LLMContext

    ctx = LLMContext(
        messages=[
            {"role": "system", "content": "sys"},
            {"role": "user", "content": "I built payment systems."},
        ]
    )
    assert mock_mod._last_user_text(ctx) == "I built payment systems."

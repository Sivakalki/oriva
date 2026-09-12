from __future__ import annotations

import sys
import types

import pytest

from config import STTConfig
from pipelines.providers.whisper.stt import _resolve_device


def _fake_ctranslate2(cuda_count: int) -> types.ModuleType:
    mod = types.ModuleType("ctranslate2")
    mod.get_cuda_device_count = lambda: cuda_count  # type: ignore[attr-defined]
    return mod


def test_device_cpu_always_forces_plain_int8(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setitem(sys.modules, "ctranslate2", _fake_ctranslate2(cuda_count=2))
    assert _resolve_device(STTConfig(device="cpu", compute_type="int8_float16")) == ("cpu", "int8")


def test_device_auto_uses_cuda_when_available(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setitem(sys.modules, "ctranslate2", _fake_ctranslate2(cuda_count=1))
    assert _resolve_device(STTConfig(device="auto", compute_type="int8_float16")) == (
        "cuda",
        "int8_float16",
    )


def test_device_auto_falls_back_to_cpu_without_cuda(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setitem(sys.modules, "ctranslate2", _fake_ctranslate2(cuda_count=0))
    assert _resolve_device(STTConfig(device="auto", compute_type="int8_float16")) == ("cpu", "int8")


def test_device_cuda_requested_but_unavailable_falls_back_to_cpu(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setitem(sys.modules, "ctranslate2", _fake_ctranslate2(cuda_count=0))
    assert _resolve_device(STTConfig(device="cuda", compute_type="int8_float16")) == ("cpu", "int8")


def test_device_cuda_available_uses_configured_compute_type(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setitem(sys.modules, "ctranslate2", _fake_ctranslate2(cuda_count=1))
    assert _resolve_device(STTConfig(device="cuda", compute_type="float16")) == ("cuda", "float16")

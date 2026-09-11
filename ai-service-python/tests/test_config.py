from __future__ import annotations

import textwrap
from pathlib import Path

import pytest
from pydantic import ValidationError

from config import Settings, get_settings, load_settings


def test_defaults() -> None:
    s = Settings()
    assert s.app.name == "oriva-ai"
    assert s.stt.provider == "mock"
    assert s.llm.base_url == "http://localhost:4000"
    assert s.pipeline.sample_rate == 16000
    assert s.postgres.readonly is True


def test_yaml_overrides_nested_value(tmp_path: Path) -> None:
    cfg = tmp_path / "config.yaml"
    cfg.write_text(
        textwrap.dedent(
            """
            llm:
              model: anthropic/claude-sonnet-5
            server:
              port: 9999
            """
        )
    )
    s = load_settings(cfg)
    assert s.llm.model == "anthropic/claude-sonnet-5"
    assert s.server.port == 9999
    assert s.stt.provider == "mock"  # untouched


def test_env_overrides_yaml(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    cfg = tmp_path / "config.yaml"
    cfg.write_text("llm:\n  model: from-yaml\n")
    monkeypatch.setenv("ORIVA_AI__LLM__MODEL", "from-env")
    s = load_settings(cfg)
    assert s.llm.model == "from-env"


def test_unknown_key_rejected(tmp_path: Path) -> None:
    cfg = tmp_path / "config.yaml"
    cfg.write_text("bogus: 1\n")
    with pytest.raises(ValidationError):
        load_settings(cfg)


def test_missing_file_is_ok(tmp_path: Path) -> None:
    s = load_settings(tmp_path / "nope.yaml")
    assert s.app.name == "oriva-ai"


def test_get_settings_is_cached() -> None:
    assert get_settings() is get_settings()


def test_committed_config_yaml_loads() -> None:
    s = load_settings("config.yaml")
    assert s.metrics_enabled is True
    assert s.mcp.server_url.endswith("/mcp")

from __future__ import annotations

import dataclasses

from fastapi.testclient import TestClient

from oriva_ai.app import create_app
from oriva_ai.config import Settings


def test_health(settings: Settings) -> None:
    with TestClient(create_app(settings)) as client:
        r = client.get("/health")
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "ok"
    assert body["service"] == "oriva-ai"
    assert "version" in body


def test_metrics_exposes_stage_histograms(settings: Settings) -> None:
    with TestClient(create_app(settings)) as client:
        r = client.get("/metrics")
    assert r.status_code == 200
    assert r.headers["content-type"].startswith("text/plain")
    assert "oriva_stt_latency_seconds" in r.text
    assert "oriva_voice_to_voice_seconds" in r.text


def test_metrics_disabled_returns_404(settings: Settings) -> None:
    disabled = settings.model_copy(update={"metrics_enabled": False})
    with TestClient(create_app(disabled)) as client:
        r = client.get("/metrics")
    assert r.status_code == 404


def test_settings_attached_to_app_state(settings: Settings) -> None:
    app = create_app(settings)
    assert app.state.settings is settings
    # sanity: Settings is a normal pydantic model, not a dataclass
    assert not dataclasses.is_dataclass(settings)

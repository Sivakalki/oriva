from __future__ import annotations

import dataclasses

import pytest
from fastapi.testclient import TestClient
from starlette.websockets import WebSocketDisconnect

from app import create_app
from config import Settings


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


def test_ws_route_registered(settings: Settings) -> None:
    app = create_app(settings)
    # app.routes no longer flattens routes included via router.include_router
    # (they resolve lazily); url_path_for is the stable way to check a named
    # route is registered regardless of how deeply it's nested.
    assert app.url_path_for("ws") == "/ws"


def test_lifespan_builds_pipeline(settings: Settings) -> None:
    with TestClient(create_app(settings)) as client:
        client.get("/health")
        assert client.app.state.pipeline is not None  # type: ignore[attr-defined]


def test_ws_requires_session_id_when_mcp_enabled(settings: Settings) -> None:
    assert settings.mcp.enabled
    with TestClient(create_app(settings)) as client:
        with pytest.raises(WebSocketDisconnect) as exc:
            with client.websocket_connect("/ws"):
                pass
    assert exc.value.code == 1008


def test_ws_token_closed_phase_rejected(
    settings: Settings, monkeypatch: pytest.MonkeyPatch
) -> None:
    from pipelines.join_lookup import Resolved

    async def fake_resolve(_base: str, _token: str) -> Resolved:
        return Resolved(session_id="s1", phase="closed")

    monkeypatch.setattr("pipelines.session.resolve_token", fake_resolve)
    with TestClient(create_app(settings)) as client:
        with pytest.raises(WebSocketDisconnect) as exc:
            with client.websocket_connect("/ws?token=whatever"):
                pass
    assert exc.value.code == 1008


def test_ws_token_invalid_rejected(settings: Settings, monkeypatch: pytest.MonkeyPatch) -> None:
    from pipelines.join_lookup import TokenNotFound

    async def fake_resolve(_base: str, _token: str) -> object:
        raise TokenNotFound("nope")

    monkeypatch.setattr("pipelines.session.resolve_token", fake_resolve)
    with TestClient(create_app(settings)) as client:
        with pytest.raises(WebSocketDisconnect) as exc:
            with client.websocket_connect("/ws?token=nope"):
                pass
    assert exc.value.code == 1008

"""Business logic for one `/ws` connection: resolve the candidate's join
token (or a bare session_id) into an interview session, reject the
connection if it's invalid or the interview has ended, then build and run
the pipeline for it.

The api/v1 layer only registers the route and hands us the raw WebSocket —
everything else (token resolution, close codes, pipeline assembly, running
the session) lives here so it stays testable without FastAPI in the loop.
"""

from __future__ import annotations

from fastapi import WebSocket
from loguru import logger

from config import Settings
from pipelines.assembly import build_session_pipeline
from pipelines.join_lookup import TokenNotFound, resolve_token, start_session
from pipelines.runner import PipelineSession
from transport.websocket import build_transport

_WS_POLICY_VIOLATION = 1008


async def _resolve_session_id(
    websocket: WebSocket, settings: Settings, token: str | None, session_id: str | None
) -> str | None:
    """Returns the session id to use, or None after closing the socket."""
    if token:
        try:
            resolved = await resolve_token(settings.backend.base_url, token)
        except TokenNotFound:
            await websocket.close(code=_WS_POLICY_VIOLATION, reason="invalid interview link")
            return None
        except Exception as exc:  # noqa: BLE001 — backend unreachable etc.
            logger.warning("join token resolve failed: {}", exc)
            await websocket.close(code=_WS_POLICY_VIOLATION, reason="could not verify link")
            return None
        if resolved.phase == "closed":
            await websocket.close(code=_WS_POLICY_VIOLATION, reason="interview has ended")
            return None
        session_id = resolved.session_id

    if settings.mcp.enabled and not session_id:
        await websocket.close(
            code=_WS_POLICY_VIOLATION, reason="token or session_id query param required"
        )
        return None

    return session_id or ""


async def handle_ws_connection(websocket: WebSocket, settings: Settings) -> None:
    """Connect with `/ws?token=<join token>` (preferred) — the interview
    session and phase are resolved from backend-go. `/ws?session_id=<id>` is
    kept for tests. No telephony (docs/PLAN.md Phase 1).
    """
    token = websocket.query_params.get("token")
    session_id = websocket.query_params.get("session_id")

    session_id = await _resolve_session_id(websocket, settings, token, session_id)
    if session_id is None:
        return

    if token:
        # Best-effort: drive the session's state machine to "in_progress" now
        # that the call is actually starting. Never blocks or fails the call
        # on a transient backend hiccup — a stuck state just means a later
        # refresh of the candidate's join page mis-reports the phase, which
        # is recoverable, whereas dropping the call over this would not be.
        try:
            await start_session(settings.backend.base_url, token)
        except Exception as exc:  # noqa: BLE001 — backend unreachable etc.
            logger.warning("join session start failed | token={} error={}", token, exc)

    await websocket.accept()
    build = await build_session_pipeline(settings, session_id)
    transport = build_transport(websocket)

    logger.info("ws client connected | session_id={}", session_id)
    try:
        await PipelineSession(build, transport).run()
    finally:
        logger.info("ws client disconnected | session_id={}", session_id)

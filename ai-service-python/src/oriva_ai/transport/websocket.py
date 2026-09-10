"""Local WebSocket transport: one Pipecat session per `/ws` connection.

Connect with `/ws?token=<join token>` (preferred) — the ai-service resolves the
interview session and phase from backend-go. `/ws?session_id=<id>` is kept for
tests. No telephony (docs/PLAN.md Phase 1).
"""

from __future__ import annotations

from fastapi import FastAPI, WebSocket
from loguru import logger
from pipecat.serializers.protobuf import ProtobufFrameSerializer
from pipecat.transports.websocket.fastapi import (
    FastAPIWebsocketParams,
    FastAPIWebsocketTransport,
)

from oriva_ai.config import Settings
from oriva_ai.pipeline import PipelineSession
from oriva_ai.pipeline.assembly import build_session_pipeline
from oriva_ai.pipeline.join_lookup import TokenNotFound, resolve_token

_WS_POLICY_VIOLATION = 1008


def register_ws_route(app: FastAPI) -> None:
    @app.websocket("/ws")
    async def ws(websocket: WebSocket) -> None:
        settings: Settings = app.state.settings
        token = websocket.query_params.get("token")
        session_id = websocket.query_params.get("session_id")

        if token:
            try:
                resolved = await resolve_token(settings.backend.base_url, token)
            except TokenNotFound:
                await websocket.close(code=_WS_POLICY_VIOLATION, reason="invalid interview link")
                return
            except Exception as exc:  # noqa: BLE001 — backend unreachable etc.
                logger.warning("join token resolve failed: {}", exc)
                await websocket.close(code=_WS_POLICY_VIOLATION, reason="could not verify link")
                return
            if resolved.phase == "closed":
                await websocket.close(code=_WS_POLICY_VIOLATION, reason="interview has ended")
                return
            session_id = resolved.session_id

        if settings.mcp.enabled and not session_id:
            await websocket.close(
                code=_WS_POLICY_VIOLATION, reason="token or session_id query param required"
            )
            return

        await websocket.accept()
        build = await build_session_pipeline(settings, session_id or "")

        params = FastAPIWebsocketParams(
            audio_in_enabled=True,
            audio_out_enabled=True,
            add_wav_header=False,
            serializer=ProtobufFrameSerializer(),
        )
        transport = FastAPIWebsocketTransport(websocket, params)

        logger.info("ws client connected | session_id={}", session_id)
        try:
            await PipelineSession(build, transport).run()
        finally:
            logger.info("ws client disconnected | session_id={}", session_id)

"""Local WebSocket transport: one Pipecat session per `/ws` connection.

No telephony — this is the bake-off's local transport (docs/PLAN.md Phase 1).
Connect with `/ws?session_id=<interview session id>`; the id is bound to every
MCP tool call the LLM makes during the interview.
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

_WS_POLICY_VIOLATION = 1008


def register_ws_route(app: FastAPI) -> None:
    @app.websocket("/ws")
    async def ws(websocket: WebSocket) -> None:
        settings: Settings = app.state.settings
        session_id = websocket.query_params.get("session_id")
        if settings.mcp.enabled and not session_id:
            await websocket.close(
                code=_WS_POLICY_VIOLATION, reason="session_id query param required"
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

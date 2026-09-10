"""Local WebSocket transport: one Pipecat session per `/ws` connection.

No telephony — this is the bake-off's local transport (docs/PLAN.md Phase 1).
"""

from __future__ import annotations

from fastapi import FastAPI, WebSocket
from loguru import logger
from pipecat.serializers.protobuf import ProtobufFrameSerializer
from pipecat.transports.websocket.fastapi import (
    FastAPIWebsocketParams,
    FastAPIWebsocketTransport,
)

from oriva_ai.pipeline import PipelineBuild, PipelineSession


def register_ws_route(app: FastAPI) -> None:
    @app.websocket("/ws")
    async def ws(websocket: WebSocket) -> None:
        await websocket.accept()
        build: PipelineBuild = app.state.pipeline

        # Pipecat 1.8 runs turn detection (VAD + smart-turn) inside the user
        # context aggregator; assembly.make_task wires it from settings.pipeline.vad.
        params = FastAPIWebsocketParams(
            audio_in_enabled=True,
            audio_out_enabled=True,
            add_wav_header=False,
            serializer=ProtobufFrameSerializer(),
        )
        transport = FastAPIWebsocketTransport(websocket, params)

        logger.info("ws client connected")
        try:
            await PipelineSession(build, transport).run()
        finally:
            logger.info("ws client disconnected")

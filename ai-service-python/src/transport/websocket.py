"""Pure transport construction for a `/ws` connection — protobuf framing,
audio-in/out enabled. No business logic (session resolution, pipeline
assembly) lives here; that's pipelines/session.py.
"""

from __future__ import annotations

from fastapi import WebSocket
from pipecat.serializers.protobuf import ProtobufFrameSerializer
from pipecat.transports.websocket.fastapi import (
    FastAPIWebsocketParams,
    FastAPIWebsocketTransport,
)


def build_transport(websocket: WebSocket) -> FastAPIWebsocketTransport:
    params = FastAPIWebsocketParams(
        audio_in_enabled=True,
        audio_out_enabled=True,
        add_wav_header=False,
        serializer=ProtobufFrameSerializer(),
    )
    return FastAPIWebsocketTransport(websocket, params)

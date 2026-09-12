"""Compatibility shim for @pipecat-ai/websocket-transport's protobuf client.

The official npm client (confirmed by reading its bundled source --
node_modules/@pipecat-ai/websocket-transport/dist/index.js, v1.7.1, the
latest published version as of this writing, not a stale-package issue)
only implements two `oneofKind` branches in its protobuf deserializer:
"audio" and "message". Pipecat's Python-side ProtobufFrameSerializer,
however, also advertises "text", "transcription", and "interruption" as
serializable frame types -- and FastAPIWebsocketTransport's own output
processor explicitly serializes and sends an InterruptionFrame on every
barge-in (pipecat/transports/websocket/fastapi.py's process_frame). Any of
those three reaching the client crashes its message handler with "Unknown
frame kind" and kills the connection -- confirmed live: this is exactly
what happened on the first real candidate barge-in during live testing
(real STT + real audio, not mock).

This is a genuine gap between the two officially-paired packages, not
something this pipeline's own construction causes -- InterruptionFrame is
core interruption-handling behavior in every Pipecat + websocket-transport
pairing, not specific to the queue subsystem. Until the JS client adds
support, drop these three frame types immediately before transport.output()
so they never reach the crashing serialize() call. Interruption
cancellation itself still works correctly upstream (in the TTS service,
aggregators, etc., which all run before this filter) -- only the
client-facing "you were interrupted" notification is lost, which the
client doesn't need anyway: its audio stream simply stops.
"""

from __future__ import annotations

from pipecat.frames.frames import Frame, InterruptionFrame, TextFrame, TranscriptionFrame
from pipecat.processors.frame_processor import FrameDirection, FrameProcessor

# Exact types only (not isinstance): matches ProtobufFrameSerializer's own
# exact-type dict lookup, so e.g. TTSTextFrame (a TextFrame subclass) is left
# alone here -- the serializer already silently drops any non-exact-match
# type with a log warning instead of crashing, so it's not a risk on its own.
_UNSUPPORTED_BY_CLIENT: tuple[type[Frame], ...] = (InterruptionFrame, TextFrame, TranscriptionFrame)


class WebsocketClientCompatFilter(FrameProcessor):
    """Drops frame types the current websocket-transport client can't
    deserialize, right before they'd reach the transport's output."""

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        if type(frame) in _UNSUPPORTED_BY_CLIENT:
            return
        await self.push_frame(frame, direction)

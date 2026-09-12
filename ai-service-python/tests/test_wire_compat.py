from __future__ import annotations

from pipecat.frames.frames import (
    InterruptionFrame,
    OutputAudioRawFrame,
    TextFrame,
    TranscriptionFrame,
)
from pipecat.tests.utils import run_test

from pipelines.wire_compat import WebsocketClientCompatFilter


async def test_drops_frame_kinds_the_client_cannot_deserialize() -> None:
    frames = [
        InterruptionFrame(),
        TranscriptionFrame("hello", "user-1", "2026-01-01T00:00:00Z"),
        TextFrame("some text"),
    ]
    down, _ = await run_test(WebsocketClientCompatFilter(), frames_to_send=frames)
    assert down == []


async def test_passes_through_everything_else() -> None:
    audio = OutputAudioRawFrame(audio=b"\x00\x00", sample_rate=16000, num_channels=1)
    down, _ = await run_test(WebsocketClientCompatFilter(), frames_to_send=[audio])
    assert down == [audio]


async def test_passes_through_tts_text_frame_subclass() -> None:
    # TTSTextFrame IS-A TextFrame but is not the *exact* type dropped above --
    # ProtobufFrameSerializer itself already silently drops non-exact matches
    # with a log warning (not a crash), so there's no need to filter it here.
    from pipecat.frames.frames import TTSTextFrame

    frame = TTSTextFrame("spoken text", aggregated_by="tts")
    down, _ = await run_test(WebsocketClientCompatFilter(), frames_to_send=[frame])
    assert down == [frame]

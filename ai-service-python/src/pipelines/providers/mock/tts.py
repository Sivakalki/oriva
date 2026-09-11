"""Zero-dependency mock TTS. Yields silence sized to the text length —
deterministic, no network, no models.
"""

from __future__ import annotations

import asyncio
from collections.abc import AsyncGenerator

from pipecat.frames.frames import Frame, TTSAudioRawFrame
from pipecat.services.settings import TTSSettings
from pipecat.services.tts_service import TTSService

from config import TTSConfig

_TTS_TTFA_S = 0.04


def _tts_settings() -> TTSSettings:
    return TTSSettings(model="mock", voice="mock", language=None)


class MockTTSService(TTSService):
    """Yields silence sized to the text length."""

    def __init__(self) -> None:
        super().__init__(settings=_tts_settings())

    async def run_tts(self, text: str, context_id: str) -> AsyncGenerator[Frame | None, None]:
        await self.start_tts_usage_metrics(text)
        rate = self.sample_rate or 16000
        chunk = b"\x00" * (int(rate * 0.02) * 2)
        for i in range(max(1, len(text.split()) * 3)):
            if i == 0:
                await asyncio.sleep(_TTS_TTFA_S)
                await self.stop_ttfb_metrics()
            yield TTSAudioRawFrame(chunk, rate, 1)


def build_tts(cfg: TTSConfig) -> MockTTSService:
    return MockTTSService()

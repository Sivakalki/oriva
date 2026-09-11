"""Zero-dependency mock STT. Deterministic, no network, no models — emits a
fixed transcript so the whole pipeline (and the metrics observer) can be
exercised end to end with no external services or API keys.
"""

from __future__ import annotations

import asyncio
from collections.abc import AsyncGenerator
from datetime import UTC, datetime

from pipecat.frames.frames import Frame, TranscriptionFrame
from pipecat.services.settings import STTSettings
from pipecat.services.stt_service import SegmentedSTTService

from config import STTConfig

# A small synthetic delay so the pipeline produces a non-zero stage metric.
_STT_DELAY_S = 0.03


def _now_iso() -> str:
    return datetime.now(UTC).isoformat()


def _stt_settings() -> STTSettings:
    return STTSettings(model="mock", language=None)


class MockSTTService(SegmentedSTTService):
    """Emits a fixed transcript for every speech segment."""

    def __init__(self, transcript: str) -> None:
        super().__init__(settings=_stt_settings())
        self._transcript = transcript

    @property
    def wants_wav_segments(self) -> bool:
        return False

    async def run_stt(self, audio: bytes) -> AsyncGenerator[Frame | None, None]:
        await self.start_ttfb_metrics()
        await asyncio.sleep(_STT_DELAY_S)
        await self.stop_ttfb_metrics()
        yield TranscriptionFrame(self._transcript, "", _now_iso())


def build_stt(cfg: STTConfig) -> MockSTTService:
    return MockSTTService(transcript=cfg.mock_transcript)

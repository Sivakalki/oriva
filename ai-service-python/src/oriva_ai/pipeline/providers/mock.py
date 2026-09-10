"""Zero-dependency mock STT / LLM / TTS services.

Deterministic, no network, no models. They still emit Pipecat TTFB metrics and
push unhandled frames downstream, so the whole pipeline (and the metrics
observer) is exercised end to end.
"""

from __future__ import annotations

import asyncio
from collections.abc import AsyncGenerator
from datetime import UTC, datetime

from pipecat.frames.frames import (
    Frame,
    LLMContextFrame,
    LLMFullResponseEndFrame,
    LLMFullResponseStartFrame,
    LLMTextFrame,
    TranscriptionFrame,
    TTSAudioRawFrame,
)
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.frame_processor import FrameDirection
from pipecat.services.llm_service import LLMService
from pipecat.services.settings import LLMSettings, STTSettings, TTSSettings
from pipecat.services.stt_service import SegmentedSTTService
from pipecat.services.tts_service import TTSService

from oriva_ai.config import LLMConfig, STTConfig, TTSConfig

# Small synthetic delays so the pipeline produces non-zero stage metrics.
_STT_DELAY_S = 0.03
_LLM_TTFT_S = 0.08
_TTS_TTFA_S = 0.04


def _now_iso() -> str:
    return datetime.now(UTC).isoformat()


def _stt_settings() -> STTSettings:
    return STTSettings(model="mock", language=None)


def _llm_settings() -> LLMSettings:
    return LLMSettings(
        model="mock",
        system_instruction=None,
        temperature=None,
        max_tokens=None,
        top_p=None,
        top_k=None,
        frequency_penalty=None,
        presence_penalty=None,
        seed=None,
        filter_incomplete_user_turns=None,
        user_turn_completion_config=None,
    )


def _tts_settings() -> TTSSettings:
    return TTSSettings(model="mock", voice="mock", language=None)


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


class MockLLMService(LLMService):
    """Streams a templated reply built from the last user message."""

    def __init__(self, reply_template: str) -> None:
        super().__init__(settings=_llm_settings())
        self._template = reply_template

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        if isinstance(frame, LLMContextFrame):
            await self._respond(frame.context)
        else:
            await self.push_frame(frame, direction)

    async def _respond(self, context: LLMContext) -> None:
        reply = self._template.format(user=_last_user_text(context))
        await self.push_frame(LLMFullResponseStartFrame())
        await self.start_processing_metrics()
        await self.start_ttfb_metrics()
        first = True
        for word in reply.split():
            if first:
                await asyncio.sleep(_LLM_TTFT_S)
                await self.stop_ttfb_metrics()
                first = False
            await self.push_frame(LLMTextFrame(word + " "))
        await self.stop_processing_metrics()
        await self.push_frame(LLMFullResponseEndFrame())


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


def _last_user_text(context: LLMContext) -> str:
    for message in reversed(context.get_messages()):
        if isinstance(message, dict) and message.get("role") == "user":
            content = message.get("content", "")
            if isinstance(content, str):
                return content
            if isinstance(content, list):
                return " ".join(
                    part.get("text", "")
                    for part in content
                    if isinstance(part, dict) and part.get("type") == "text"
                )
    return ""


def build_stt(cfg: STTConfig) -> MockSTTService:
    return MockSTTService(transcript=cfg.mock_transcript)


def build_llm(cfg: LLMConfig) -> MockLLMService:
    return MockLLMService(reply_template=cfg.mock_reply_template)


def build_tts(cfg: TTSConfig) -> MockTTSService:
    return MockTTSService()

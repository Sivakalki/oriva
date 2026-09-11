"""Zero-dependency mock LLM. Streams a templated reply built from the last
user message — deterministic, no network, no API keys.
"""

from __future__ import annotations

import asyncio

from pipecat.frames.frames import (
    Frame,
    LLMContextFrame,
    LLMFullResponseEndFrame,
    LLMFullResponseStartFrame,
    LLMTextFrame,
)
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.frame_processor import FrameDirection
from pipecat.services.llm_service import LLMService
from pipecat.services.settings import LLMSettings

from config import LLMConfig

_LLM_TTFT_S = 0.08


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


def build_llm(cfg: LLMConfig) -> MockLLMService:
    return MockLLMService(reply_template=cfg.mock_reply_template)

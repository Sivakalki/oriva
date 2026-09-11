"""A deterministic offline assembly for the harness and tests.

The live ``/ws`` pipeline uses Pipecat's universal context aggregator, whose
turn-completion heuristics assume a real VAD/transport. Offline we replace the
user side with an explicit bridge: every finalized transcript becomes a user
message and triggers one LLM run. The assistant side still uses the real
aggregator so replies are recorded back into the context.
"""

from __future__ import annotations

import time

from pipecat.frames.frames import Frame, LLMContextFrame, TranscriptionFrame
from pipecat.pipeline.pipeline import Pipeline
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.aggregators.llm_response_universal import LLMContextAggregatorPair
from pipecat.processors.frame_processor import FrameDirection, FrameProcessor

from pipelines.assembly import PipelineBuild


class TranscriptToContextBridge(FrameProcessor):
    """On each TranscriptionFrame: append a user message and emit LLMContextFrame."""

    def __init__(self, context: LLMContext) -> None:
        super().__init__()
        self._context = context

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        await self.push_frame(frame, direction)
        if isinstance(frame, TranscriptionFrame) and frame.text.strip():
            self._context.add_message({"role": "user", "content": frame.text})
            await self.push_frame(LLMContextFrame(self._context), direction)


class Collector(FrameProcessor):
    """Records every frame that reaches it, with a wall-clock timestamp."""

    def __init__(self) -> None:
        super().__init__()
        self.events: list[tuple[float, Frame]] = []

    @property
    def frames(self) -> list[Frame]:
        return [f for _, f in self.events]

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        self.events.append((time.perf_counter(), frame))
        await self.push_frame(frame, direction)


def build_offline_pipeline(
    build: PipelineBuild,
    *,
    post_llm: FrameProcessor | None = None,
    post_tts: FrameProcessor | None = None,
) -> Pipeline:
    assistant = LLMContextAggregatorPair(build.context).assistant()
    stages: list[FrameProcessor] = [
        build.stt,
        TranscriptToContextBridge(build.context),
        build.llm,
    ]
    if post_llm is not None:
        stages.append(post_llm)
    stages.append(build.tts)
    if post_tts is not None:
        stages.append(post_tts)
    stages.append(assistant)
    return Pipeline(stages)

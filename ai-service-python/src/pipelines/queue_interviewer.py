"""Deterministic, Python-driven question pacing for a live interview.

Replaces letting the live conversational LLM decide what to ask and when:
this pipeline stage owns a fixed queue of pre-generated questions
(question_queue.py) and speaks the current one directly via TTS -- no LLM
round trip needed just to ask a question that's already known. After each
candidate answer it classifies confirmed-vs-repeat-request heuristically
(answer_classifier.py, also LLM-free) and either advances the queue (record
the turn, pop, ask the next one) or re-asks the same question verbatim.

Before the generated questions, the session runs through two fixed stages
so the candidate never sits in silence while the question queue is being
built: a "how are you" greeting (candidate's answer only decides which of
two fixed reassurance lines to speak next, via a keyword heuristic -- see
answer_classifier.classify_mood), then a self-introduction question, which
is recorded like any other turn. Only then does the pre-generated queue
start.

Positioned in the live pipeline between the user context aggregator and the
LLM node (see assembly.py's make_task): it intercepts every LLMContextFrame
the aggregator emits (the candidate's answer, once their turn is detected as
complete) and never forwards it to the LLM -- the LLM node stays wired in the
pipeline but is simply never invoked while this stage is active.
"""

from __future__ import annotations

import time
from collections import deque
from typing import Any, Literal

from loguru import logger
from pipecat.frames.frames import EndWorkerFrame, Frame, LLMContextFrame, StartFrame, TTSSpeakFrame
from pipecat.processors.frame_processor import FrameDirection, FrameProcessor
from pipecat.services.mcp_service import MCPClient

from pipelines.answer_classifier import classify_answer, classify_mood
from pipelines.mcp_tools import call_tool

_Stage = Literal["greeting", "self_intro", "main"]


def _last_user_text(messages: list[Any]) -> str:
    """The most recent user message's text, or "" if there isn't one."""
    for message in reversed(messages):
        if not isinstance(message, dict) or message.get("role") != "user":
            continue
        content = message.get("content")
        if isinstance(content, str):
            return content
        if isinstance(content, list):
            # Some providers represent content as a list of typed parts
            # (e.g. [{"type": "text", "text": "..."}]) rather than a bare str.
            parts = [p.get("text", "") for p in content if isinstance(p, dict)]
            return " ".join(p for p in parts if p)
        return ""
    return ""


class QueueInterviewer(FrameProcessor):
    """Owns the greeting, self-intro, and question queue for one session."""

    def __init__(
        self,
        mcp_client: MCPClient,
        questions: list[str],
        *,
        duration_minutes: int,
        wrap_up_text: str,
        candidate_name: str = "",
        greeting_template: str = "Hi {name}, thanks for joining today! How are you doing?",
        mood_positive_reaction: str = "That's great to hear!",
        mood_negative_reaction: str = "I'm sorry to hear that -- no worries at all.",
        self_intro_question: str = "Let's get started -- tell me about yourself.",
    ) -> None:
        super().__init__()
        self._mcp = mcp_client
        self._queue: deque[str] = deque(questions)
        self._duration_seconds = max(duration_minutes, 1) * 60
        self._wrap_up_text = wrap_up_text
        self._deadline: float | None = None
        self._done = False

        self._stage: _Stage = "greeting"
        self._greeting_text = greeting_template.format(name=candidate_name or "there")
        self._mood_positive = mood_positive_reaction
        self._mood_negative = mood_negative_reaction
        self._self_intro_question = self_intro_question

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)

        if isinstance(frame, StartFrame):
            await self.push_frame(frame, direction)
            self._deadline = time.monotonic() + self._duration_seconds
            await self.push_frame(TTSSpeakFrame(text=self._greeting_text), direction)
            return

        if isinstance(frame, LLMContextFrame) and not self._done:
            # Swallowed at every stage: the candidate's answer is handled
            # here, never forwarded to the LLM node further down the pipeline.
            if self._stage == "greeting":
                await self._handle_greeting_answer(frame, direction)
            elif self._stage == "self_intro":
                await self._handle_self_intro_answer(frame, direction)
            else:
                await self._handle_answer(frame, direction)
            return

        await self.push_frame(frame, direction)

    async def _handle_greeting_answer(
        self, frame: LLMContextFrame, direction: FrameDirection
    ) -> None:
        answer = _last_user_text(frame.context.get_messages())
        reaction = (
            self._mood_positive
            if classify_mood(answer) == "positive"
            else self._mood_negative
        )
        await self.push_frame(TTSSpeakFrame(text=reaction), direction)
        await self.push_frame(TTSSpeakFrame(text=self._self_intro_question), direction)
        self._stage = "self_intro"

    async def _handle_self_intro_answer(
        self, frame: LLMContextFrame, direction: FrameDirection
    ) -> None:
        answer = _last_user_text(frame.context.get_messages())
        if classify_answer(answer) == "repeat":
            logger.debug("queue: re-asking self-intro, candidate asked for a repeat")
            await self.push_frame(TTSSpeakFrame(text=self._self_intro_question), direction)
            return

        try:
            await call_tool(
                self._mcp, "record_turn", question=self._self_intro_question, answer=answer
            )
        except Exception as exc:  # noqa: BLE001 -- never block the interview on this
            logger.warning("queue: record_turn (self-intro) failed, continuing anyway: {}", exc)

        self._stage = "main"
        await self._ask_current(direction)

    async def _ask_current(self, direction: FrameDirection) -> None:
        if not self._queue or self._time_up():
            await self._wrap_up(direction)
            return
        await self.push_frame(TTSSpeakFrame(text=self._queue[0]), direction)

    async def _handle_answer(self, frame: LLMContextFrame, direction: FrameDirection) -> None:
        answer = _last_user_text(frame.context.get_messages())

        if classify_answer(answer) == "repeat":
            logger.debug("queue: re-asking, candidate asked for a repeat")
            await self.push_frame(TTSSpeakFrame(text=self._queue[0]), direction)
            return

        question = self._queue[0]
        try:
            await call_tool(self._mcp, "record_turn", question=question, answer=answer)
        except Exception as exc:  # noqa: BLE001 -- never block the interview on this
            logger.warning("queue: record_turn failed, continuing anyway: {}", exc)

        self._queue.popleft()
        await self._ask_current(direction)

    def _time_up(self) -> bool:
        return self._deadline is not None and time.monotonic() >= self._deadline

    async def _wrap_up(self, direction: FrameDirection) -> None:
        if self._done:
            return
        self._done = True
        logger.info("queue: wrapping up interview")
        await self.push_frame(TTSSpeakFrame(text=self._wrap_up_text), direction)
        try:
            await call_tool(
                self._mcp, "advance_state", to_state="completed", reason="interview finished"
            )
            await call_tool(
                self._mcp, "advance_state", to_state="scoring", reason="interview finished"
            )
        except Exception as exc:  # noqa: BLE001 -- the call still ends gracefully either way
            logger.warning("queue: advance_state failed: {}", exc)
        # Queued ahead of this (the wrap-up TTSSpeakFrame) flush before the
        # pipeline actually ends -- see EndWorkerFrame's docstring.
        await self.push_frame(EndWorkerFrame(reason="interview queue exhausted"), direction)

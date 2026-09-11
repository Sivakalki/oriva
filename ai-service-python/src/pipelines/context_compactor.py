"""Background context compaction for a live interview session.

A long interview's LLM context grows every turn. Left unbounded it eventually
blows the LLM's context window and slows every subsequent call. This module
summarizes the older half of the conversation once it passes a word-count
threshold (pipeline.context_compress_words), keeping the most recent turns
verbatim.

Compression runs as a background asyncio task, triggered right after each
assistant turn completes -- the natural gap while the candidate is still
speaking or thinking is exactly when it should finish, so it's off the
critical path for "how fast can the next question go out" (docs/PLAN.md
Phase 2). It is *not* fully race-proof: if the candidate replies unusually
fast, in principle the live pipeline could read the context while this task
is mid-swap. In practice the summarization call (network round trip) so
outlasts the reaction window that this hasn't been observed to matter, and
add_message-vs-set_messages ordering means a torn read is a stale summary at
worst, never a crash or corrupted state.
"""

from __future__ import annotations

import asyncio
import time
from typing import Any

import httpx
from loguru import logger
from pipecat.frames.frames import Frame, LLMFullResponseEndFrame
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.frame_processor import FrameDirection, FrameProcessor

from config import Settings
from telemetry import CONTEXT_COMPRESS_SECONDS, CONTEXT_COMPRESSIONS, CONTEXT_WORDS

# Most recent messages (across both roles) kept verbatim, never summarized --
# roughly the last 3 Q&A exchanges, so the model still has the immediate
# thread of conversation word-for-word.
_KEEP_RECENT_MESSAGES = 6

_SUMMARIZE_PROMPT = (
    "Summarize this portion of a job interview conversation in 2-4 sentences. "
    "Keep concrete facts the candidate stated (technologies, years of experience, "
    "project names, numbers) -- drop filler and small talk."
)


def _word_count(context: LLMContext) -> int:
    total = 0
    for message in context.get_messages():
        content = message.get("content") if isinstance(message, dict) else None
        if isinstance(content, str):
            total += len(content.split())
    return total


class ContextCompactor:
    """Owns background compression for one session's LLMContext."""

    def __init__(self, context: LLMContext, settings: Settings) -> None:
        self._context = context
        self._settings = settings
        self._lock = asyncio.Lock()
        self._compressing = False

    def maybe_schedule(self) -> None:
        """Call after each assistant turn. Fires a background compress task
        if the context has grown past the configured threshold; a no-op if
        one is already running (the next turn will check again).
        """
        words = _word_count(self._context)
        CONTEXT_WORDS.observe(words)
        threshold = self._settings.pipeline.context_compress_words
        if threshold <= 0 or words < threshold or self._compressing:
            return
        self._compressing = True
        asyncio.create_task(self._compress(words))

    async def _compress(self, words_before: int) -> None:
        start = time.perf_counter()
        try:
            async with self._lock:
                messages = self._context.get_messages()
                has_system = (
                    bool(messages)
                    and isinstance(messages[0], dict)
                    and messages[0].get("role") == "system"
                )
                body = messages[1:] if has_system else messages
                if len(body) <= _KEEP_RECENT_MESSAGES:
                    return  # not enough history yet to be worth summarizing

                head, tail = body[:-_KEEP_RECENT_MESSAGES], body[-_KEEP_RECENT_MESSAGES:]
                summary = await self._summarize(head)

                new_messages: list[Any] = [messages[0]] if has_system else []
                new_messages.append(
                    {"role": "system", "content": f"[Summary of earlier conversation]: {summary}"}
                )
                new_messages.extend(tail)
                self._context.set_messages(new_messages)

            CONTEXT_COMPRESSIONS.inc()
            logger.info(
                "context compressed | words_before={} words_after={} elapsed_ms={:.0f}",
                words_before,
                _word_count(self._context),
                (time.perf_counter() - start) * 1000,
            )
        except Exception as exc:  # noqa: BLE001 -- best-effort background job, never crash the call
            logger.warning("context compression failed: {}", exc)
        finally:
            CONTEXT_COMPRESS_SECONDS.observe(time.perf_counter() - start)
            self._compressing = False

    async def _summarize(self, messages: list[Any]) -> str:
        cfg = self._settings.llm
        if cfg.provider == "mock":
            # No real endpoint to call -- a fast deterministic stand-in keeps
            # the mock pipeline (and tests) network-free.
            return f"({len(messages)} earlier messages omitted)"

        transcript = "\n".join(
            f"{m.get('role', '?')}: {m.get('content')}"
            for m in messages
            if isinstance(m.get("content"), str)
        )
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(
                f"{cfg.base_url.rstrip('/')}/chat/completions",
                headers={"Authorization": f"Bearer {cfg.api_key}"},
                json={
                    "model": cfg.model,
                    "messages": [
                        {"role": "system", "content": _SUMMARIZE_PROMPT},
                        {"role": "user", "content": transcript},
                    ],
                    "temperature": 0.2,
                },
            )
            resp.raise_for_status()
            data = resp.json()
            return str(data["choices"][0]["message"]["content"]).strip()


class ContextCompactionTrigger(FrameProcessor):
    """Pipeline stage: checks whether the context needs compressing after
    each assistant turn. Never blocks frame flow -- every frame is pushed
    downstream immediately; compression (if triggered) runs as a detached
    background task.
    """

    def __init__(self, compactor: ContextCompactor) -> None:
        super().__init__()
        self._compactor = compactor

    async def process_frame(self, frame: Frame, direction: FrameDirection) -> None:
        await super().process_frame(frame, direction)
        await self.push_frame(frame, direction)
        if isinstance(frame, LLMFullResponseEndFrame):
            self._compactor.maybe_schedule()

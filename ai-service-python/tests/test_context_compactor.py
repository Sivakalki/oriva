from __future__ import annotations

import pytest
from pipecat.processors.aggregators.llm_context import LLMContext

from config import Settings
from pipelines.context_compactor import ContextCompactor, _word_count


def _settings(threshold: int) -> Settings:
    s = Settings()
    s.pipeline.context_compress_words = threshold
    return s


def _context_with(n_turns: int, words_per_message: int = 20) -> LLMContext:
    messages = [{"role": "system", "content": "You are Oriva."}]
    for i in range(n_turns):
        messages.append(
            {"role": "user", "content": " ".join(f"w{i}" for _ in range(words_per_message))}
        )
        messages.append(
            {"role": "assistant", "content": " ".join(f"a{i}" for _ in range(words_per_message))}
        )
    return LLMContext(messages=messages)


def test_word_count() -> None:
    ctx = LLMContext(messages=[{"role": "system", "content": "one two three"}])
    assert _word_count(ctx) == 3


def test_maybe_schedule_below_threshold_does_nothing(monkeypatch: pytest.MonkeyPatch) -> None:
    ctx = _context_with(n_turns=1)
    compactor = ContextCompactor(ctx, _settings(threshold=10_000))

    called = False

    def fake_create_task(coro):  # noqa: ANN001
        nonlocal called
        called = True
        coro.close()

    monkeypatch.setattr("pipelines.context_compactor.asyncio.create_task", fake_create_task)
    compactor.maybe_schedule()
    assert called is False


def test_maybe_schedule_disabled_by_zero_threshold() -> None:
    ctx = _context_with(n_turns=50)  # plenty of words
    compactor = ContextCompactor(ctx, _settings(threshold=0))
    # threshold=0 disables compaction outright; maybe_schedule must not raise
    # and must not flip the in-flight flag.
    compactor.maybe_schedule()
    assert compactor._compressing is False  # noqa: SLF001


async def test_compress_summarizes_head_keeps_recent_tail() -> None:
    ctx = _context_with(n_turns=10, words_per_message=5)  # well over a small threshold
    settings = _settings(threshold=1)  # force compression regardless of size
    compactor = ContextCompactor(ctx, settings)

    words_before = _word_count(ctx)
    await compactor._compress(words_before)  # noqa: SLF001

    messages = ctx.get_messages()
    assert messages[0]["role"] == "system"
    assert messages[0]["content"] == "You are Oriva."
    assert messages[1]["role"] == "system"
    assert "Summary of earlier conversation" in messages[1]["content"]
    # 6 most recent messages (3 Q&A pairs) kept verbatim after the summary.
    assert len(messages) == 2 + 6
    assert messages[-1]["content"] == "a9 a9 a9 a9 a9"
    assert _word_count(ctx) < words_before
    assert compactor._compressing is False  # noqa: SLF001 -- flag cleared after finishing


async def test_compress_noop_when_not_enough_history() -> None:
    ctx = _context_with(n_turns=1)  # 1 system + 2 messages: nothing worth compressing
    compactor = ContextCompactor(ctx, _settings(threshold=1))
    before = ctx.get_messages()

    await compactor._compress(_word_count(ctx))  # noqa: SLF001

    assert ctx.get_messages() == before

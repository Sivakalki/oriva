from __future__ import annotations

from typing import Any

from pipecat.frames.frames import EndWorkerFrame, LLMContextFrame, TTSSpeakFrame
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.tests.utils import run_test

from pipelines.queue_interviewer import QueueInterviewer


class _FakeContent:
    def __init__(self, text: str | None) -> None:
        self.text = text


class _FakeResult:
    def __init__(
        self, *, structured: dict[str, Any] | None = None, is_error: bool = False
    ) -> None:
        self.structuredContent = structured
        self.isError = is_error
        self.content: list[_FakeContent] = []
        if is_error:
            self.content = [_FakeContent("boom")]


class _FakeSession:
    def __init__(self) -> None:
        self.calls: list[tuple[str, dict[str, Any]]] = []

    async def call_tool(self, name: str, arguments: dict[str, Any]) -> _FakeResult:
        self.calls.append((name, arguments))
        return _FakeResult(structured={"ok": True})


class _FakeMCPClient:
    """Stands in for pipecat's MCPClient -- QueueInterviewer only touches the
    two attributes call_tool() reads (via mcp_tools.call_tool)."""

    def __init__(self, session: _FakeSession) -> None:
        self._session = session
        self._tools_arguments: dict[str, dict[str, Any]] = {
            "record_turn": {"session_id": "s1"},
            "advance_state": {"session_id": "s1"},
        }

    def _ensure_connected(self) -> _FakeSession:
        return self._session


def _context_with_answer(text: str) -> LLMContext:
    ctx = LLMContext(messages=[{"role": "system", "content": "sys"}])
    ctx.add_message({"role": "user", "content": text})
    return ctx


async def test_asks_first_question_on_start() -> None:
    session = _FakeSession()
    interviewer = QueueInterviewer(
        _FakeMCPClient(session), ["Q1", "Q2"], duration_minutes=30, wrap_up_text="bye"
    )
    down, _ = await run_test(interviewer, frames_to_send=[])
    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == ["Q1"]


async def test_confirmed_answer_records_and_advances_queue() -> None:
    session = _FakeSession()
    interviewer = QueueInterviewer(
        _FakeMCPClient(session), ["Q1", "Q2"], duration_minutes=30, wrap_up_text="bye"
    )
    ctx = _context_with_answer("I have five years of backend experience with Go.")
    down, _ = await run_test(interviewer, frames_to_send=[LLMContextFrame(ctx)])

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == ["Q1", "Q2"]
    assert session.calls[0][0] == "record_turn"
    assert session.calls[0][1]["question"] == "Q1"
    assert session.calls[0][1]["answer"] == "I have five years of backend experience with Go."


async def test_repeat_request_reasks_same_question_without_recording() -> None:
    session = _FakeSession()
    interviewer = QueueInterviewer(
        _FakeMCPClient(session), ["Q1", "Q2"], duration_minutes=30, wrap_up_text="bye"
    )
    ctx = _context_with_answer("sorry, can you repeat that")
    down, _ = await run_test(interviewer, frames_to_send=[LLMContextFrame(ctx)])

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == ["Q1", "Q1"]
    assert session.calls == []


async def test_queue_exhausted_speaks_wrap_up_and_ends_pipeline() -> None:
    session = _FakeSession()
    interviewer = QueueInterviewer(
        _FakeMCPClient(session), ["Q1"], duration_minutes=30, wrap_up_text="thanks, bye"
    )
    ctx = _context_with_answer("I have five years of backend experience with Go.")
    down, _ = await run_test(
        interviewer, frames_to_send=[LLMContextFrame(ctx)], send_end_frame=False
    )

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == ["Q1", "thanks, bye"]
    assert any(isinstance(f, EndWorkerFrame) for f in down)
    assert [c[0] for c in session.calls] == ["record_turn", "advance_state", "advance_state"]
    assert session.calls[1][1]["to_state"] == "completed"
    assert session.calls[2][1]["to_state"] == "scoring"


async def test_record_turn_failure_does_not_block_the_queue() -> None:
    class _FailingSession(_FakeSession):
        async def call_tool(self, name: str, arguments: dict[str, Any]) -> _FakeResult:
            await super().call_tool(name, arguments)
            if name == "record_turn":
                return _FakeResult(is_error=True)
            return _FakeResult(structured={"ok": True})

    session = _FailingSession()
    interviewer = QueueInterviewer(
        _FakeMCPClient(session), ["Q1", "Q2"], duration_minutes=30, wrap_up_text="bye"
    )
    ctx = _context_with_answer("I have five years of backend experience with Go.")
    down, _ = await run_test(interviewer, frames_to_send=[LLMContextFrame(ctx)])

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == ["Q1", "Q2"]  # still advances despite the failed record_turn call

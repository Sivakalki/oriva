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


def _answer(text: str) -> LLMContextFrame:
    return LLMContextFrame(_context_with_answer(text))


GREETING = "Hi Jane, thanks for joining today! How are you doing?"
POSITIVE_REACTION = "Great!"
NEGATIVE_REACTION = "No worries, relax."
SELF_INTRO_Q = "Tell me about yourself."

DEFAULT_KWARGS: dict[str, Any] = dict(
    duration_minutes=30,
    wrap_up_text="bye",
    candidate_name="Jane",
    greeting_template="Hi {name}, thanks for joining today! How are you doing?",
    mood_positive_reaction=POSITIVE_REACTION,
    mood_negative_reaction=NEGATIVE_REACTION,
    self_intro_question=SELF_INTRO_Q,
)


def _make(session: _FakeSession, questions: list[str]) -> QueueInterviewer:
    return QueueInterviewer(_FakeMCPClient(session), questions, **DEFAULT_KWARGS)


def _past_intro_frames(self_intro_answer: str = "I'm a backend engineer.") -> list[LLMContextFrame]:
    """Frames that carry a session from StartFrame through greeting and
    self-intro, landing it at the start of the main question queue."""
    return [_answer("doing well, thanks"), _answer(self_intro_answer)]


# --- Greeting stage ---


async def test_greeting_spoken_on_start() -> None:
    session = _FakeSession()
    interviewer = _make(session, ["Q1"])
    down, _ = await run_test(interviewer, frames_to_send=[])
    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING]


async def test_greeting_falls_back_to_generic_name() -> None:
    session = _FakeSession()
    kwargs = {**DEFAULT_KWARGS, "candidate_name": ""}
    interviewer = QueueInterviewer(_FakeMCPClient(session), ["Q1"], **kwargs)
    down, _ = await run_test(interviewer, frames_to_send=[])
    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == ["Hi there, thanks for joining today! How are you doing?"]


async def test_positive_mood_reaction_then_self_intro_question() -> None:
    session = _FakeSession()
    interviewer = _make(session, ["Q1"])
    down, _ = await run_test(interviewer, frames_to_send=[_answer("I'm doing great, thanks!")])
    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING, POSITIVE_REACTION, SELF_INTRO_Q]
    assert session.calls == []  # no record_turn yet -- still small talk


async def test_negative_mood_reaction_then_self_intro_question() -> None:
    session = _FakeSession()
    interviewer = _make(session, ["Q1"])
    down, _ = await run_test(interviewer, frames_to_send=[_answer("a bit nervous honestly")])
    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING, NEGATIVE_REACTION, SELF_INTRO_Q]


# --- Self-intro stage ---


async def test_self_intro_confirmed_answer_records_and_starts_main_queue() -> None:
    session = _FakeSession()
    interviewer = _make(session, ["Q1", "Q2"])
    frames = _past_intro_frames("I'm a backend engineer with five years of experience.")
    down, _ = await run_test(interviewer, frames_to_send=frames)

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING, POSITIVE_REACTION, SELF_INTRO_Q, "Q1"]
    assert session.calls[0][0] == "record_turn"
    assert session.calls[0][1]["question"] == SELF_INTRO_Q
    assert session.calls[0][1]["answer"] == "I'm a backend engineer with five years of experience."


async def test_self_intro_repeat_request_reasks_without_recording() -> None:
    session = _FakeSession()
    interviewer = _make(session, ["Q1"])
    frames = [_answer("doing well"), _answer("sorry, can you repeat that")]
    down, _ = await run_test(interviewer, frames_to_send=frames)

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING, POSITIVE_REACTION, SELF_INTRO_Q, SELF_INTRO_Q]
    assert session.calls == []


# --- Main question queue (unchanged behaviour, now reached after intro) ---


async def test_confirmed_answer_records_and_advances_queue() -> None:
    session = _FakeSession()
    interviewer = _make(session, ["Q1", "Q2"])
    frames = _past_intro_frames() + [_answer("I have five years of backend experience with Go.")]
    down, _ = await run_test(interviewer, frames_to_send=frames)

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING, POSITIVE_REACTION, SELF_INTRO_Q, "Q1", "Q2"]
    assert session.calls[1][0] == "record_turn"
    assert session.calls[1][1]["question"] == "Q1"
    assert session.calls[1][1]["answer"] == "I have five years of backend experience with Go."


async def test_repeat_request_reasks_same_question_without_recording() -> None:
    session = _FakeSession()
    interviewer = _make(session, ["Q1", "Q2"])
    frames = _past_intro_frames() + [_answer("sorry, can you repeat that")]
    down, _ = await run_test(interviewer, frames_to_send=frames)

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING, POSITIVE_REACTION, SELF_INTRO_Q, "Q1", "Q1"]
    assert len(session.calls) == 1  # only the self-intro's record_turn


async def test_queue_exhausted_speaks_wrap_up_and_ends_pipeline() -> None:
    session = _FakeSession()
    interviewer = _make(session, ["Q1"])
    frames = _past_intro_frames() + [_answer("I have five years of backend experience with Go.")]
    down, _ = await run_test(interviewer, frames_to_send=frames, send_end_frame=False)

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING, POSITIVE_REACTION, SELF_INTRO_Q, "Q1", "bye"]
    assert any(isinstance(f, EndWorkerFrame) for f in down)
    call_names = [c[0] for c in session.calls]
    assert call_names == ["record_turn", "record_turn", "advance_state", "advance_state"]
    assert session.calls[2][1]["to_state"] == "completed"
    assert session.calls[3][1]["to_state"] == "scoring"


async def test_record_turn_failure_does_not_block_the_queue() -> None:
    class _FailingSession(_FakeSession):
        async def call_tool(self, name: str, arguments: dict[str, Any]) -> _FakeResult:
            await super().call_tool(name, arguments)
            if name == "record_turn" and arguments.get("question") == "Q1":
                return _FakeResult(is_error=True)
            return _FakeResult(structured={"ok": True})

    session = _FailingSession()
    interviewer = QueueInterviewer(_FakeMCPClient(session), ["Q1", "Q2"], **DEFAULT_KWARGS)
    frames = _past_intro_frames() + [_answer("I have five years of backend experience with Go.")]
    down, _ = await run_test(interviewer, frames_to_send=frames)

    speaks = [f.text for f in down if isinstance(f, TTSSpeakFrame)]
    assert speaks == [GREETING, POSITIVE_REACTION, SELF_INTRO_Q, "Q1", "Q2"]  # still advances

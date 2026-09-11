from __future__ import annotations

import pytest

from pipelines.answer_classifier import classify_answer


@pytest.mark.parametrize(
    "text",
    [
        "I have about five years of backend experience with Go and Postgres.",
        "Sure. I led a migration from a monolith to microservices last year.",
    ],
)
def test_real_answers_are_confirmed(text: str) -> None:
    assert classify_answer(text) == "confirmed"


@pytest.mark.parametrize(
    "text",
    [
        "",
        "   ",
        "what",
        "Sorry, could you repeat that?",
        "Can you say that again?",
        "One more time please",
        "Pardon?",
        "I didn't catch that",
        "SAY IT AGAIN",
    ],
)
def test_silence_and_repeat_requests_are_flagged(text: str) -> None:
    assert classify_answer(text) == "repeat"

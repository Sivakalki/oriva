from __future__ import annotations

import pytest

from pipelines.answer_classifier import classify_answer, classify_mood


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


@pytest.mark.parametrize(
    "text",
    [
        "I'm doing great, thanks!",
        "Pretty good, excited to be here.",
        "Fine, thank you.",
        "",  # empty/unclear answer defaults to positive, not negative
    ],
)
def test_positive_mood_is_the_default(text: str) -> None:
    assert classify_mood(text) == "positive"


@pytest.mark.parametrize(
    "text",
    [
        "a bit nervous honestly",
        "I'm not doing great today",
        "sorry, kind of tired",
        "stressed, to be honest",
    ],
)
def test_negative_mood_needs_an_explicit_match(text: str) -> None:
    assert classify_mood(text) == "negative"

"""Heuristic answer-confirmation classifier for the question queue.

Deliberately *not* an LLM call: the whole point of the queue subsystem is to
take turn-pacing decisions off the unreliable live conversational model, so
this is plain deterministic text matching instead. False positives are
possible (an answer that happens to contain "again") but are an accepted
trade-off for v1 -- worst case the candidate is re-asked a question they
already answered, which is recoverable and far less disruptive than the
LLM-reliability bugs this subsystem replaces.
"""

from __future__ import annotations

import re
from typing import Literal

Classification = Literal["confirmed", "repeat"]

# Substring match on the normalized transcript. Kept short/generic on
# purpose -- these are the phrases a candidate actually says when they
# missed the question, not an exhaustive NLU model.
_REPEAT_PHRASES: tuple[str, ...] = (
    "repeat that",
    "repeat the question",
    "say that again",
    "say it again",
    "one more time",
    "come again",
    "didn't catch",
    "did not catch",
    "didn't hear",
    "did not hear",
    "what was the question",
    "could you repeat",
    "can you repeat",
    "pardon",
    "sorry what",
    "sorry, what",
    "sorry could you",
)

# A real answer is very rarely under this many words; shorter than this
# (including empty/silence) is treated the same as an explicit repeat
# request rather than as a confirmed (near-)empty answer.
_MIN_ANSWER_WORDS = 2

_WHITESPACE_RE = re.compile(r"\s+")


def classify_answer(text: str) -> Classification:
    normalized = _WHITESPACE_RE.sub(" ", text.strip().lower())
    if not normalized:
        return "repeat"
    if len(normalized.split()) < _MIN_ANSWER_WORDS:
        return "repeat"
    if any(phrase in normalized for phrase in _REPEAT_PHRASES):
        return "repeat"
    return "confirmed"

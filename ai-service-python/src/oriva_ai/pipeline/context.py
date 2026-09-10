"""The initial LLM context for an interview session.

Minimal in this slice — dynamic, JD/resume-informed question generation is a later
phase (docs/PLAN.md Phase 2).
"""

from __future__ import annotations

from pipecat.processors.aggregators.llm_context import LLMContext

from oriva_ai.config import Settings

_SYSTEM_PROMPT = (
    "You are Oriva, an AI technical interviewer conducting a live phone screen. "
    "Ask one question at a time. Listen to the candidate's answer and ask a natural "
    "follow-up based on what they actually said. Keep your turns short and spoken-style. "
    "Do not answer questions outside the scope of the interview."
)


def interview_context(settings: Settings) -> LLMContext:
    return LLMContext(
        messages=[
            {"role": "system", "content": _SYSTEM_PROMPT},
            {"role": "assistant", "content": settings.pipeline.greeting},
        ]
    )

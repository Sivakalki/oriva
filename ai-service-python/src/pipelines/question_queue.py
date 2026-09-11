"""Upfront question generation for the queue-driven interviewer.

One LLM call, made once at session start (not per-turn): given the job
description and resume from get_interview_plan, produce a fixed ordered list
of questions. This is the only place the live LLM's output feeds the
interview's control flow -- everything about *when* to ask, re-ask, or move
on is decided deterministically in Python from here on
(queue_interviewer.py), which is the whole point: the small conversational
model proved unreliable at pacing itself live (see git history -- 44s of
dead air followed by five questions in one breath), but a single upfront
batch call has none of that live-latency pressure.
"""

from __future__ import annotations

from typing import Any

import httpx
from loguru import logger

from config import Settings

DEFAULT_QUESTIONS: tuple[str, ...] = (
    "Tell me about your relevant experience for this role.",
    "Walk me through a challenging project you worked on recently.",
    "How do you approach debugging a production issue you've never seen before?",
    "Tell me about a time you disagreed with a technical decision. What did you do?",
    "What are you looking for in your next role?",
)

_GEN_SYSTEM_PROMPT = (
    "You are preparing questions for a live technical phone screen. Given the "
    "job description and the candidate's resume, write exactly {n} interview "
    "questions, one per line, no numbering or bullets, no extra commentary. "
    "Tailor them to the specific role and to what's actually on the resume, "
    "the way a real interviewer would prepare in advance."
)


async def generate_questions(settings: Settings, plan: dict[str, Any]) -> list[str]:
    """Returns an ordered list of questions for the interview.

    Falls back to DEFAULT_QUESTIONS -- never raises -- on a mock LLM, a
    network/parse failure, or an empty model response: a generic interview
    beats a call that can't start.
    """
    cfg = settings.llm
    n = settings.pipeline.question_count

    if cfg.provider == "mock":
        return list(DEFAULT_QUESTIONS[:n])

    job_title = plan.get("job_title") or ""
    job_description = plan.get("job_description") or ""
    resume = plan.get("candidate_resume") or ""
    user_content = (
        f"Job title: {job_title}\n\nJob description:\n{job_description}\n\n"
        f"Candidate resume:\n{resume}"
    )

    try:
        async with httpx.AsyncClient(timeout=20.0) as client:
            resp = await client.post(
                f"{cfg.base_url.rstrip('/')}/chat/completions",
                headers={"Authorization": f"Bearer {cfg.api_key}"},
                json={
                    "model": cfg.model,
                    "messages": [
                        {"role": "system", "content": _GEN_SYSTEM_PROMPT.format(n=n)},
                        {"role": "user", "content": user_content},
                    ],
                    "temperature": 0.4,
                },
            )
            resp.raise_for_status()
            data = resp.json()
            text = str(data["choices"][0]["message"]["content"])
    except Exception as exc:  # noqa: BLE001 -- question generation is best-effort
        logger.warning("question generation failed, using defaults: {}", exc)
        return list(DEFAULT_QUESTIONS[:n])

    questions = [line.strip(" -\t*") for line in text.splitlines()]
    questions = [q for q in questions if q]
    if not questions:
        logger.warning("question generation returned nothing usable, using defaults")
        return list(DEFAULT_QUESTIONS[:n])
    return questions[:n]

from __future__ import annotations

from collections.abc import Callable

import httpx
import pytest

from config import Settings
from pipelines.question_queue import DEFAULT_QUESTIONS, generate_questions

Handler = Callable[[httpx.Request], httpx.Response]
_RealAsyncClient = httpx.AsyncClient


def _mock_client(handler: Handler) -> Callable[..., httpx.AsyncClient]:
    """A drop-in replacement for httpx.AsyncClient that routes through a
    MockTransport, for monkeypatching httpx.AsyncClient itself (generate_questions
    constructs its own client internally, unlike resolve_token's injectable
    client_factory)."""
    return lambda **kwargs: _RealAsyncClient(transport=httpx.MockTransport(handler))


def _real_llm_settings() -> Settings:
    s = Settings()
    s.llm.provider = "openai"
    return s


async def test_mock_provider_returns_defaults() -> None:
    questions = await generate_questions(Settings(), {"job_title": "Backend Engineer"})
    assert questions == list(DEFAULT_QUESTIONS[: Settings().pipeline.question_count])


async def test_real_provider_parses_one_question_per_line(monkeypatch: pytest.MonkeyPatch) -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "content": "What backend systems have you built?\n"
                            "- Tell me about a production incident you handled.\n"
                            "How do you approach code review?\n"
                        }
                    }
                ]
            },
        )

    monkeypatch.setattr(httpx, "AsyncClient", _mock_client(handler))
    questions = await generate_questions(_real_llm_settings(), {"job_title": "Backend Engineer"})
    assert questions == [
        "What backend systems have you built?",
        "Tell me about a production incident you handled.",
        "How do you approach code review?",
    ]


async def test_real_provider_falls_back_to_defaults_on_network_error(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def handler(_request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("connection refused")

    monkeypatch.setattr(httpx, "AsyncClient", _mock_client(handler))
    questions = await generate_questions(_real_llm_settings(), {})
    assert questions == list(DEFAULT_QUESTIONS[: Settings().pipeline.question_count])


async def test_real_provider_falls_back_to_defaults_on_empty_response(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def handler(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"choices": [{"message": {"content": "   \n  "}}]})

    monkeypatch.setattr(httpx, "AsyncClient", _mock_client(handler))
    questions = await generate_questions(_real_llm_settings(), {})
    assert questions == list(DEFAULT_QUESTIONS[: Settings().pipeline.question_count])

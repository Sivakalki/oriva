from __future__ import annotations

from collections.abc import Callable

import httpx
import pytest

from pipelines.join_lookup import TokenNotFound, resolve_token

Handler = Callable[[httpx.Request], httpx.Response]


def factory(handler: Handler) -> Callable[[], httpx.AsyncClient]:
    return lambda: httpx.AsyncClient(transport=httpx.MockTransport(handler))


async def test_resolve_ok() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/api/v1/join/tok-123"
        return httpx.Response(200, json={"session_id": "sess-9", "phase": "open"})

    r = await resolve_token("http://backend:8080", "tok-123", client_factory=factory(handler))
    assert r.session_id == "sess-9"
    assert r.phase == "open"


async def test_resolve_404() -> None:
    def handler(_r: httpx.Request) -> httpx.Response:
        return httpx.Response(404, json={"message": "invalid or expired interview link"})

    with pytest.raises(TokenNotFound):
        await resolve_token("http://backend:8080", "nope", client_factory=factory(handler))

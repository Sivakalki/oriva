"""Resolve a candidate join token to a session id via backend-go.

The candidate connects to ``/ws?token=<join_token>``; Go owns the mapping from
token to interview session and the interview's phase.
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass

import httpx


class TokenNotFound(Exception):
    """The join token is unknown or expired."""


@dataclass
class Resolved:
    session_id: str
    phase: str  # before | open | late | closed


ClientFactory = Callable[[], httpx.AsyncClient]


def _default_client() -> httpx.AsyncClient:
    return httpx.AsyncClient(timeout=5.0)


async def resolve_token(
    backend_base_url: str, token: str, *, client_factory: ClientFactory = _default_client
) -> Resolved:
    url = f"{backend_base_url.rstrip('/')}/api/v1/join/{token}"
    async with client_factory() as client:
        resp = await client.get(url)
    if resp.status_code == 404:
        raise TokenNotFound(token)
    resp.raise_for_status()
    body = resp.json()
    return Resolved(session_id=body["session_id"], phase=body["phase"])

from __future__ import annotations

import os
from typing import Any

import pytest

from config import MCPConfig
from pipelines.mcp_tools import SESSION_TOOLS, build_mcp_client, call_tool, load_tools


class _FakeContent:
    def __init__(self, text: str | None) -> None:
        self.text = text


class _FakeResult:
    def __init__(
        self,
        *,
        structured: dict[str, Any] | None = None,
        is_error: bool = False,
        text: str | None = None,
    ) -> None:
        self.structuredContent = structured
        self.isError = is_error
        self.content = [_FakeContent(text)] if text is not None else []


class _FakeSession:
    def __init__(self, result: _FakeResult) -> None:
        self.result = result
        self.calls: list[tuple[str, dict[str, Any]]] = []

    async def call_tool(self, name: str, arguments: dict[str, Any]) -> _FakeResult:
        self.calls.append((name, arguments))
        return self.result


class _FakeClient:
    def __init__(
        self, session: _FakeSession, fixed: dict[str, dict[str, Any]] | None = None
    ) -> None:
        self._session = session
        self._tools_arguments = fixed or {}

    def _ensure_connected(self) -> _FakeSession:
        return self._session


def test_session_tools_match_go_surface() -> None:
    assert SESSION_TOOLS == (
        "get_interview_plan",
        "record_turn",
        "advance_state",
    )


def test_build_mcp_client_wires_url_auth_and_session() -> None:
    cfg = MCPConfig(server_url="http://go:8080/mcp", auth_token="tok-abc")
    client = build_mcp_client(cfg, "sess-123")

    params = client._server_params  # noqa: SLF001
    assert params.url == "http://go:8080/mcp"
    assert params.headers == {"Authorization": "Bearer tok-abc"}

    assert client._tools_arguments == {  # noqa: SLF001
        t: {"session_id": "sess-123"} for t in SESSION_TOOLS
    }


async def test_call_tool_returns_structured_content() -> None:
    session = _FakeSession(_FakeResult(structured={"session_id": "s1", "state": "in_progress"}))
    client = _FakeClient(session, fixed={"get_interview_plan": {"session_id": "s1"}})

    out = await call_tool(client, "get_interview_plan")  # type: ignore[arg-type]

    assert out == {"session_id": "s1", "state": "in_progress"}
    assert session.calls == [("get_interview_plan", {"session_id": "s1"})]


async def test_call_tool_merges_fixed_arguments_over_explicit_ones() -> None:
    session = _FakeSession(_FakeResult(structured={"recorded": True}))
    client = _FakeClient(session, fixed={"record_turn": {"session_id": "s1"}})

    await call_tool(client, "record_turn", question="Q", answer="A")  # type: ignore[arg-type]

    assert session.calls == [("record_turn", {"question": "Q", "answer": "A", "session_id": "s1"})]


async def test_call_tool_falls_back_to_parsing_text_content() -> None:
    session = _FakeSession(_FakeResult(text='{"recorded": true, "turn_index": 2}'))
    client = _FakeClient(session)

    out = await call_tool(client, "record_turn", question="Q", answer="A")  # type: ignore[arg-type]

    assert out == {"recorded": True, "turn_index": 2}


async def test_call_tool_raises_on_tool_error() -> None:
    session = _FakeSession(_FakeResult(is_error=True, text="session not found"))
    client = _FakeClient(session)

    with pytest.raises(RuntimeError, match="session not found"):
        await call_tool(client, "get_interview_plan")  # type: ignore[arg-type]


@pytest.mark.integration
async def test_live_go_mcp_server() -> None:
    url = os.environ.get("ORIVA_GO_MCP_URL")
    if not url:
        pytest.skip("ORIVA_GO_MCP_URL not set")

    cfg = MCPConfig(
        server_url=url,
        auth_token=os.environ.get("ORIVA_GO_MCP_TOKEN", "dev-mcp-token"),
    )
    session_id = os.environ.get("ORIVA_GO_MCP_SESSION_ID", "00000000-0000-0000-0000-000000000000")

    client = build_mcp_client(cfg, session_id)
    try:
        schema = await load_tools(client)
        names = {t.name for t in schema.standard_tools}
        assert set(SESSION_TOOLS) <= names

        # session_id is injected via tools_arguments, so it is not advertised.
        for tool in schema.standard_tools:
            props = getattr(tool, "properties", None) or {}
            assert "session_id" not in props
    finally:
        await client.close()

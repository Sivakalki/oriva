from __future__ import annotations

import os

import pytest

from config import MCPConfig
from pipelines.mcp_tools import SESSION_TOOLS, build_mcp_client, load_tools


def test_session_tools_match_go_surface() -> None:
    assert SESSION_TOOLS == (
        "get_interview_plan",
        "retrieve_context",
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

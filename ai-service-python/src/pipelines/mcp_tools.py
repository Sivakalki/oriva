"""MCP client wiring: connect to the Go MCP server and load its tool schemas.

The Go server owns the schemas (``oriva.tools.v1``, docs/ARCHITECTURE.md §2).
Every tool takes a ``session_id``; we inject it via ``tools_arguments`` so the
LLM never sees or guesses it — Pipecat merges the fixed value into every call
and strips the parameter from the advertised schema.
"""

from __future__ import annotations

import json
from typing import Any

from mcp.client.session_group import StreamableHttpParameters
from pipecat.adapters.schemas.tools_schema import ToolsSchema
from pipecat.services.mcp_service import MCPClient

from config import MCPConfig

# The tool surface exposed by backend-go (mcp/server.go).
SESSION_TOOLS: tuple[str, ...] = (
    "get_interview_plan",
    "record_turn",
    "advance_state",
)


def build_mcp_client(cfg: MCPConfig, session_id: str) -> MCPClient:
    """Construct an MCPClient bound to one interview session."""
    return MCPClient(
        StreamableHttpParameters(
            url=cfg.server_url,
            headers={"Authorization": f"Bearer {cfg.auth_token}"},
        ),
        tools_arguments={tool: {"session_id": session_id} for tool in SESSION_TOOLS},
    )


async def load_tools(client: MCPClient) -> ToolsSchema:
    """Start the connection and return the tool schema (handlers attached)."""
    return await client.tools()


async def call_tool(client: MCPClient, name: str, **arguments: Any) -> dict[str, Any]:
    """Call an MCP tool directly, bypassing the LLM function-calling path.

    Used by the question-queue subsystem (pipelines/queue_interviewer.py),
    which drives get_interview_plan/record_turn/advance_state itself instead
    of leaving the live LLM to decide when to call them. `client` must
    already be connected (i.e. `tools()`/`start()` has been awaited).

    Fixed per-tool arguments (session_id, injected via `tools_arguments` in
    build_mcp_client) are merged in automatically, same as an LLM-triggered
    call -- callers never pass session_id themselves.

    Raises RuntimeError if the tool call itself failed (`result.isError`).
    """
    session = client._ensure_connected()  # noqa: SLF001 -- same session tools() opened
    fixed = client._tools_arguments.get(name)  # noqa: SLF001
    merged = {**arguments, **(fixed or {})}

    result = await session.call_tool(name, arguments=merged)
    if result.isError:
        text = "".join(c.text for c in result.content if hasattr(c, "text"))
        raise RuntimeError(f"mcp tool {name!r} failed: {text or 'unknown error'}")

    if result.structuredContent is not None:
        return result.structuredContent
    for content in result.content:
        content_text = getattr(content, "text", None)
        if content_text:
            parsed = json.loads(content_text)
            return parsed if isinstance(parsed, dict) else {}
    return {}

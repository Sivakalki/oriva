"""MCP client wiring: connect to the Go MCP server and load its tool schemas.

The Go server owns the schemas (``oriva.tools.v1``, docs/ARCHITECTURE.md §2).
Every tool takes a ``session_id``; we inject it via ``tools_arguments`` so the
LLM never sees or guesses it — Pipecat merges the fixed value into every call
and strips the parameter from the advertised schema.
"""

from __future__ import annotations

from mcp.client.session_group import StreamableHttpParameters
from pipecat.adapters.schemas.tools_schema import ToolsSchema
from pipecat.services.mcp_service import MCPClient

from config import MCPConfig

# The tool surface exposed by backend-go (mcp/server.go).
SESSION_TOOLS: tuple[str, ...] = (
    "get_interview_plan",
    "retrieve_context",
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

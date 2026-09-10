# AI Service — Slice 3 Design (MCP Client)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** ai-service slice 2 (`6cffe0c`), backend-go slice 4 (`f295dbd`, the Go MCP server)
**Scope:** Wire Pipecat's `MCPClient` to the Go MCP server so the LLM can call
`oriva.tools.v1` tools mid-interview. No frontend/telephony session routing, no
reconnect logic, no transcript rendering of tool calls.

## 1. Goal

`docs/ARCHITECTURE.md` §2: "Python's Pipecat pipeline runs an `MCPClient` that
connects to Go's MCP server, discovers the available tools, and hands their
schemas to the LLM so it can call them mid-conversation." The Go server (slice 4)
exposes four tools over Streamable HTTP behind a static bearer token. This slice
connects to it.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | Pipecat `MCPClient` + `StreamableHttpParameters(url, headers={Authorization})` | Matches the Go server's transport + static bearer auth |
| D2 | `session_id` injected via `MCPClient(tools_arguments=...)` | Pipecat merges these into every call and *removes them from the advertised schema* — the LLM never sees or guesses `session_id` |
| D3 | `MCPClient` is built per `/ws` connection, not at app startup | `session_id` is per interview; the connection lifetime is the call |
| D4 | `build_pipeline(settings)` stays sync (no MCP); new `async build_session_pipeline(settings, session_id)` adds MCP | App-startup validation and the offline harness don't need MCP; keeps that path dependency-free |
| D5 | `/ws` requires `?session_id=`; missing → close 1008 when `mcp.enabled` | The pipeline cannot record turns or advance state without knowing the session |
| D6 | Live cross-service test gated by `ORIVA_GO_MCP_URL` | Mirrors the Go side's `TEST_DATABASE_URL` pattern; unit tests cover the wiring |
| D7 | `pipecat-ai[mcp]` extra added (pulls the `mcp` python SDK) | Required for `MCPClient` |

## 3. Config (`config.py`)

`MCPConfig` gains:
```python
class MCPConfig(BaseModel):
    server_url: str = "http://localhost:8080/mcp"
    enabled: bool = True
    auth_token: str = "dev-mcp-token"   # matches backend-go dev default
```
`config.yaml`'s `mcp:` block gets `enabled` and `auth_token`.

## 4. `pipeline/mcp_tools.py` (new)

```python
from mcp import StreamableHttpParameters           # re-exported by the mcp SDK
from pipecat.services.mcp_service import MCPClient
from pipecat.adapters.schemas.tools_schema import ToolsSchema

SESSION_TOOLS = ("get_interview_plan", "retrieve_context", "record_turn", "advance_state")

def build_mcp_client(cfg: MCPConfig, session_id: str) -> MCPClient:
    return MCPClient(
        StreamableHttpParameters(
            url=cfg.server_url,
            headers={"Authorization": f"Bearer {cfg.auth_token}"},
        ),
        tools_arguments={t: {"session_id": session_id} for t in SESSION_TOOLS},
    )

async def load_tools(client: MCPClient) -> ToolsSchema:
    return await client.tools()   # starts the connection, returns schema with handlers attached
```
`StreamableHttpParameters` import path verified at build time (SDK re-export vs
`mcp.client.session_group`).

## 5. Context (`pipeline/context.py`)

```python
def interview_context(settings: Settings, tools: ToolsSchema | None = None) -> LLMContext:
    return LLMContext(messages=[...], tools=tools) if tools else LLMContext(messages=[...])
```
`LLMContext(tools=...)` is how Pipecat 1.8 advertises tools; handlers carried on
the schema auto-register with the LLM service.

## 6. Assembly (`pipeline/assembly.py`)

```python
@dataclass
class PipelineBuild:
    settings: Settings
    stt: STTService
    llm: LLMService
    tts: TTSService
    context: LLMContext
    mcp_client: MCPClient | None = None   # kept alive for the connection's lifetime

def build_pipeline(settings) -> PipelineBuild:          # unchanged: sync, no MCP

async def build_session_pipeline(settings, session_id: str) -> PipelineBuild:
    build = build_pipeline(settings)
    if not settings.mcp.enabled:
        return build
    client = build_mcp_client(settings.mcp, session_id)
    tools = await load_tools(client)
    build.context = interview_context(settings, tools=tools)
    build.mcp_client = client
    return build
```
Pipecat closes the MCP connection at pipeline teardown; `PipelineSession` does
not need explicit cleanup, but `runner.py` calls `mcp_client.close()` in a
`finally` as a belt-and-braces guard (safe to call twice).

## 7. Transport (`transport/websocket.py`)

```python
@app.websocket("/ws")
async def ws(websocket: WebSocket):
    session_id = websocket.query_params.get("session_id")
    settings: Settings = app.state.settings
    if settings.mcp.enabled and not session_id:
        await websocket.close(code=1008, reason="session_id query param required")
        return
    await websocket.accept()
    build = await build_session_pipeline(settings, session_id or "")
    ...
```
The app still builds `app.state.pipeline = build_pipeline(settings)` at startup
(provider validation + a target for `/health`'s "pipeline ready" log).

## 8. Runner (`pipeline/runner.py`)

`PipelineSession.__init__` takes the build; `run()` gets a `finally` that calls
`await build.mcp_client.close()` when set (idempotent per Pipecat docs).

## 9. Deps

`pyproject.toml`: `pipecat-ai[mcp,silero]`. `uv.lock` updated.

## 10. Testing

Unit (`pytest`, no network):
- **`test_mcp_tools.py`**
  - `SESSION_TOOLS` == the four Go tool names, in the documented order.
  - `build_mcp_client(cfg, "s-123")` → the client's `server_params.url` ==
    `cfg.server_url`, `server_params.headers["Authorization"] == "Bearer <token>"`,
    and `tools_arguments == {t: {"session_id": "s-123"} for t in SESSION_TOOLS}`.
    (Reach into `client._server_params` / `client._tools_arguments`.)
- **`test_pipeline.py`** (extend)
  - `build_session_pipeline(settings(mcp disabled), "s1")` returns a build with
    `mcp_client is None` and still runs the offline flow end to end.
  - `interview_context(settings, tools=None)` vs a fake `ToolsSchema` — the
    latter sets `context`'s tools.
- **`test_app.py`** (extend)
  - `/ws` with `mcp.enabled` and no `session_id` → connection closes 1008
    (via `TestClient.websocket_connect` raising / close frame).

Live integration (`@pytest.mark.integration`, skipped unless `ORIVA_GO_MCP_URL`
is set; `make test-integration`):
- `client = MCPClient(StreamableHttpParameters(url=$ORIVA_GO_MCP_URL,
  headers={Authorization: Bearer $ORIVA_GO_MCP_TOKEN}))`; `await client.tools()`
  → tool names == `SESSION_TOOLS` set.
- If `ORIVA_GO_MCP_SESSION_ID` is also set: `build_mcp_client(...)` then call
  `record_turn(question=…, answer=…)` through the client → result `recorded: True`.

`make test` = unit only. A `pytest.ini`/`pyproject` marker `integration` is
registered so the skip is clean.

## 11. Out of scope (later slices)

- How `session_id` reaches the `/ws` URL (frontend join flow / telephony webhook).
- MCP reconnect / retry / circuit-breaking on a dropped Go connection.
- Rendering tool calls + results into the interview transcript.
- The real `retrieve_context` (pgvector) — still a Go-side stub.
- Auth beyond the shared bearer token (per-session tokens, mTLS).
- Tool-call latency instrumentation into the Prometheus histograms.

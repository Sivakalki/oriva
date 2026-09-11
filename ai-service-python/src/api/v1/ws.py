"""WS /ws — the live candidate voice session. Thin handler: hands the raw
WebSocket to the pipelines service layer, which owns everything else (token
resolution, pipeline assembly, running the session).
"""

from __future__ import annotations

from fastapi import APIRouter, WebSocket

from pipelines.session import handle_ws_connection

router = APIRouter()


@router.websocket("/ws")
async def ws(websocket: WebSocket) -> None:
    await handle_ws_connection(websocket, websocket.app.state.settings)

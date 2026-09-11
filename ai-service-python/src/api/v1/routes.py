"""v1 API router: combines every v1 endpoint router into one, mounted once
by app.py. Add a new endpoint by adding its router module next to this one
and including it here.
"""

from __future__ import annotations

from fastapi import APIRouter

from api.v1 import health, metrics, ws

router = APIRouter()
router.include_router(health.router)
router.include_router(metrics.router)
router.include_router(ws.router)

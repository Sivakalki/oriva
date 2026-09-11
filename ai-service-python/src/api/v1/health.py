"""GET /health."""

from __future__ import annotations

from importlib.metadata import PackageNotFoundError, version

from fastapi import APIRouter, Request

router = APIRouter()

try:
    _VERSION = version("oriva-ai")
except PackageNotFoundError:
    _VERSION = "0.0.0-dev"


@router.get("/health")
async def health(request: Request) -> dict[str, str]:
    settings = request.app.state.settings
    return {"status": "ok", "service": settings.app.name, "version": _VERSION}

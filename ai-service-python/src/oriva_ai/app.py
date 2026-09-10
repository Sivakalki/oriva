"""FastAPI app: health, metrics, and the /ws pipeline endpoint."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI, Response
from loguru import logger
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest

import oriva_ai.telemetry  # noqa: F401  (registers the Prometheus metric objects)
from oriva_ai import __version__
from oriva_ai.config import Settings
from oriva_ai.pipeline import build_pipeline
from oriva_ai.transport import register_ws_route


def create_app(settings: Settings) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        app.state.pipeline = build_pipeline(settings)  # raises on bad provider config
        logger.info(
            "pipeline ready | env={} stt={}/{} llm={}/{} tts={}/{} vad={}",
            settings.app.env,
            settings.stt.provider,
            settings.stt.model,
            settings.llm.provider,
            settings.llm.model,
            settings.tts.provider,
            settings.tts.voice,
            settings.pipeline.vad,
        )
        yield
        logger.info("ai service stopping")

    app = FastAPI(title=settings.app.name, version=__version__, lifespan=lifespan)
    app.state.settings = settings

    @app.get("/health")
    async def health() -> dict[str, str]:
        return {"status": "ok", "service": settings.app.name, "version": __version__}

    @app.get("/metrics")
    async def metrics() -> Response:
        if not settings.metrics_enabled:
            return Response(status_code=404)
        return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)

    register_ws_route(app)
    return app

"""FastAPI app: health + metrics. Transport signaling and the pipeline runner
attach here in later slices."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI, Response
from loguru import logger
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest

import oriva_ai.telemetry  # noqa: F401  (registers the Prometheus metric objects)
from oriva_ai import __version__
from oriva_ai.config import Settings


def create_app(settings: Settings) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        logger.info(
            "ai service starting | env={} transport={} stt={}/{} llm={}/{} tts={}/{}",
            settings.app.env,
            settings.transport.type,
            settings.stt.provider,
            settings.stt.model,
            settings.llm.provider,
            settings.llm.model,
            settings.tts.provider,
            settings.tts.voice,
        )
        # TODO(slice 2): start the Pipecat pipeline runner here.
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

    return app

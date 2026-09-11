"""FastAPI app factory: wires the v1 API router (health, metrics, /ws)."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from loguru import logger

import telemetry  # noqa: F401  (registers the Prometheus metric objects)
from api.v1.health import _VERSION
from api.v1.routes import router as api_v1_router
from config import Settings
from pipelines import build_pipeline


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

    app = FastAPI(title=settings.app.name, version=_VERSION, lifespan=lifespan)
    app.state.settings = settings

    app.include_router(api_v1_router)
    return app

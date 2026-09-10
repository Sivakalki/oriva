"""Run a pipeline build against a transport for the lifetime of one session."""

from __future__ import annotations

from loguru import logger
from pipecat.frames.frames import EndFrame
from pipecat.pipeline.runner import PipelineRunner
from pipecat.transports.base_transport import BaseTransport

from oriva_ai.pipeline.assembly import PipelineBuild


class PipelineSession:
    """Owns the PipelineTask + PipelineRunner for a single connection."""

    def __init__(self, build: PipelineBuild, transport: BaseTransport) -> None:
        self._build = build
        self._task = build.make_task(transport)
        self._runner = PipelineRunner(handle_sigint=False)

    async def run(self) -> None:
        logger.info("pipeline session starting")
        try:
            await self._runner.run(self._task)
        finally:
            if self._build.mcp_client is not None:
                try:
                    await self._build.mcp_client.close()
                except Exception as exc:  # noqa: BLE001 — teardown best-effort
                    logger.warning("mcp client close failed: {}", exc)
            logger.info("pipeline session ended")

    async def stop(self) -> None:
        await self._task.queue_frame(EndFrame())

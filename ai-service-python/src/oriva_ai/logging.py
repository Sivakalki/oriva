"""Logging setup — everything flows through loguru (which Pipecat also uses)."""

from __future__ import annotations

import logging
import sys
from types import FrameType

from loguru import logger

from oriva_ai.config import LoggingConfig

_CONSOLE_FORMAT = (
    "<green>{time:YYYY-MM-DD HH:mm:ss.SSS}</green> "
    "<level>{level: <8}</level> "
    "<cyan>{name}</cyan>:<cyan>{function}</cyan> - <level>{message}</level>"
)


class _InterceptHandler(logging.Handler):
    """Redirect stdlib logging records into loguru."""

    def emit(self, record: logging.LogRecord) -> None:
        try:
            level: str | int = logger.level(record.levelname).name
        except ValueError:
            level = record.levelno

        frame: FrameType | None = logging.currentframe()
        depth = 2
        while frame and frame.f_code.co_filename == logging.__file__:
            frame = frame.f_back
            depth += 1

        # Keep the originating logger name (e.g. "uvicorn.access") in {name}.
        def _rename(r: dict[str, object]) -> None:
            r["name"] = record.name

        patched = logger.patch(_rename)  # type: ignore[arg-type]
        patched.opt(depth=depth, exception=record.exc_info).log(level, record.getMessage())


def setup_logging(cfg: LoggingConfig) -> None:
    """Configure the single loguru sink and intercept stdlib logging."""
    logger.remove()
    if cfg.format == "json":
        logger.add(sys.stderr, level=cfg.level, serialize=True)
    else:
        logger.add(sys.stderr, level=cfg.level, format=_CONSOLE_FORMAT, colorize=True)

    logging.basicConfig(handlers=[_InterceptHandler()], level=0, force=True)
    for name in ("uvicorn", "uvicorn.error", "uvicorn.access", "asyncio"):
        std = logging.getLogger(name)
        std.handlers = [_InterceptHandler()]
        std.propagate = False

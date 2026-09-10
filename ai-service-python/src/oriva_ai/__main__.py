"""Entrypoint: `python -m oriva_ai` / `uv run oriva-ai`."""

from __future__ import annotations

import sys

import uvicorn
from pydantic import ValidationError

from oriva_ai.app import create_app
from oriva_ai.config import load_settings
from oriva_ai.logging import setup_logging


def main() -> None:
    try:
        settings = load_settings()
    except (ValidationError, ValueError, OSError) as exc:
        print(f"config error:\n{exc}", file=sys.stderr)
        raise SystemExit(2) from exc

    setup_logging(settings.logging)

    uvicorn.run(
        create_app(settings),
        host=settings.server.host,
        port=settings.server.port,
        log_config=None,  # loguru owns logging
    )


if __name__ == "__main__":
    main()

"""Entrypoint: `uv run oriva-ai` (or `uv run uvicorn app:create_app --factory`)."""

from __future__ import annotations

import sys

import uvicorn
from pydantic import ValidationError

from app import create_app
from config import load_settings
from log_setup import setup_logging


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

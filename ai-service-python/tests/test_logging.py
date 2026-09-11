from __future__ import annotations

import json
import logging

from loguru import logger

from config import LoggingConfig
from log_setup import setup_logging


def test_json_sink_emits_parseable_lines() -> None:
    setup_logging(LoggingConfig(level="INFO", format="json"))
    sink: list[str] = []
    logger.add(sink.append, level="INFO", serialize=True)

    logger.info("hello {}", "world")

    record = json.loads(sink[-1])
    assert record["record"]["message"] == "hello world"
    assert record["record"]["level"]["name"] == "INFO"


def test_stdlib_logging_is_intercepted() -> None:
    setup_logging(LoggingConfig(level="DEBUG", format="console"))
    sink: list[str] = []
    logger.add(sink.append, level="DEBUG")

    logging.getLogger("some.third_party").warning("from stdlib")

    assert any("from stdlib" in line for line in sink)

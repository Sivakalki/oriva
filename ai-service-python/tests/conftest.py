from __future__ import annotations

import sys

import pytest
from loguru import logger

from config import Settings

# Keep Pipecat's very chatty DEBUG logs out of the test output.
logger.remove()
logger.add(sys.stderr, level="WARNING")


@pytest.fixture
def settings() -> Settings:
    return Settings()

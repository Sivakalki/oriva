from __future__ import annotations

import pytest

from oriva_ai.config import Settings


@pytest.fixture
def settings() -> Settings:
    return Settings()

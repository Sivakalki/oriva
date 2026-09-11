"""Local Piper TTS. Imports pipecat's piper service lazily so a missing
`pipecat-ai[piper]` extra only fails when this provider is actually selected.
"""

from __future__ import annotations

from pathlib import Path

from pipecat.services.tts_service import TTSService

from config import TTSConfig

# Where lazily-downloaded voice models land.
_MODEL_DIR = Path.home() / ".cache" / "oriva-ai" / "models"


def build_tts(cfg: TTSConfig) -> TTSService:
    from pipecat.services.piper.tts import PiperTTSService

    _MODEL_DIR.mkdir(parents=True, exist_ok=True)
    # The voice model (~60MB) is downloaded into download_dir on first use.
    return PiperTTSService(
        settings=PiperTTSService.Settings(voice=cfg.voice),
        download_dir=_MODEL_DIR,
    )

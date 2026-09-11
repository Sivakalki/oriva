from __future__ import annotations

from pathlib import Path

from config import Settings
from harness.clip_run import run_clip

FIXTURE = Path("src/harness/fixtures/short_answer.wav")


async def test_run_clip_fills_every_field() -> None:
    r = await run_clip(Settings(), FIXTURE, "fx")
    assert r.clip_id == "fx"
    assert r.transcript
    assert r.reply
    assert r.audio_chunks > 0
    assert r.stt_ms is not None
    assert r.llm_ttft_ms is not None
    assert r.tts_ttfa_ms is not None
    assert r.v2v_ms is not None

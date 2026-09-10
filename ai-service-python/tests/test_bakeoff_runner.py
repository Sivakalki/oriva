from __future__ import annotations

import json
from pathlib import Path

import pytest

from oriva_ai.bakeoff import report, runner

CLIPS_DIR = Path("src/oriva_ai/bakeoff/clips")
CLIPS = [
    {"id": "clean_short", "file": "clean_short.wav", "reference_transcript": "I have five years"},
    {"id": "noisy_short", "file": "noisy_short.wav", "reference_transcript": "I prefer go"},
]


async def test_run_matrix_all_mock() -> None:
    result = await runner.run_matrix(
        [{"name": "all-mock", "stt": {"provider": "mock"}}], CLIPS, CLIPS_DIR
    )
    assert len(result.combos) == 1
    agg = result.combos[0]
    assert agg.available
    assert agg.n == 2
    assert agg.stt_ms_p50 is not None
    assert agg.v2v_ms_p95 is not None
    assert len(result.per_clip) == 2


async def test_run_matrix_unavailable_provider(monkeypatch: pytest.MonkeyPatch) -> None:
    from oriva_ai.pipeline.providers import registry

    def boom(_cfg: object) -> object:
        raise ModuleNotFoundError("No module named 'faster_whisper'")

    monkeypatch.setitem(registry.STT_PROVIDERS, "whisper", boom)

    result = await runner.run_matrix(
        [{"name": "needs-whisper", "stt": {"provider": "whisper"}}], CLIPS, CLIPS_DIR
    )
    agg = result.combos[0]
    assert agg.available is False
    assert agg.error and "faster_whisper" in agg.error


async def test_report_write(tmp_path: Path) -> None:
    result = await runner.run_matrix(
        [{"name": "all-mock", "stt": {"provider": "mock"}}], CLIPS, CLIPS_DIR
    )
    report.write(result, tmp_path)

    csv_lines = (tmp_path / "results.csv").read_text().splitlines()
    assert csv_lines[0].startswith("combo,available,n,")
    assert len(csv_lines) == 2  # header + 1 combo

    payload = json.loads((tmp_path / "results.json").read_text())
    assert payload["combos"][0]["combo"] == "all-mock"
    assert len(payload["per_clip"]) == 2

    assert "all-mock" in report.render_table(result)

from __future__ import annotations

from bakeoff.metrics import aggregate, normalize, word_error_rate
from harness.clip_run import ClipResult


def test_normalize_strips_punct_and_case() -> None:
    assert normalize("Hello, WORLD!  it's fine.") == ["hello", "world", "it's", "fine"]


def test_wer_identical() -> None:
    assert word_error_rate("a b c d e", "a b c d e") == 0.0


def test_wer_one_substitution_in_five() -> None:
    assert word_error_rate("a b c d e", "a b X d e") == 0.2


def test_wer_empty_hypothesis() -> None:
    assert word_error_rate("a b", "") == 1.0


def test_wer_both_empty() -> None:
    assert word_error_rate("", "") == 0.0


def _cr(stt: float, llm: float, tts: float, v2v: float) -> ClipResult:
    return ClipResult(
        clip_id="c",
        transcript="t",
        reply="r",
        audio_chunks=1,
        stt_ms=stt,
        llm_ttft_ms=llm,
        tts_ttfa_ms=tts,
        v2v_ms=v2v,
    )


def test_aggregate() -> None:
    scored = [
        (_cr(10, 100, 40, 200), 0.0),
        (_cr(20, 110, 50, 220), 0.5),
        (_cr(30, 120, 60, 240), 1.0),
    ]
    agg = aggregate("combo-a", scored)
    assert agg.available
    assert agg.n == 3
    assert agg.stt_ms_p50 is not None and 10 <= agg.stt_ms_p50 <= 30
    assert agg.wer_mean == 0.5

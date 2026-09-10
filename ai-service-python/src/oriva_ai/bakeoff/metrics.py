"""Scoring: word error rate and per-combo aggregation."""

from __future__ import annotations

import re
import statistics
from dataclasses import dataclass

from oriva_ai.harness.clip_run import ClipResult

_PUNCT = re.compile(r"[^\w\s']")


def normalize(text: str) -> list[str]:
    """Lowercase, strip punctuation, split on whitespace."""
    return _PUNCT.sub(" ", text.lower()).split()


def _levenshtein(a: list[str], b: list[str]) -> int:
    if not a:
        return len(b)
    if not b:
        return len(a)
    prev = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        cur = [i]
        for j, cb in enumerate(b, 1):
            cur.append(min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + (ca != cb)))
        prev = cur
    return prev[-1]


def word_error_rate(reference: str, hypothesis: str) -> float:
    """Levenshtein distance over word tokens, divided by the reference length.

    Empty reference and empty hypothesis -> 0.0. Empty hypothesis against a
    non-empty reference -> 1.0.
    """
    ref, hyp = normalize(reference), normalize(hypothesis)
    if not ref:
        return 0.0 if not hyp else 1.0
    return _levenshtein(ref, hyp) / len(ref)


def _pct(values: list[float], q: float) -> float | None:
    vals = [v for v in values if v is not None]
    if not vals:
        return None
    if len(vals) == 1:
        return vals[0]
    # statistics.quantiles gives 100 cut points; index q*100 - 1
    return statistics.quantiles(vals, n=100, method="inclusive")[int(q * 100) - 1]


@dataclass
class ComboAggregate:
    combo: str
    available: bool
    n: int
    stt_ms_p50: float | None = None
    stt_ms_p95: float | None = None
    llm_ttft_ms_p50: float | None = None
    llm_ttft_ms_p95: float | None = None
    tts_ttfa_ms_p50: float | None = None
    tts_ttfa_ms_p95: float | None = None
    v2v_ms_p50: float | None = None
    v2v_ms_p95: float | None = None
    wer_mean: float | None = None
    error: str | None = None


def unavailable(combo: str, error: str) -> ComboAggregate:
    return ComboAggregate(combo=combo, available=False, n=0, error=error)


def aggregate(combo: str, scored: list[tuple[ClipResult, float]]) -> ComboAggregate:
    def col(attr: str) -> list[float]:
        return [getattr(r, attr) for r, _ in scored if getattr(r, attr) is not None]

    wers = [w for _, w in scored]
    return ComboAggregate(
        combo=combo,
        available=True,
        n=len(scored),
        stt_ms_p50=_pct(col("stt_ms"), 0.50),
        stt_ms_p95=_pct(col("stt_ms"), 0.95),
        llm_ttft_ms_p50=_pct(col("llm_ttft_ms"), 0.50),
        llm_ttft_ms_p95=_pct(col("llm_ttft_ms"), 0.95),
        tts_ttfa_ms_p50=_pct(col("tts_ttfa_ms"), 0.50),
        tts_ttfa_ms_p95=_pct(col("tts_ttfa_ms"), 0.95),
        v2v_ms_p50=_pct(col("v2v_ms"), 0.50),
        v2v_ms_p95=_pct(col("v2v_ms"), 0.95),
        wer_mean=statistics.fmean(wers) if wers else None,
    )

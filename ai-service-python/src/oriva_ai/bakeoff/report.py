"""Render a MatrixResult as a console table and write results.csv / results.json."""

from __future__ import annotations

import csv
import dataclasses
import json
from pathlib import Path
from typing import TYPE_CHECKING

from oriva_ai.bakeoff.metrics import ComboAggregate

if TYPE_CHECKING:
    from oriva_ai.bakeoff.runner import MatrixResult

_COLUMNS = [
    "combo",
    "available",
    "n",
    "stt_ms_p50",
    "stt_ms_p95",
    "llm_ttft_ms_p50",
    "llm_ttft_ms_p95",
    "tts_ttfa_ms_p50",
    "tts_ttfa_ms_p95",
    "v2v_ms_p50",
    "v2v_ms_p95",
    "wer_mean",
    "error",
]


def _cell(v: object) -> str:
    if v is None:
        return "-"
    if isinstance(v, float):
        return f"{v:.1f}" if v >= 1 else f"{v:.3f}"
    return str(v)


def render_table(m: MatrixResult) -> str:
    headers = ["combo", "n", "stt p50/p95", "llm_ttft p50/p95", "tts p50/p95", "v2v p50/p95", "wer"]
    rows: list[list[str]] = []
    for c in m.combos:
        if not c.available:
            rows.append([c.combo, "-", "unavailable", "", "", "", c.error or ""])
            continue
        rows.append(
            [
                c.combo,
                str(c.n),
                f"{_cell(c.stt_ms_p50)}/{_cell(c.stt_ms_p95)}",
                f"{_cell(c.llm_ttft_ms_p50)}/{_cell(c.llm_ttft_ms_p95)}",
                f"{_cell(c.tts_ttfa_ms_p50)}/{_cell(c.tts_ttfa_ms_p95)}",
                f"{_cell(c.v2v_ms_p50)}/{_cell(c.v2v_ms_p95)}",
                _cell(c.wer_mean),
            ]
        )
    widths = [
        max(len(h), *(len(r[i]) for r in rows)) if rows else len(h) for i, h in enumerate(headers)
    ]
    line = "  ".join(h.ljust(widths[i]) for i, h in enumerate(headers))
    sep = "  ".join("-" * widths[i] for i in range(len(headers)))
    body = "\n".join("  ".join(r[i].ljust(widths[i]) for i in range(len(headers))) for r in rows)
    return f"{line}\n{sep}\n{body}"


def write(m: MatrixResult, out_dir: Path) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    print(render_table(m))

    with (out_dir / "results.csv").open("w", newline="") as fh:
        w = csv.DictWriter(fh, fieldnames=_COLUMNS)
        w.writeheader()
        for c in m.combos:
            w.writerow({k: _csv_val(getattr(c, k)) for k in _COLUMNS})

    payload = {
        "combos": [dataclasses.asdict(c) for c in m.combos],
        "per_clip": m.per_clip,
    }
    (out_dir / "results.json").write_text(json.dumps(payload, indent=2))
    print(f"\nwrote {out_dir / 'results.csv'} and {out_dir / 'results.json'}")


def _csv_val(v: object) -> object:
    if isinstance(v, float):
        return round(v, 4)
    return v


__all__ = ["render_table", "write", "ComboAggregate"]

"""Push per-combo aggregates to a Prometheus Pushgateway for Grafana."""

from __future__ import annotations

from typing import TYPE_CHECKING

from prometheus_client import CollectorRegistry, Gauge, push_to_gateway

if TYPE_CHECKING:
    from oriva_ai.bakeoff.runner import MatrixResult

_METRICS = (
    "stt_ms_p50",
    "stt_ms_p95",
    "llm_ttft_ms_p50",
    "llm_ttft_ms_p95",
    "tts_ttfa_ms_p50",
    "tts_ttfa_ms_p95",
    "v2v_ms_p50",
    "v2v_ms_p95",
    "wer_mean",
)


def push(m: MatrixResult, gateway: str = "localhost:9091", job: str = "oriva_bakeoff") -> None:
    registry = CollectorRegistry()
    gauges = {
        name: Gauge(f"oriva_bakeoff_{name}", f"bake-off {name}", ["combo"], registry=registry)
        for name in _METRICS
    }
    for combo in m.combos:
        if not combo.available:
            continue
        for name, gauge in gauges.items():
            value = getattr(combo, name)
            if value is not None:
                gauge.labels(combo=combo.combo).set(value)

    push_to_gateway(gateway, job=job, registry=registry)
    print(f"pushed {sum(c.available for c in m.combos)} combos to {gateway}")

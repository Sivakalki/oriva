from __future__ import annotations

from pipecat.frames.frames import (
    BotStartedSpeakingFrame,
    Frame,
    LLMTextFrame,
    TranscriptionFrame,
    TTSAudioRawFrame,
    UserStoppedSpeakingFrame,
)
from pipecat.observers.base_observer import FramePushed
from pipecat.processors.frame_processor import FrameDirection
from prometheus_client import REGISTRY

from oriva_ai.config import Settings
from oriva_ai.telemetry.observer import MetricsObserver


def _count(metric: str, **labels: str) -> float:
    return REGISTRY.get_sample_value(f"{metric}_count", labels) or 0.0


def _push(frame: Frame, ts: int) -> FramePushed:
    return FramePushed(
        source=None,
        destination=None,
        frame=frame,
        direction=FrameDirection.DOWNSTREAM,
        timestamp=ts,
    )


async def test_stage_latencies_recorded() -> None:
    s = Settings()
    obs = MetricsObserver(s, "stt#0", "llm#0", "tts#0")

    before = {
        "stt": _count("oriva_stt_latency_seconds", provider=s.stt.provider, model=s.stt.model),
        "llm": _count("oriva_llm_ttft_seconds", provider=s.llm.provider, model=s.llm.model),
        "tts": _count("oriva_tts_first_audio_seconds", provider=s.tts.provider, voice=s.tts.voice),
        "v2v": _count("oriva_voice_to_voice_seconds", llm_model=s.llm.model),
    }

    ns = 1_000_000_000
    await obs.on_push_frame(_push(UserStoppedSpeakingFrame(), 0))
    await obs.on_push_frame(_push(TranscriptionFrame("hi", "", "t"), 30 * ns // 1000))
    await obs.on_push_frame(_push(LLMTextFrame("You "), 110 * ns // 1000))
    await obs.on_push_frame(_push(TTSAudioRawFrame(b"\x00", 16000, 1), 150 * ns // 1000))

    assert (
        _count("oriva_stt_latency_seconds", provider=s.stt.provider, model=s.stt.model)
        == before["stt"] + 1
    )
    assert (
        _count("oriva_llm_ttft_seconds", provider=s.llm.provider, model=s.llm.model)
        == before["llm"] + 1
    )
    assert (
        _count("oriva_tts_first_audio_seconds", provider=s.tts.provider, voice=s.tts.voice)
        == before["tts"] + 1
    )
    assert _count("oriva_voice_to_voice_seconds", llm_model=s.llm.model) == before["v2v"] + 1


async def test_voice_to_voice_on_bot_started() -> None:
    s = Settings()
    obs = MetricsObserver(s, "stt#0", "llm#0", "tts#0")
    before = _count("oriva_voice_to_voice_seconds", llm_model=s.llm.model)

    await obs.on_push_frame(_push(UserStoppedSpeakingFrame(), 1_000_000_000))
    await obs.on_push_frame(_push(BotStartedSpeakingFrame(), 1_900_000_000))

    assert _count("oriva_voice_to_voice_seconds", llm_model=s.llm.model) == before + 1
    got = REGISTRY.get_sample_value("oriva_voice_to_voice_seconds_sum", {"llm_model": s.llm.model})
    assert got is not None and got >= 0.9

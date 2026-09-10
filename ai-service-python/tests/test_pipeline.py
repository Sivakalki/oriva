from __future__ import annotations

from pipecat.frames.frames import LLMContextFrame, LLMTextFrame, TTSAudioRawFrame
from pipecat.pipeline.task import PipelineParams
from pipecat.tests.utils import run_test

from oriva_ai.config import Settings
from oriva_ai.harness.memory_transport import wav_to_frames
from oriva_ai.pipeline import build_pipeline
from oriva_ai.pipeline.offline import Collector, build_offline_pipeline

FIXTURE = "src/oriva_ai/harness/fixtures/short_answer.wav"


def test_build_pipeline_all_mock() -> None:
    build = build_pipeline(Settings())
    assert build.stt.name and build.llm.name and build.tts.name


async def test_mock_pipeline_runs_end_to_end() -> None:
    build = build_pipeline(Settings())
    post_llm, post_tts = Collector(), Collector()
    pipeline = build_offline_pipeline(build, post_llm=post_llm, post_tts=post_tts)

    down, _ = await run_test(
        pipeline,
        frames_to_send=wav_to_frames(FIXTURE),
        expected_down_frames=None,
        pipeline_params=PipelineParams(enable_metrics=True, enable_usage_metrics=True),
    )

    # STT produced a transcript -> the bridge emitted a context frame.
    assert any(isinstance(f, LLMContextFrame) for f in down)

    # The LLM streamed a templated reply built from the transcript ...
    reply = "".join(f.text for f in post_llm.frames if isinstance(f, LLMTextFrame))
    assert "You said:" in reply
    assert "backend experience" in reply

    # ... and TTS synthesized audio from it.
    assert sum(isinstance(f, TTSAudioRawFrame) for f in post_tts.frames) > 0

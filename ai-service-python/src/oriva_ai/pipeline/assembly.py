"""Assemble the STT -> LLM -> TTS pipeline from settings."""

from __future__ import annotations

from dataclasses import dataclass

from pipecat.pipeline.pipeline import Pipeline
from pipecat.pipeline.task import PipelineParams, PipelineTask
from pipecat.processors.aggregators.llm_context import LLMContext
from pipecat.processors.aggregators.llm_response_universal import (
    LLMContextAggregatorPair,
    LLMUserAggregatorParams,
)
from pipecat.services.llm_service import LLMService
from pipecat.services.mcp_service import MCPClient
from pipecat.services.stt_service import STTService
from pipecat.services.tts_service import TTSService
from pipecat.transports.base_transport import BaseTransport

from oriva_ai.config import Settings
from oriva_ai.pipeline.context import interview_context
from oriva_ai.pipeline.mcp_tools import build_mcp_client, load_tools
from oriva_ai.pipeline.providers import build_llm, build_stt, build_tts
from oriva_ai.telemetry.observer import MetricsObserver


@dataclass
class PipelineBuild:
    """Everything needed to run the pipeline for one connection."""

    settings: Settings
    stt: STTService
    llm: LLMService
    tts: TTSService
    context: LLMContext
    mcp_client: MCPClient | None = None

    def make_task(self, transport: BaseTransport) -> PipelineTask:
        user_params = LLMUserAggregatorParams()
        if self.settings.pipeline.vad == "silero":
            from pipecat.audio.vad.silero import SileroVADAnalyzer

            user_params.vad_analyzer = SileroVADAnalyzer()
        aggregators = LLMContextAggregatorPair(self.context, user_params=user_params)
        pipeline = Pipeline(
            [
                transport.input(),
                self.stt,
                aggregators.user(),
                self.llm,
                self.tts,
                transport.output(),
                aggregators.assistant(),
            ]
        )
        observer = MetricsObserver(
            self.settings,
            stt_name=self.stt.name,
            llm_name=self.llm.name,
            tts_name=self.tts.name,
        )
        return PipelineTask(
            pipeline,
            params=PipelineParams(
                enable_metrics=True,
                enable_usage_metrics=True,
                audio_in_sample_rate=self.settings.pipeline.sample_rate,
            ),
            observers=[observer],
        )


def build_pipeline(settings: Settings) -> PipelineBuild:
    """Construct the services once. Raises on bad provider config (fail fast).

    No MCP — used by app startup (validation) and the offline harness.
    """
    return PipelineBuild(
        settings=settings,
        stt=build_stt(settings.stt),
        llm=build_llm(settings.llm),
        tts=build_tts(settings.tts),
        context=interview_context(settings),
    )


async def build_session_pipeline(settings: Settings, session_id: str) -> PipelineBuild:
    """Build a pipeline bound to one interview session, with MCP tools attached."""
    build = build_pipeline(settings)
    if not settings.mcp.enabled:
        return build

    client = build_mcp_client(settings.mcp, session_id)
    tools = await load_tools(client)
    build.context = interview_context(settings, tools=tools)
    build.mcp_client = client
    return build

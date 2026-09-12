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
from pipecat.turns.user_stop.speech_timeout_user_turn_stop_strategy import (
    SpeechTimeoutUserTurnStopStrategy,
)
from pipecat.turns.user_turn_strategies import UserTurnStrategies

from config import Settings
from pipelines.context import interview_context
from pipelines.context_compactor import ContextCompactionTrigger, ContextCompactor
from pipelines.mcp_tools import build_mcp_client, call_tool, load_tools
from pipelines.providers import build_llm, build_stt, build_tts
from pipelines.question_queue import generate_questions
from pipelines.queue_interviewer import QueueInterviewer
from pipelines.wire_compat import WebsocketClientCompatFilter
from telemetry.observer import MetricsObserver


@dataclass
class PipelineBuild:
    """Everything needed to run the pipeline for one connection."""

    settings: Settings
    stt: STTService
    llm: LLMService
    tts: TTSService
    context: LLMContext
    mcp_client: MCPClient | None = None
    # Set only when MCP is enabled (build_session_pipeline): owns question
    # pacing for the call. When None, make_task falls back to the older
    # free-form flow where the live LLM decides everything itself -- the dev/
    # offline/MCP-disabled path only, not how a real interview runs.
    queue_interviewer: QueueInterviewer | None = None

    def make_task(self, transport: BaseTransport) -> PipelineTask:
        # Plain silence-timeout turn-stop, not Pipecat's own default (a local
        # ML "does this sound complete" model) -- see PipelineConfig.
        # speech_timeout_secs's docstring for why.
        user_params = LLMUserAggregatorParams(
            user_turn_strategies=UserTurnStrategies(
                stop=[
                    SpeechTimeoutUserTurnStopStrategy(
                        user_speech_timeout=self.settings.pipeline.speech_timeout_secs
                    )
                ]
            )
        )
        if self.settings.pipeline.vad == "silero":
            from pipecat.audio.vad.silero import SileroVADAnalyzer

            user_params.vad_analyzer = SileroVADAnalyzer()
        aggregators = LLMContextAggregatorPair(self.context, user_params=user_params)
        compactor = ContextCompactor(self.context, self.settings)
        stages = [transport.input(), self.stt, aggregators.user()]
        if self.queue_interviewer is not None:
            # Owns question pacing from here: it never forwards the
            # candidate's answer to self.llm, so the LLM node stays wired
            # but unused for the duration of the call.
            stages.append(self.queue_interviewer)
        stages.append(self.llm)
        stages.extend(
            [
                self.tts,
                # See wire_compat.py: the websocket-transport client can't
                # deserialize InterruptionFrame/TextFrame/TranscriptionFrame --
                # drop them here so a barge-in never crashes the connection.
                WebsocketClientCompatFilter(),
                transport.output(),
                aggregators.assistant(),
                # After the assistant's reply is committed to context: check
                # whether it's grown past pipeline.context_compress_words and,
                # if so, compress it in the background (never blocks here).
                ContextCompactionTrigger(compactor),
            ]
        )
        pipeline = Pipeline(stages)
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
    """Build a pipeline bound to one interview session, with MCP tools attached.

    When MCP is enabled (the normal deployment mode), this also fetches the
    interview plan and generates the question queue upfront so the call can
    be fully queue-driven from the first frame -- see QueueInterviewer.
    """
    build = build_pipeline(settings)
    if not settings.mcp.enabled:
        return build

    client = build_mcp_client(settings.mcp, session_id)
    tools = await load_tools(client)
    build.context = interview_context(settings, tools=tools)
    build.mcp_client = client

    plan = await call_tool(client, "get_interview_plan")
    questions = await generate_questions(settings, plan)
    build.queue_interviewer = QueueInterviewer(
        client,
        questions,
        duration_minutes=plan.get("duration_minutes") or settings.pipeline.default_duration_minutes,
        wrap_up_text=settings.pipeline.wrap_up_message,
    )
    return build

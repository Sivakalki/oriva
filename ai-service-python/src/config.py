"""Single typed configuration for the service.

Precedence (highest first): environment variables (``ORIVA_AI__SECTION__KEY``) >
``config.yaml`` > model defaults. The YAML path is overridable with
``ORIVA_AI_CONFIG_FILE``.
"""

from __future__ import annotations

import os
import socket
from functools import lru_cache
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, Field
from pydantic_settings import (
    BaseSettings,
    PydanticBaseSettingsSource,
    SettingsConfigDict,
    YamlConfigSettingsSource,
)

# Workaround: some hosts' IPv6 egress hangs in SYN-SENT against origins like
# huggingface.co while IPv4 works fine, which silently stalls the first
# Whisper/Piper model download for minutes. Force IPv4-only DNS resolution
# process-wide. This module is imported by every entrypoint (app, bakeoff,
# harness) before any network client is created, so it's the one place this
# is guaranteed to run early. Set ORIVA_AI_FORCE_IPV4=0 to disable.
if os.environ.get("ORIVA_AI_FORCE_IPV4", "1") != "0":
    _orig_getaddrinfo = socket.getaddrinfo

    def _ipv4_only_getaddrinfo(host, port, family=0, type=0, proto=0, flags=0):  # type: ignore[no-untyped-def]  # noqa: A002
        return _orig_getaddrinfo(host, port, socket.AF_INET, type, proto, flags)

    socket.getaddrinfo = _ipv4_only_getaddrinfo

# Workaround: faster-whisper/ctranslate2's CUDA path needs libcublas.so.12
# and libcudnn.so.9 at runtime, but pip-installed nvidia-cublas-cu12/
# nvidia-cudnn-cu12 don't register themselves on the dynamic linker's search
# path -- they just drop .so files under site-packages/nvidia/*/lib. Setting
# LD_LIBRARY_PATH from *within* the running process doesn't help either:
# glibc reads it once at process start, not per dlopen (confirmed by testing
# directly -- exporting it before `python` starts works, mutating
# os.environ after does not). ctypes-preloading the actual .so files with
# RTLD_GLOBAL does work regardless of that timing, since it's the process
# itself holding the library open rather than asking the linker to find it
# by name later. Runs once at import time, before any CUDA code path can
# need it; silently a no-op if the nvidia packages aren't installed (a
# CPU-only STT config never needs this). Set ORIVA_AI_PRELOAD_CUDA_LIBS=0
# to disable.
if os.environ.get("ORIVA_AI_PRELOAD_CUDA_LIBS", "1") != "0":
    try:
        import ctypes
        import glob

        import nvidia.cublas.lib  # type: ignore[import-untyped]
        import nvidia.cudnn.lib  # type: ignore[import-untyped]

        for _pkg_path in (nvidia.cublas.lib.__path__[0], nvidia.cudnn.lib.__path__[0]):
            for _so in sorted(glob.glob(os.path.join(_pkg_path, "*.so*"))):
                try:
                    ctypes.CDLL(_so, mode=ctypes.RTLD_GLOBAL)
                except OSError:
                    pass  # best-effort -- a real CUDA use will surface its own error
    except ImportError:
        pass  # nvidia-cublas-cu12/nvidia-cudnn-cu12 not installed -- CPU-only is fine

DEFAULT_CONFIG_PATH = "config.yaml"
CONFIG_PATH_ENV = "ORIVA_AI_CONFIG_FILE"

# Set by load_settings() just before Settings() is constructed so that
# settings_customise_sources can point the YAML source at the right file.
_active_yaml_path: Path = Path(DEFAULT_CONFIG_PATH)


class AppConfig(BaseModel):
    name: str = "oriva-ai"
    env: Literal["dev", "staging", "prod"] = "dev"


class LoggingConfig(BaseModel):
    level: str = "DEBUG"
    format: Literal["console", "json"] = "console"


class ServerConfig(BaseModel):
    host: str = "0.0.0.0"
    port: int = 8090


class TransportConfig(BaseModel):
    type: Literal["websocket", "webrtc"] = "websocket"


class STTConfig(BaseModel):
    provider: str = "mock"
    # distil-medium.en: close to medium-tier accuracy at close to small-tier
    # speed -- a real step up from base.en, still light enough to share a
    # 4GB GPU with the LLM (see device/compute_type below).
    model: str = "distil-medium.en"
    language: str = "en"
    # "auto" tries CUDA first, falls back to CPU if unavailable -- never
    # crash-loops just because the GPU is busy or absent. See
    # pipelines/providers/whisper/stt.py for how this is resolved: CTranslate2
    # compute types are NOT interchangeable between devices (e.g.
    # "int8_float16" errors outright on CPU), so the CPU fallback always
    # forces plain "int8" regardless of the value below.
    device: str = "auto"
    # Compute type used only on the CUDA path (ignored on CPU -- see above).
    # int8_float16: quantized weights + fp16 compute, roughly half the VRAM
    # of plain float16 -- meaningfully more accurate than CPU-only int8
    # thanks to GPU compute, without risking the whole 4GB card on one model.
    compute_type: str = "int8_float16"
    mock_transcript: str = "I have about five years of backend experience."


class LLMConfig(BaseModel):
    provider: str = "mock"
    model: str = "openai/gpt-4o-mini"
    base_url: str = "http://localhost:4000"
    api_key: str = "not-needed-for-local-litellm"
    mock_reply_template: str = "You said: {user} Can you tell me more about that?"


class TTSConfig(BaseModel):
    provider: str = "mock"
    voice: str = "en_US-lessac-medium"


class PipelineConfig(BaseModel):
    greeting: str = "Hi, thanks for joining. Let's begin when you're ready."
    sample_rate: int = 16000
    vad: Literal["silero", "none"] = "silero"
    # Background context compaction (pipelines/context_compactor.py): once
    # the live LLM context passes this many words, it's summarized in the
    # background so it doesn't grow unbounded over a long interview. 0 disables.
    context_compress_words: int = 800
    # Question-queue subsystem (pipelines/question_queue.py,
    # pipelines/queue_interviewer.py): active whenever MCP is enabled, in
    # place of letting the live conversational LLM decide pacing/ordering.
    question_count: int = 5
    wrap_up_message: str = (
        "That's everything I needed to ask. Thanks so much for your time "
        "today -- we'll follow up soon with next steps."
    )
    # Used only if backend-go's get_interview_plan doesn't return a
    # duration_minutes (e.g. an older session scheduled before that field
    # existed).
    default_duration_minutes: int = 30


class MCPConfig(BaseModel):
    server_url: str = "http://localhost:8080/mcp"
    enabled: bool = True
    auth_token: str = "dev-mcp-token"  # matches backend-go dev default


class BackendConfig(BaseModel):
    base_url: str = "http://localhost:8080"  # backend-go origin (join-token lookup)


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_prefix="ORIVA_AI__",
        env_nested_delimiter="__",
        extra="forbid",
    )

    app: AppConfig = Field(default_factory=AppConfig)
    logging: LoggingConfig = Field(default_factory=LoggingConfig)
    server: ServerConfig = Field(default_factory=ServerConfig)
    transport: TransportConfig = Field(default_factory=TransportConfig)
    stt: STTConfig = Field(default_factory=STTConfig)
    llm: LLMConfig = Field(default_factory=LLMConfig)
    tts: TTSConfig = Field(default_factory=TTSConfig)
    pipeline: PipelineConfig = Field(default_factory=PipelineConfig)
    mcp: MCPConfig = Field(default_factory=MCPConfig)
    backend: BackendConfig = Field(default_factory=BackendConfig)
    metrics_enabled: bool = True

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: type[BaseSettings],
        init_settings: PydanticBaseSettingsSource,
        env_settings: PydanticBaseSettingsSource,
        dotenv_settings: PydanticBaseSettingsSource,
        file_secret_settings: PydanticBaseSettingsSource,
    ) -> tuple[PydanticBaseSettingsSource, ...]:
        yaml_source = YamlConfigSettingsSource(settings_cls, yaml_file=_active_yaml_path)
        # env before yaml -> env wins.
        return (init_settings, env_settings, dotenv_settings, yaml_source, file_secret_settings)


def load_settings(path: str | Path | None = None) -> Settings:
    """Build Settings from the YAML file plus environment overrides."""
    global _active_yaml_path
    _active_yaml_path = Path(path or os.environ.get(CONFIG_PATH_ENV, DEFAULT_CONFIG_PATH))
    return Settings()


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Cached process-wide settings."""
    return load_settings()

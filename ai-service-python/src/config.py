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
    model: str = "base.en"
    language: str = "en"
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


class PostgresConfig(BaseModel):
    dsn: str = "postgresql://oriva:oriva@localhost:5432/oriva"
    readonly: bool = True


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
    postgres: PostgresConfig = Field(default_factory=PostgresConfig)
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

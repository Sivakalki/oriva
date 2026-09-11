"""LLM via an OpenAI-compatible chat-completions endpoint. Imports pipecat's
openai service lazily so a missing `pipecat-ai[openai]` extra only fails
when this provider is actually selected. Used for both the "openai" and
"litellm" config keys — and, with base_url pointed at a local Ollama
server's /v1 endpoint, for a local Ollama-served model too.
"""

from __future__ import annotations

from typing import cast

from pipecat.services.llm_service import LLMService

from config import LLMConfig


def build_llm(cfg: LLMConfig) -> LLMService:
    from pipecat.services.openai.llm import OpenAILLMService

    return cast(
        LLMService,
        OpenAILLMService(model=cfg.model, base_url=cfg.base_url, api_key=cfg.api_key),
    )

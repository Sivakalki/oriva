"""LLM via any OpenAI-compatible chat-completions endpoint — OpenAI itself, a
LiteLLM proxy, or a local Ollama server (its /v1 endpoint). Needs
`uv add "pipecat-ai[openai]"`.
"""

from pipelines.providers.openai.llm import build_llm

__all__ = ["build_llm"]

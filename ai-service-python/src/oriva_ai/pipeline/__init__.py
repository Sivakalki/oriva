"""Voice pipeline (STT -> LLM -> TTS).

Empty in slice 1. Slice 2 adds the Pipecat pipeline skeleton with config-driven
swappable STT/LLM/TTS services and a local WebSocket transport.
"""

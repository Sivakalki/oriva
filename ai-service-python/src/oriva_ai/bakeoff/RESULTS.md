# Bake-off results

Latest run: `uv run python -m oriva_ai.bakeoff` (see `out/results.{csv,json}`, gitignored).

## 2026-09-10 — first real TTS (Piper, local)

STT and LLM are still `mock` (the synthetic tone-sweep clips don't transcribe to
anything, so WER is not meaningful yet). TTS-time-to-first-audio and
voice-to-voice are real for the Piper combos.

| combo                          | tts_ttfa p50/p95 (ms) | v2v p50/p95 (ms) |
|--------------------------------|-----------------------|------------------|
| all-mock                       | 46 / 70               | 129 / 154        |
| mock-stt-mock-llm-piper (med)  | 128 / 143             | 210 / 225        |
| mock-stt-mock-llm-piper (low)  | 97 / 100              | 180 / 183        |

Piper first-audio (~100-145 ms) sits inside the `docs/ARCHITECTURE.md` §1 budget
(TTS TTFA 40-150 ms). The `low` voice is ~30 ms faster to first audio.

## Still needed for a real STT/LLM bake-off (PLAN.md Phase 1 steps 4-5)

- Record real candidate-answer clips (replace the synthetic set — see
  `clips/README.md`) so `whisper` STT produces real transcripts and WER means
  something.
- Stand up a LiteLLM proxy (or point `llm.base_url` at a local model) and add the
  `litellm` combos in `combos.yaml`.

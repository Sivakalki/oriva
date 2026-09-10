from __future__ import annotations

from oriva_ai.harness.run_clip import main


def test_run_clip_on_fixture(capsys) -> None:  # type: ignore[no-untyped-def]
    rc = main(["src/oriva_ai/harness/fixtures/short_answer.wav"])
    out = capsys.readouterr().out
    assert rc == 0
    for line in (
        "stt_latency_ms",
        "llm_ttft_ms",
        "tts_first_audio_ms",
        "voice_to_voice_ms",
    ):
        assert line in out
    assert "I have about five years" in out  # mock transcript
    assert "You said:" in out  # mock reply

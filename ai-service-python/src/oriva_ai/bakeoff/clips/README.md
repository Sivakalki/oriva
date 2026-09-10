# Bake-off clip set

`manifest.yaml` + the `*.wav` files are the fixed test set the bake-off replays
against every provider combo (`docs/PLAN.md` Phase 1 step 3).

The committed clips are **synthetic tone sweeps** — they let the harness run end
to end with no recordings and no STT installed. WER numbers against them are
illustrative, not meaningful, until real speech is used.

## Replacing them with real recordings

1. Record 10–15 candidate-answer clips as **mono 16 kHz 16-bit PCM WAV**, covering
   pauses, filler words, background noise, accents, and very short / very long
   answers.
2. Drop the `.wav` files into this directory.
3. For each, add a row to `manifest.yaml`:
   ```yaml
   - id: <slug>
     file: <slug>.wav
     reference_transcript: "the exact words spoken"
     tags: [accent, noise, long]   # any of: clean fillers noise accent short medium long
   ```
4. Delete the synthetic clips and their manifest rows.

`tools/gen_bakeoff_clips.py` regenerates the synthetic set.

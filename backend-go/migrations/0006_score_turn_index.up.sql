-- Links a score row to the response turn it scored (NULL = the session's
-- overall score, computed once all turns are scored). `model` records which
-- judge LLM produced the score, for audit/debugging (docs/PLAN.md Phase 2).
ALTER TABLE scores
    ADD COLUMN turn_index int NULL,
    ADD COLUMN model text NOT NULL DEFAULT '';

-- At most one score per (session, turn): re-scoring a turn overwrites, it
-- doesn't accumulate. Partial index so NULL (overall) turns are unconstrained.
CREATE UNIQUE INDEX scores_session_turn_idx
    ON scores (session_id, turn_index)
    WHERE turn_index IS NOT NULL;

-- At most one overall score per session, same reasoning.
CREATE UNIQUE INDEX scores_session_overall_idx
    ON scores (session_id)
    WHERE turn_index IS NULL AND rubric_item = 'overall';

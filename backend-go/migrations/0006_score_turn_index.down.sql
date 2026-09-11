DROP INDEX IF EXISTS scores_session_overall_idx;
DROP INDEX IF EXISTS scores_session_turn_idx;
ALTER TABLE scores
    DROP COLUMN model,
    DROP COLUMN turn_index;

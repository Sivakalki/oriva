DROP INDEX IF EXISTS interview_sessions_org_id_idx;
ALTER TABLE interview_sessions DROP COLUMN IF EXISTS org_id;

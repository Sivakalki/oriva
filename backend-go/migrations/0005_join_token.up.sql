ALTER TABLE interview_sessions ADD COLUMN join_token text;
UPDATE interview_sessions
   SET join_token = replace(gen_random_uuid()::text, '-', '') || replace(gen_random_uuid()::text, '-', '')
 WHERE join_token IS NULL;
ALTER TABLE interview_sessions ALTER COLUMN join_token SET NOT NULL;
ALTER TABLE interview_sessions
  ADD CONSTRAINT interview_sessions_join_token_key UNIQUE (join_token);

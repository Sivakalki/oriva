-- Max interview duration, set by the recruiter at schedule time. The AI
-- service uses this to pace/wrap up the conversation; the candidate's call
-- screen shows a countdown against it.
ALTER TABLE interview_sessions
    ADD COLUMN duration_minutes int NOT NULL DEFAULT 30;

ALTER TABLE interview_sessions
    ADD COLUMN org_id uuid NOT NULL REFERENCES organizations (id);
CREATE INDEX interview_sessions_org_id_idx ON interview_sessions (org_id);

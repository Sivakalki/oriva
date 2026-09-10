CREATE TABLE session_state_events (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES interview_sessions (id),
    org_id     uuid NOT NULL REFERENCES organizations (id),
    from_state text NOT NULL REFERENCES session_states (name),
    to_state   text NOT NULL REFERENCES session_states (name),
    reason     text NOT NULL DEFAULT '',
    actor      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX session_state_events_session_id_idx ON session_state_events (session_id, created_at);

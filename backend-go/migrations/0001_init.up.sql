-- Draft schema for the AI interview platform. Refined in later slices as the
-- AI service solidifies (docs/PLAN.md Phase 0).

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE organizations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE jobs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations (id),
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_org_id_idx ON jobs (org_id);

CREATE TABLE candidates (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations (id),
    email       citext NOT NULL,
    name        text NOT NULL DEFAULT '',
    resume_text text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, email)
);

-- Canonical session-state list and legal transition graph (docs/ARCHITECTURE.md
-- section 4). Go is the sole authority; it loads these at startup. Seeded in 0002.
CREATE TABLE session_states (
    name        text PRIMARY KEY,
    label       text NOT NULL,
    is_terminal boolean NOT NULL DEFAULT false
);

CREATE TABLE session_state_transitions (
    from_state text NOT NULL REFERENCES session_states (name),
    to_state   text NOT NULL REFERENCES session_states (name),
    PRIMARY KEY (from_state, to_state)
);

CREATE TABLE interview_sessions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id       uuid NOT NULL REFERENCES jobs (id),
    candidate_id uuid NOT NULL REFERENCES candidates (id),
    state        text NOT NULL REFERENCES session_states (name),
    scheduled_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX interview_sessions_candidate_id_idx ON interview_sessions (candidate_id);
CREATE INDEX interview_sessions_job_id_idx ON interview_sessions (job_id);
CREATE INDEX interview_sessions_state_idx ON interview_sessions (state);

CREATE TABLE responses (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES interview_sessions (id),
    turn_index int  NOT NULL,
    question   text NOT NULL,
    answer     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (session_id, turn_index)
);

CREATE TABLE scores (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  uuid NOT NULL REFERENCES interview_sessions (id),
    rubric_item text NOT NULL,
    value       numeric,
    rationale   text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX scores_session_id_idx ON scores (session_id);

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations (id),
    email         citext NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role          text NOT NULL CHECK (role IN ('scheduler', 'candidate')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX users_org_id_idx ON users (org_id);

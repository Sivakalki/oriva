-- Seed the 12 canonical states and the legal transition graph
-- (docs/ARCHITECTURE.md section 4).

INSERT INTO session_states (name, label, is_terminal) VALUES
    ('scheduled',   'Scheduled',   false),
    ('invited',     'Invited',     false),
    ('ready',       'Ready',       false),
    ('dispatched',  'Dispatched',  false),
    ('in_progress', 'In progress', false),
    ('completed',   'Completed',   false),
    ('scoring',     'Scoring',     false),
    ('scored',      'Scored',      true),
    ('declined',    'Declined',    true),
    ('abandoned',   'Abandoned',   true),
    ('interrupted', 'Interrupted', false),
    ('failed',      'Failed',      true);

INSERT INTO session_state_transitions (from_state, to_state) VALUES
    -- happy path
    ('scheduled',   'invited'),
    ('invited',     'ready'),
    ('ready',       'dispatched'),
    ('dispatched',  'in_progress'),
    ('in_progress', 'completed'),
    ('completed',   'scoring'),
    ('scoring',     'scored'),
    -- consent / no-show
    ('ready',       'declined'),
    ('invited',     'abandoned'),
    ('ready',       'abandoned'),
    -- connection drop + the single rejoin edge
    ('dispatched',  'interrupted'),
    ('in_progress', 'interrupted'),
    ('interrupted', 'dispatched'),
    -- terminal technical failure from any non-terminal active state
    ('scheduled',   'failed'),
    ('invited',     'failed'),
    ('ready',       'failed'),
    ('dispatched',  'failed'),
    ('in_progress', 'failed'),
    ('completed',   'failed'),
    ('scoring',     'failed'),
    ('interrupted', 'failed');

CREATE TABLE test_sessions (
 id text PRIMARY KEY,
 name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
 project text NOT NULL,
 standalone boolean NOT NULL,
 started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 finished_at timestamptz,
 sample_interval_seconds integer NOT NULL CHECK (sample_interval_seconds BETWEEN 5 AND 60),
 CHECK ((project<>'' AND NOT standalone) OR (project='' AND standalone)),
 CHECK (finished_at IS NULL OR finished_at>=started_at)
);
CREATE UNIQUE INDEX test_sessions_active_scope_idx ON test_sessions(project,standalone) WHERE finished_at IS NULL;
CREATE INDEX test_sessions_retention_idx ON test_sessions(finished_at);
CREATE TABLE session_members (
 session_id text NOT NULL REFERENCES test_sessions(id) ON DELETE CASCADE,
 container_id text NOT NULL,
 name text NOT NULL,
 service text NOT NULL,
 first_observed_at timestamptz NOT NULL,
 PRIMARY KEY(session_id,container_id)
);
CREATE INDEX session_members_container_idx ON session_members(container_id);

CREATE TABLE alert_rules (
 project text PRIMARY KEY,
 enabled boolean NOT NULL,
 cpu_enabled boolean NOT NULL,
 memory_enabled boolean NOT NULL,
 cpu_threshold double precision NOT NULL CHECK(cpu_threshold>0 AND cpu_threshold<=100000),
 memory_threshold double precision NOT NULL CHECK(memory_threshold>0 AND memory_threshold<=100),
 nonzero_exit boolean NOT NULL,
 oom boolean NOT NULL,
 unhealthy boolean NOT NULL,
 enabled_since timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE local_alerts (
 id text PRIMARY KEY,
 project text NOT NULL,
 container_id text NOT NULL,
 name text NOT NULL,
 service text NOT NULL,
 kind text NOT NULL,
 evidence_from timestamptz NOT NULL,
 at timestamptz NOT NULL,
 last_observed_at timestamptz NOT NULL,
 value double precision,
 threshold double precision,
 detail text NOT NULL,
 resolved_at timestamptz,
 resolution text NOT NULL DEFAULT '',
 dismissed_at timestamptz,
 source_event_id text UNIQUE
);
CREATE INDEX local_alerts_project_idx ON local_alerts(project,last_observed_at DESC);
CREATE INDEX local_alerts_review_idx ON local_alerts(last_observed_at DESC) WHERE dismissed_at IS NULL;
CREATE TABLE alert_conditions (
 project text NOT NULL REFERENCES alert_rules(project) ON DELETE CASCADE,
 container_id text NOT NULL,
 kind text NOT NULL,
 state jsonb NOT NULL,
 alert_id text,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(project,container_id,kind)
);

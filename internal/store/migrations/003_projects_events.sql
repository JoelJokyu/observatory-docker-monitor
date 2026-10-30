ALTER TABLE containers ADD COLUMN project text NOT NULL DEFAULT '';
ALTER TABLE containers ADD COLUMN service text NOT NULL DEFAULT '';
CREATE TABLE container_events (
 id text PRIMARY KEY,
 container_id text NOT NULL,
 name text NOT NULL,
 project text NOT NULL,
 service text NOT NULL,
 action text NOT NULL,
 at timestamptz NOT NULL,
 exit_code text NOT NULL,
 signal text NOT NULL
);
CREATE INDEX container_events_time_idx ON container_events(at DESC,id);
CREATE INDEX container_events_project_idx ON container_events(project,at DESC);
CREATE INDEX container_events_container_idx ON container_events(container_id,at DESC);

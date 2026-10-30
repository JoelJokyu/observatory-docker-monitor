CREATE TABLE containers (
 id text PRIMARY KEY,
 name text NOT NULL,
 image text NOT NULL,
 state text NOT NULL,
 seen_at timestamptz NOT NULL,
 present boolean NOT NULL DEFAULT true
);
CREATE TABLE samples (
 container_id text NOT NULL REFERENCES containers(id),
 at timestamptz NOT NULL,
 cpu_percent double precision CHECK (cpu_percent >= 0),
 memory_bytes bigint NOT NULL CHECK (memory_bytes >= 0),
 memory_limit_bytes bigint NOT NULL CHECK (memory_limit_bytes >= 0),
 PRIMARY KEY (container_id, at)
);
CREATE INDEX samples_retention_idx ON samples(at);
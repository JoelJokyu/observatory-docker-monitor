# Observatory — Docker monitoring for local development

A local-first Docker monitoring and test investigation tool for individual developers and small teams. One Go application serves an embedded React/TypeScript dashboard; PostgreSQL stores resource/event history, sessions and local alerts. Licensed under [MIT](LICENSE); dependencies retain their own [notices](THIRD_PARTY_NOTICES.md).

## Installation and release status

**Source is available at [JoelJokyu/observatory-docker-monitor](https://github.com/JoelJokyu/observatory-docker-monitor). Prebuilt images and downloadable releases have not been published yet.** The planned user installation is a small ZIP plus prebuilt Linux amd64/arm64 images. No compilation or separate database installation is required for that route. See [user installation instructions](packaging/INSTALL.md) and [maintainer release steps](docs/releases.md). The local test bundle in dist/ references a local test image and cannot yet be downloaded/run by other people.

For contributing/building from source, use the instructions below and [CONTRIBUTING.md](CONTRIBUTING.md). Deployment boundaries and reporting setup are in [SECURITY.md](SECURITY.md). Host compatibility is verified separately from merely building an image architecture.

## Build from source with Docker Desktop

Use Docker Desktop in **Linux container mode**, with the Engine running. Clone the source, then open the project directory:

```powershell
git clone https://github.com/JoelJokyu/observatory-docker-monitor.git
cd observatory-docker-monitor
```

From that directory:

```powershell
docker compose up --build -d
docker compose logs -f app
```

Open http://localhost:8080. No manual psql or table creation is required: Compose provisions a dedicated PostgreSQL database, and the application applies the versioned migration transactionally on startup.

Your chat containers can run under a different Compose project on the same Engine. Start them as usual; discovery includes all containers on that Engine, including this platform's own containers. Images alone do not produce measurements: a container must exist, and resource samples require it to be running.

The first CPU reading is unknown; allow two successful collection cycles. Stopped/removed containers retain history. A recreated container gets a new identity even if its name is unchanged.

Stop the platform without removing its data:

```powershell
docker compose down
```

The database uses its own named volume. Avoid `down -v` unless intentionally deleting this project's stored history. The default database password is for local development only; change it before the first initialization if needed. Updating it after initialization also requires updating the database role, not just an environment variable.

## What this slice does

- Discovers running and stopped containers through the Docker Go SDK.
- Samples running containers every 10 seconds, with up to four concurrent Docker requests and per-request timeouts.
- Calculates CPU as a percentage of one core; a multi-core container can exceed 100%.
- Reports Linux memory usage excluding inactive file cache when available, plus the Docker-reported memory limit. That limit may reflect the Engine/VM ceiling rather than an explicitly configured container limit.
- Stores container identity and one row per timestamp with CPU/memory measurements.
- Collects network receive/send totals, block read/write totals, and PIDs when Docker reports those counters. Computes per-second I/O rates from comparable consecutive observations; first samples, observed resets, and topology changes have unknown rates.
- Serves container status and historical bucket means through net/http.
- Presents a React/TypeScript dashboard with dark/light themes, search and state filters, a resizable container detail panel, and resource/I/O charts with fifteen-minute through seven-day windows. Unknown and stale values remain explicit.
- Deletes samples older than seven days in bounded batches every minute. Retention is eventual; a large backlog can take multiple passes. Empty removed-container identities expire after seven days; existing stopped containers remain discoverable.
- Logs collection/storage errors and exposes platform readiness through `/api/health`.

CPU is null after startup, a counter reset, a failed collection cycle, or an unusable previous sample. Failed delivery is not replaced by zeros. Samples are written directly to PostgreSQL; there is no durable delivery buffer or automatic backfill. Database failures can lose observations, and subsequent CPU collection starts from a new baseline.

## Local Go development

Requires Go 1.25.6 or newer and a reachable Linux Docker Engine. Start just the database:

```powershell
docker compose up -d db
$env:DATABASE_URL = 'postgres://observatory:local-development-only@localhost:55432/observatory?sslmode=disable'
# When Docker Desktop uses a named pipe, specify its endpoint explicitly.
$env:DOCKER_HOST = 'npipe:////./pipe/dockerDesktopLinuxEngine'
go run ./cmd/observatory
```

The SDK reads DOCKER_HOST and TLS-related Docker environment variables; it does not automatically resolve Docker CLI contexts. The containerized setup configures the Unix socket explicitly. Local Go execution binds to 127.0.0.1:8080 by default.

Environment variables:

| Variable | Default | Meaning |
| --- | --- | --- |
| DATABASE_URL | Required | PostgreSQL connection string |
| LISTEN_ADDR | 127.0.0.1:8080 | HTTP listener; Compose overrides it inside the container |
| SAMPLE_INTERVAL | 10s | Collection interval, allowed range 5s–1m |
| DOCKER_HOST | SDK platform default | Docker Engine endpoint |

## Checks

```powershell
go test ./...
go vet ./...
```

PostgreSQL integration tests require a running test database. They create an isolated temporary schema, apply migrations, verify persistence/query/retention behavior, and remove that schema afterward:

```powershell
$env:TEST_DATABASE_URL = 'postgres://observatory:local-development-only@localhost:55432/observatory?sslmode=disable'
go test -v ./internal/store
```

Without TEST_DATABASE_URL, that integration test explicitly skips. Unit tests cover CPU/memory normalization, resets, collection failure/recovery, freshness, and history validation. Test success alone does not establish real Docker compatibility or performance capacity.

## API

- `GET /api/health`: database connectivity and collector state; returns 503 when degraded or stale.
- `GET /api/containers`: inventory, latest samples, collection interval, and collector state.
- `GET /api/containers/{id}/history?from=RFC3339&to=RFC3339`: recent history; defaults to the last hour. Maximum window is seven days. Responses aggregate to roughly 600 buckets and identify the bucket width, count, and first/last observation times.

CPU, memory, PIDs, and I/O-rate chart values are bucket means; they can hide short peaks and small gaps inside a bucket. The API exposes coverage metadata but the first dashboard is not a complete gap-analysis tool. Unknown IDs return empty history. There is no pagination or arbitrary query language in this slice.

## Development boundary

The dashboard and database are published only on localhost. Authentication, TLS, remote Engines, log ingestion/archiving, traces, alerts, and performance benchmarks are not implemented.

The application currently runs as root inside its local development container to reach the Docker socket. A read-only socket mount does **not** restrict Docker API permissions: access to the socket can grant broad control of the Engine. The collector only invokes listing and stats operations, but that is not an enforced authorization boundary. Do not expose this development setup publicly. Revisit socket isolation and process privileges before a release.

## Design and evidence

See `docs/architecture.md` for responsibilities, data flow, decisions, and verification status. The root `plan.md` tracks the wider project plan. No throughput, latency, or supported-capacity results have been claimed.
## Updating to the first presentable milestone

From this project directory:

```powershell
docker compose up --build -d app
```

The build uses Node 24 to compile React/TypeScript, copies the generated assets into the Go binary, and deploys the same monolith. The first rebuild downloads the Node build image and frontend dependencies. There is no extra frontend service in the running deployment.

Migration 002 adds nullable I/O/PID columns automatically; existing history and the database volume are preserved. Old measurements cannot be backfilled and display unknown for the newly introduced fields. Allow two successful collection cycles for the first I/O rates. If an Engine does not expose a counter, it remains unknown rather than becoming zero.

The table shows cumulative NET I/O (receive/send) and BLOCK I/O (read/write), in binary byte units. These counters belong to the current container run. The Network & disk tab charts rates in bytes per second. PIDs include processes and threads, matching the Linux cgroup counter. Block I/O is not filesystem capacity.

Click a container to inspect it in the side panel. Close with Escape; resize using the divider or its left/right arrow keys. On smaller screens, details open as a sheet. Theme and desktop panel width are saved locally. New measurements continue to follow Docker IDs across recreation.

## Frontend development

Node 24 LTS is recommended. The machine's existing Node 20.18 is too old for this Vite version; Docker builds provide Node 24 independently, so Docker-only users do not need to update their host Node installation.

```powershell
cd frontend
npm ci
npm run dev
```

The Vite development server uses http://127.0.0.1:5173 and proxies `/api` to the running Go app on port 8080. For a local Go build after frontend changes, run `npm run build` first; generated assets live under `internal/httpapi/web` and are included in the Go binary. The checked-in generated assets also allow ordinary Go builds without a frontend toolchain.

```powershell
npm run build
npm test
```

React sources are in `frontend/src`, including the main application/detail panel, chart component, API types, formatting/freshness helpers, and theme tokens. Charts use SVG paths and preserve gaps between missing observations. Tests check missing versus zero, stale-data handling, and chart gap segmentation.
## Milestone 2 — Docker log snapshots

The container detail panel now has a Logs tab. It reads a bounded snapshot directly from Docker for the selected resource-investigation window. The snapshot displays exact time bounds and fetch time, and only updates when opened, when the selected duration changes, or when Refresh logs is clicked. Resource auto-refresh does not follow logs.

There is no log database, schema migration, backfill, or independent retention. Running and stopped containers work when Docker can still serve their output. Removed containers keep resource history but no Observatory log archive. Docker logging-driver configuration and rotation govern what is available.

The default is up to 500 Docker tail lines, with bounded text/wire size and a deadline. TTY output is explicitly combined; otherwise stdout/stderr are labeled without inferring severity. Multiline and script-like content render as text. Oversized output is marked partial/truncated, and missing coverage is never presented as complete history.

See `docs/logs-api.md`, `docs/decisions/0002-docker-log-snapshots.md`, and `demo/README.md` for the contract, accepted design, and repeatable test workload. The current plan is `plan.md` in this project root.

Keep the Compose project name unchanged when reusing an existing local database volume.

## Projects and lifecycle events

Use the **Project** selector to focus the inventory and summary on a Compose application, or select Standalone containers. Open the clock icon in the left navigation for its recorded event timeline, or the **Events** tab in a container's side panel. Starts, exits, removals, health changes and OOM events appear with timestamps and available exit/signal evidence. Events are retained for seven days; Docker's limited replay can leave collection gaps. An empty timeline is not a complete audit result.

`GET /api/events` supports bounded project/container/time queries. See [event behavior and API](docs/events.md) for the contract and an isolated demo. Named sessions, investigation tools, local alerts and session comparison are implemented; see [plan.md](plan.md) for the milestone record.

## Named test sessions

The flask icon opens **Test sessions**. Choose a project, start a named session, run your own test, then finish it to review per-container resource statistics and lifecycle counts. New/replacement containers in that project join automatically. A session survives Observatory restarts and remains active until finished. One can be active per project at a time.

Completed session metadata expires seven days after finish. Summaries use the underlying seven-day monitoring history and disclose older unavailable portions and collection limitations. No log archive is added. See [session behavior and API](docs/sessions.md).

## Investigate an event or session

Use **View resources** or **View logs** on an event or session member. The side panel keeps the selected time window fixed across resources and logs; **Return to recent history** restores relative controls. Search and stream filters apply only to loaded log lines, and **Copy matching lines** includes their timestamps/streams. Docker still owns log retention. See [investigation behavior](docs/investigation.md).

## Local project alerts

Open the bell and choose a project, then **Configure alerts** to opt in. CPU/memory thresholds require 30 seconds of consecutive recorded evidence. Nonzero exits, OOM and unhealthy Docker events can also create alerts. Dismiss acknowledges an occurrence; it does not claim recovery. The bell counts undismissed alerts across projects, and entries link to resources/logs for investigation. No notifications leave the app. See [rules, API and limitations](docs/alerts.md).


## Compare completed sessions

Open **Test sessions → Compare sessions**, select baseline A and another completed run B in the same project. Compare per-container CPU mean/peak, memory peak and lifecycle counts with duration, sample counts and retained windows alongside. Replacement IDs stay separate; missing values remain unknown. Links reopen each run’s resources/logs in its own fixed time window. Match your workloads before interpreting differences. See [comparison definitions](docs/comparison.md).

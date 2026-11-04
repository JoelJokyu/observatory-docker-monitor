# Observatory — Docker monitoring for local development

A local-first Docker monitoring and test investigation tool for individual developers and small teams. One Go application serves an embedded React/TypeScript dashboard; PostgreSQL stores resource/event history, sessions and local alerts. Licensed under [MIT](LICENSE); dependencies retain their own [notices](THIRD_PARTY_NOTICES.md).

## Installation and release status

Download the numbered installation ZIP and checksum from [GitHub Releases](https://github.com/JoelJokyu/observatory-docker-monitor/releases). Follow its `INSTALL.md`, copy `.env.example` to `.env`, set a unique database password, then run `docker compose pull` and `docker compose up -d`. Open http://localhost:8080. This route requires Docker/Compose, with no Go/Node compilation or separate database installation.

Images target Linux amd64 and arm64. Both passed anonymous installation checks on native Linux CI. Locally built images passed Windows Docker Desktop checks, but pulling the published GHCR image failed on the maintainer's Windows installation; that unresolved limitation is documented in the [release notes](https://github.com/JoelJokyu/observatory-docker-monitor/releases/tag/v0.1.0). macOS host compatibility remains unverified. The source-build route is below. See [installation details](packaging/INSTALL.md) and [maintainer release steps](docs/releases.md).

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

Your application containers can run under a different Compose project on the same Engine. Start them as usual; discovery includes all containers on that Engine, including this platform's own containers. Images alone do not produce measurements: a container must exist, and resource samples require it to be running.

The first CPU reading is unknown; allow two successful collection cycles. Stopped/removed containers retain history. A recreated container gets a new identity even if its name is unchanged.

Stop the platform without removing its data:

```powershell
docker compose down
```

The database uses its own named volume. Keep the Compose project name unchanged when reusing an existing volume. Avoid `down -v` unless intentionally deleting this project's stored history. The default database password is for local development only; change it before the first initialization if needed. Updating it after initialization also requires updating the database role, not just an environment variable.

## Features

- Discovers running and stopped containers through the Docker Go SDK.
- Samples running containers every 10 seconds, with up to four concurrent Docker requests and per-request timeouts.
- Calculates CPU as a percentage of one core; a multi-core container can exceed 100%.
- Reports Linux memory usage excluding inactive file cache when available, plus the Docker-reported memory limit. That limit may reflect the Engine/VM ceiling rather than an explicitly configured container limit.
- Stores container identity and one row per timestamp with CPU/memory measurements.
- Collects network receive/send totals, block read/write totals, and PIDs when Docker reports those counters. Computes per-second I/O rates from comparable consecutive observations; first samples, observed resets, and topology changes have unknown rates.
- Serves container status and historical bucket means through net/http.
- Presents a React/TypeScript dashboard with dark/light themes, search and state filters, a resizable container detail panel, and resource/I/O charts with fifteen-minute through seven-day windows. Unknown and stale values remain explicit.
- Deletes samples older than seven days in bounded batches every minute. Retention is eventual; a large backlog can take multiple passes. Empty removed-container identities expire after seven days; existing stopped containers remain discoverable.
- Groups containers by Compose project and records lifecycle/health events.
- Reads bounded Docker log snapshots with message search, stream filtering and copying.
- Records named test sessions and compares completed sessions using per-container observations and lifecycle counts.
- Provides project-specific opt-in alerts for sustained CPU/memory thresholds, nonzero exits, OOM and unhealthy events, with dismissal and investigation links.
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

Without TEST_DATABASE_URL, the PostgreSQL integration tests explicitly skip. Unit tests cover CPU/memory normalization, resets, collection failure/recovery, freshness, and history validation. Test success alone does not establish real Docker compatibility or performance capacity.

## API

- `GET /api/health`: database connectivity and collector state; returns 503 when degraded or stale.
- `GET /api/containers`: inventory, latest samples, collection interval, and collector state.
- `GET /api/containers/{id}/history?from=RFC3339&to=RFC3339`: recent history; defaults to the last hour. Maximum window is seven days. Responses aggregate to roughly 600 buckets and identify the bucket width, count, and first/last observation times.
- `GET /api/containers/{id}/logs`: bounded on-demand Docker output; see [log API](docs/logs-api.md).
- `GET /api/events`: recorded lifecycle/health events with project, container and time filters; see [event API](docs/events.md).
- `GET/POST /api/sessions`, `GET /api/sessions/{id}` and `POST /api/sessions/{id}/finish`: list, start, inspect and finish test sessions; see [session API](docs/sessions.md).
- `GET /api/alerts`, `POST /api/alerts/{id}/dismiss` and `GET/PUT /api/alert-rules`: alert history, acknowledgment and per-project settings; see [alert API](docs/alerts.md).

CPU, memory, PIDs, and I/O-rate chart values are bucket means; they can hide short peaks and small gaps inside a bucket. The API exposes observation counts and time bounds, which do not establish continuous coverage. Unknown IDs return empty resource history. Event, session and alert lists have documented result limits; there is no arbitrary query language.

## Deployment scope

Both supplied Compose setups bind the dashboard to localhost. The source-build setup also publishes PostgreSQL on localhost; the release bundle keeps PostgreSQL internal. Authentication, application TLS termination, remote Engine deployments, archived logs, traces and performance benchmarks are outside the current release. Local project alerts and on-demand Docker log snapshots are implemented.

The application runs as root in its container to reach the Docker socket. A read-only socket mount does **not** restrict Docker API permissions: socket access can grant broad control of the Engine. Observatory uses Docker read APIs for discovery, resource statistics, events and log snapshots; this application behavior is not an enforced authorization boundary. The release Compose file adds a read-only app filesystem, dropped capabilities and no-new-privileges. Keep the installation local and review [SECURITY.md](SECURITY.md) for deployment limits, the Linux kernel advisory and private vulnerability reporting.

## Design and evidence

See `docs/architecture.md` for responsibilities, data flow, decisions, and verification status. The root `plan.md` tracks the wider project plan. No throughput, latency, or supported-capacity results have been claimed.

## Rebuilding from source

From this project directory:

```powershell
docker compose up --build -d app
```

The build uses Node 24 to compile React/TypeScript, copies the generated assets into the Go binary, and deploys the same monolith. An uncached build downloads the build images and frontend dependencies. There is no extra frontend service in the running deployment.

Versioned database migrations run automatically on startup. Back up before upgrading; follow the [release installation and upgrade guide](packaging/INSTALL.md) for prebuilt installations. Measurements missing from older history are not backfilled. Allow two successful collection cycles for the first I/O rates. If an Engine does not expose a counter, it remains unknown rather than becoming zero.

The table shows cumulative NET I/O (receive/send) and BLOCK I/O (read/write), in binary byte units. These counters belong to the current container run. The Network & disk tab charts rates in bytes per second. PIDs include processes and threads, matching the Linux cgroup counter. Block I/O is not filesystem capacity.

Click a container to inspect it in the side panel. Close with Escape; resize using the divider or its left/right arrow keys. On smaller screens, details open as a sheet. Theme and desktop panel width are saved locally. New measurements continue to follow Docker IDs across recreation.

## Frontend development

Use Node 24 for frontend development. Docker builds provide their own Node toolchain; running a prebuilt release does not require Node on the host.

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

## Docker log snapshots

The container detail panel has a Logs tab. It reads a bounded snapshot directly from Docker for the selected resource-investigation window. The snapshot displays exact time bounds and fetch time, and only updates when opened, when the selected duration changes, or when Refresh logs is clicked. Resource auto-refresh does not follow logs.

Log snapshots have no independent database or retention. Running and stopped containers work when Docker can still serve their output. Removed containers keep resource history but no Observatory log archive. Docker logging-driver configuration and rotation govern what is available.

The default is up to 500 Docker tail lines, with bounded text/wire size and a deadline. TTY output is explicitly combined; otherwise stdout/stderr are labeled without inferring severity. Multiline and script-like content render as text. Oversized output is marked partial/truncated, and missing coverage is never presented as complete history.

See `docs/logs-api.md`, `docs/decisions/0002-docker-log-snapshots.md`, and `demo/README.md` for the contract, accepted design, and repeatable test workload. The current plan is `plan.md` in this project root.

## Projects and lifecycle events

Use the **Project** selector to focus the inventory and summary on a Compose application, or select Standalone containers. Open the clock icon in the left navigation for its recorded event timeline, or the **Events** tab in a container's side panel. Starts, exits, removals, health changes and OOM events appear with timestamps and available exit/signal evidence. Events are retained for seven days; Docker's limited replay can leave collection gaps. An empty timeline is not a complete audit result.

`GET /api/events` supports bounded project/container/time queries. See [event behavior and API](docs/events.md) for the contract and an isolated demo. The following sections describe sessions, investigation tools, local alerts and comparison; see [plan.md](plan.md) for the development record.

## Named test sessions

The flask icon opens **Test sessions**. Choose a project, start a named session, run your own test, then finish it to review per-container resource statistics and lifecycle counts. New/replacement containers in that project join automatically. A session survives Observatory restarts and remains active until finished. One can be active per project at a time.

Completed session metadata expires seven days after finish. Summaries use the underlying seven-day monitoring history and disclose older unavailable portions and collection limitations. No log archive is added. See [session behavior and API](docs/sessions.md).

## Investigate an event or session

Use **View resources** or **View logs** on an event or session member. The side panel keeps the selected time window fixed across resources and logs; **Return to recent history** restores relative controls. Search and stream filters apply only to loaded log lines, and **Copy matching lines** includes their timestamps/streams. Docker still owns log retention. See [investigation behavior](docs/investigation.md).

## Local project alerts

Open the bell and choose a project, then **Configure alerts** to opt in. CPU/memory thresholds require 30 seconds of consecutive recorded evidence. Nonzero exits, OOM and unhealthy Docker events can also create alerts. Dismiss acknowledges an occurrence; it does not claim recovery. The bell counts undismissed alerts across projects, and entries link to resources/logs for investigation. No notifications leave the app. See [rules, API and limitations](docs/alerts.md).

## Compare completed sessions

Open **Test sessions → Compare sessions**, select baseline A and another completed run B in the same project. Compare per-container CPU mean/peak, memory peak and lifecycle counts with duration, sample counts and retained windows alongside. Replacement IDs stay separate; missing values remain unknown. Links reopen each run’s resources/logs in its own fixed time window. Match your workloads before interpreting differences. See [comparison definitions](docs/comparison.md).

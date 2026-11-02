# Verification — October 23, 2026

## Passed in this session

- `go test -v ./...`: seven top-level unit/API tests passed, including four CPU edge-case subtests. The PostgreSQL test skipped explicitly.
- `go vet ./...`: passed.
- `node --check internal/httpapi/web/app.js`: passed.
- `docker compose config --quiet`: passed. This validates configuration, not image builds or running services.
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath ./cmd/observatory`: passed, with output kept outside deliverables.

## Not verified

- Docker image build or Compose startup.
- Live collection from the user's Engine or chat application.
- Real PostgreSQL migrations, queries, retention, and restart persistence.
- Browser rendering and interaction.
- Seven-day workload behavior, latency, throughput, disk usage, or memory overhead.

Docker was initially stopped. After the user started Docker Desktop, the CLI reported permission denied connecting to the Engine's Windows named pipe. This session's permission tool could not represent that pipe path, so live checks could not proceed. No Engine configuration was changed to bypass that restriction.

## Next local checks

From the project directory:

```powershell
docker compose up --build -d
docker compose logs -f app
```

Visit http://localhost:8080, allow two collection cycles, and compare readings with `docker stats --no-stream`. CPU readings can differ because the sampling windows differ; agree on tolerances before treating this as a correctness check.

Run the PostgreSQL integration test separately:

```powershell
$env:TEST_DATABASE_URL = 'postgres://observatory:local-development-only@localhost:55432/observatory?sslmode=disable'
go test -v ./internal/store
```

Then check application/database restart persistence, container stop/recreation, collection disconnect/recovery, and controlled load. Record observations as evidence rather than assuming a successful build establishes these behaviors.

## Subsequent live checks — user-observed

The owner subsequently ran the platform locally and reported these results in chat. These supersede the earlier lack of live evidence for the specific behaviors below; they were not independently executed by the agent.

- Chat containers appeared in inventory.
- Stopping a chat container showed exited state, unknown current CPU/memory, and stale last-sample age.
- Starting the same container resumed collection; the stopped period appeared as a blank chart gap.
- Restarting the Observatory app preserved earlier history and resumed collection.
- Recreating the chat container produced a separate running ID; the previous ID showed Removed and retained its earlier history.
- Stopping PostgreSQL caused a stale-data indication. Starting PostgreSQL restored collection without restarting Observatory, with earlier history retained and an outage gap reported.
- A Docker stats spot comparison matched the dashboard CPU and memory readings. This is a spot check, not a measured accuracy assessment over varied workloads.

Still pending: the automated PostgreSQL integration test, controlled-load and counter-reset tests against a real Engine, long-retention verification, quantified performance targets, and broader UI review. The dashboard was observed working by the owner, but no agent browser inspection has been performed.

## PostgreSQL integration — user-executed pass

The owner ran `go test -v ./internal/store` with TEST_DATABASE_URL set. TestPostgresLifecycle passed in 0.14 seconds; the package reported PASS. This supersedes the earlier skipped/pending integration-test status. It verifies the test's isolated-schema migration, duplicate-sample handling, history aggregation, retention deletion, and recreation identity assertions. It does not establish performance capacity or seven days of real-world operation.

## Controlled CPU load — user-observed pass

The owner ran the bounded Alpine CPU test with a 0.5-core CPU limit and a 64 MiB memory limit. The container ran for approximately 60 seconds and terminated as expected. The dashboard reported six observations in ten-second buckets, with the latest CPU bucket mean at 49.8% and memory at approximately 0.7 MiB. The observed CPU is consistent with the configured half-core limit under the one-core percentage definition. This validates a single controlled-load scenario, not general measurement accuracy or performance capacity.

The basic functional checks now have user-observed evidence for discovery, stop/start and chart gaps, app restart persistence, recreation identity/history preservation, database outage/recovery, a Docker stats spot comparison, and controlled CPU load. The user-executed PostgreSQL integration test passed. Broader load, long-duration retention, security, and release-readiness checks remain outstanding.

## Product milestone 1 implementation checks

Implemented React/TypeScript presentation, dark/light themes, container search and state filters, a resizable side panel, resource and I/O tabs, and NET I/O/BLOCK I/O/PID collection, storage, and retrieval.

Passed against the local PostgreSQL instance in isolated test schemas:

- Fresh migration/lifecycle checks, including I/O/PID sample roundtrips and aggregated history.
- Upgrade from schema version 1 to 2, preserving prior CPU/memory history and leaving new fields null on old samples.
- Collector tests covering I/O summation, per-second calculations, missing versus zero, resets, topology changes, overflow rejection, and invalidated rate baselines after database inventory failures.
- HTTP tests, including serving the embedded React entry point and generated assets.
- Frontend TypeScript production build and three tests for presentation/freshness/gap semantics.

Final build, static, and Compose checks are recorded from their actual command results. No performance capacity has been inferred from these tests.

Browser verification of the new interface remains pending. The in-app browser reached the existing old dashboard on port 8080, but replacement preview servers were unreachable. A static fixture page was prepared as a testing intermediate, clearly marked as simulated, but could not be opened through the browser policy; it is not a demonstrated UI result and is not shipped in the app. Temporary preview servers were stopped. No fake measurements were added to the application or its database.

The owner's next action is `docker compose up --build -d app`, followed by refreshing localhost:8080. New I/O counters require successful live sampling; old history remains unknown for those fields. Confirm the side-panel layout, theme toggle, resource charts, and Network & disk tab before accepting milestone 1. The owner was asked to rebuild while final checks completed; no rebuild outcome has been reported yet.

## Milestone 2 — independent implementation and live checks, October 24, 2026

The owner approved Docker-backed on-demand snapshots and Refresh, with no independent log storage. The authoritative source was copied into the current workspace; the original generated workspace remains a preserved baseline. No PostgreSQL log schema or data-volume reset was introduced.

Passed:

- Full Go unit/API suite and go vet. New cases cover SDK-compatible framing, split records, TTY/empty output, line/text/wire bounds, oversized declared frames, invalid framing, cancellation/body closure, Docker options, failure classification, range validation, non-caching, and a four-request snapshot limit.
- Frontend TypeScript/Vite production build and eight tests, including selected-window alignment, safe text rendering, combined/partial provenance, and full-clock timestamp formatting.
- PostgreSQL lifecycle and upgrade-preservation regression tests against the local database in isolated schemas.
- Production multi-stage Docker build and startup of the app with its existing database volume. The database had been stopped; it was started without resetting data.
- Live Docker snapshots: stopped workload readable; eight lines across stdout/stderr; time-window filtering isolated two final events; an earlier empty range returned zero lines; TTY returned two combined lines; disabled logging returned HTTP 422.
- Live resource correlation: three history buckets from the controlled workload, with an observed peak CPU of 49.90906731009235%. The workload was deliberately capped at half a core. This is a scoped functional check, not a capacity result.
- Existing chat history remained accessible: a two-day query returned thirty earlier buckets.
- Browser checks: real Logs tab, multiline and script-like text, TTY explanation, unsupported-driver error, explicit refresh, and time-window selection. Visual review found a fractional-seconds-only timestamp bug; it was fixed, tested, rebuilt, and confirmed to show full local time. Screenshot: ../outputs/logs-snapshot.jpg.

Two verification-harness issues were corrected without changing product behavior: an inherited PowerShell native exit code caused a false failure after successful API checks, and a narrow legacy-history test range used an ambiguous date conversion. The final preservation check used explicit UTC bounds and passed.

Resource polling does not automatically follow logs. Snapshot coverage is limited by Docker, and stopped/removed containers are not treated as an independent log archive. Severity inference, streaming, persistent search, and log ingestion remain outside this milestone. Owner acceptance is still pending.

### Milestone 2 cleanup and final state

The isolated `observatory-log-demo` project was removed with its temporary volumes. A subsequent request for its removed workload's logs returned HTTP 404, while three resource-history buckets remained accessible. Observatory's app and existing PostgreSQL service remain running; no chat containers were started, stopped, or reconfigured during these checks.

The browser was left on the running Observatory app's Logs tab, which returned its own current Docker output with full local timestamps. Manual Refresh and changing the selected time window both produced new bounded snapshots. The screenshot file shows actual demo output before cleanup, not mock data.

## Final About-modal adjustment

Replaced the below-page About card with a centered native dialog. Verified the close button, Escape, backdrop dismissal, focus restoration to the trigger, and preservation of an open container details sheet. The production frontend and Docker builds passed and the updated local app is running. Screenshot: ../outputs/about-modal.jpg. No backend behavior or database schema changed. The owner requested this as the final UI change for the batch.

## October 26, 2026 — milestone 3, batch 1: projects and events

Implemented the approved first batch in the existing Go monolith, PostgreSQL and React/TypeScript frontend. Migration 003 adds project/service metadata and selected lifecycle events. No log archive, session storage, alert rules or comparison feature was introduced.

Checks completed:

- Full Go suite and `go vet ./...` passed. Event tests cover normalization and excluded arbitrary metadata, nanosecond replay identity, seconds-only timestamps, invalid/expired events, durable-cursor overlap, cancellation, failed-write cursor preservation, and recovery after a simulated Docker handshake failure. The inventory test verifies Compose labels reach persistence.
- Eleven frontend tests passed, including escaped event names, factual labels/exit evidence and navigation only for known inventory IDs. TypeScript/Vite and multistage Docker production builds passed.
- Real PostgreSQL integration tests passed in isolated temporary schemas: project/event roundtrip, replay deduplication, project/standalone/container filters, newest-first/half-open range/limit semantics, seven-day cleanup, event-only project discovery, existing resource lifecycle and upgrade preservation. Three schema migrations are recorded. The existing application database volume was preserved.
- A separate Compose demo (`observatory-event-demo`) produced create/start events, a healthy-to-unhealthy transition, and an intentionally short-lived service exit with code 3. The live API returned the expected metadata and project filters without unrelated events.
- Restarting Observatory retained the short-lived service's same three event IDs with no added duplicates. Removing demo containers produced destroy events and retained the exit evidence. The project's name remained selectable through stored events. A separately created standalone fixture passed standalone filtering. Both fixture groups were removed afterward; no user chat containers were altered.
- Browser review verified project-scoped inventory/summary, project event timeline, per-container Events tab, full clock timestamps, health/exit labels and retained events after fixture removal at the browser's default narrow size. A keyboard check exposed an existing tab handler using the selected tab rather than focused tab; corrected it to navigate from the focused tab. Final keyboard/focus and screenshot checks are recorded below.

Limits: Docker daemon disconnection and database write failures are simulated in collector tests, not a destructive live outage exercise in this batch. No deliberate OOM, long-duration retention run or capacity benchmark was performed. Docker replay remains bounded; complete recovery/delivery is not promised. Removed legacy identities from before migration 003 have blank project metadata because their old labels cannot be recovered. Those appear under Standalone.

Final browser checks passed against the deployed build: arrowing from a focused Logs tab selects Events; Escape closes the container panel; on the next animation frame keyboard focus returns to the event's Inspect button. The initial immediate focus read occurred before that frame; a subsequent DOM check and screenshot confirmed the focused button. The narrow timeline and side-panel layouts were inspected at the default browser size without resizing. A full-page screenshot request was unsupported; a normal screenshot was successfully saved to outputs/event-timeline.jpg. The final source and Docker builds contain the same generated asset index-Bv7vSVWN.js. Batch 1 is deployed locally and ready for owner review; later batches are queued.

## October 26, 2026 — milestone 3, batch 2: named test sessions

The owner approved including new/replacement containers in the same project automatically and expiring completed sessions seven days after finish, with older unavailable telemetry disclosed. Migration 004 and the session API/UI are deployed in the existing application/database. Existing telemetry and database volume were preserved.

Verification evidence:

- Full Go unit/API suite and `go vet ./...` passed. Session API checks cover input/body limits, origin/media guards, fresh-inventory requirement, fixed result bounds, error classification and finishing independently of Docker health.
- All 14 frontend tests passed. New cases verify elapsed duration, escaped container names, explicit zero versus unknown values, statistics definitions and retention-loss disclosure. TypeScript/Vite production build and multistage Docker build passed; final generated app asset is index-CPvf5lRV.js.
- All six real PostgreSQL integration tests passed using isolated temporary schemas. Session checks cover start snapshot, separate replacement IDs, short-lived and late-replayed event membership, exclusion of unrelated and post-finish containers, exact half-open resource/event boundaries, CPU known-count/mean/peak semantics, concurrent start protection, retry-stable finish, reopened-store persistence, standalone scopes, active/finished retention behavior and null values. Existing resource lifecycle, events and migration preservation checks also passed with four migration records.
- A browser-created isolated observatory-session-demo session included the original API, its replacement and a brief worker intentionally exiting 7. A bounded CPU workload produced a recorded per-container peak of approximately 22.91% under a 0.25-core limit. This is a scoped functional observation, not a benchmark or capacity guarantee. Forced replacement also legitimately produced a nonzero shutdown exit; the UI labels these as nonzero exits, without assuming crashes.
- The session remained active with the same member IDs after Observatory restarted. Duplicate start was rejected with 409. The final browser-finished session contained three IDs, 82 raw resource observations and 14 recorded lifecycle events. Its original start/finish boundaries persisted on reopen. Repeating finish preserved the timestamp. A subsequent worker and demo removal did not change its member IDs, sample count or event count.
- Browser review covered project selection, the start/name modal, active-session listing, summary, restart/reopen, finish, completed listing and Escape cancellation of an unsubmitted form. Replacement-name review exposed Docker Compose's temporary create name; successful in-window inventory now refreshes displayed member names/services, verified by a database regression and the final UI. Resource staleness in an active summary also checks inventory freshness, not only its previous healthy flag.
- The worker's lack of resource observations is shown as zero observations and unknown CPU/memory, rather than fabricated measurements. The two API IDs remain separate despite sharing a final name. Screenshot at outputs/test-session.jpg shows the completed summary in the browser's existing narrow viewport; no viewport override was used.
- All session-demo containers/network were removed. The completed demo session and recorded telemetry remain reviewable within their retention rules. Existing user chat workloads were not changed. App/database remain running from the current workspace.

Limits: expiry and late-replay edge cases use isolated database fixtures rather than a multi-day live run. No continuous-coverage, time-weighted average, complete membership, reliable event delivery, performance/capacity or immutable summary guarantee is made. Docker replay and collection outages can leave gaps; session duration includes downtime. Logs are still Docker-backed snapshots without an archive. Event/log navigation, alerting and comparison remain queued.

## October 28, 2026 — milestone 3, batch 3: investigation tools

Implemented frontend-only event/session navigation to fixed absolute resource/log windows plus snapshot message search, stream filters and copying of matching lines. No migration, backend service/API or log archive was added. Existing native detail panels, cancellation and bounded Docker reads remain in use.

All 17 frontend tests passed. New tests cover event context clipping to now/retention, expired/malformed timestamps, literal multiline case-insensitive filtering, combining stream and text filters, snapshot immutability and copying provenance/partial lines. TypeScript/Vite and Docker production builds passed. The full Go suite and vet passed after the generated embedded assets stabilized; an initial test run overlapped frontend asset replacement and failed to find the old generated filename, then passed when rerun sequentially. Final deployed asset: index-CyJ7LM1R.js.

Live browser evidence using isolated observatory-investigation-demo:

- API start event opened four stdout/stderr lines in its labelled two-minutes-either-side context. Resource and log tabs showed the identical fixed interval; the relative selector was disabled. Current-reading cards are hidden in the final fixed view.
- Searching BETA with stderr selected produced exactly one loaded row. Copy matching lines returned `2026-10-28T10:59:10.213668074Z [stderr] beta warning`, with no stdout lines. Search with no matches displayed the loaded-snapshot-only explanation.
- Script-like text and multiline continuation were displayed plainly. TTY output retained its combined-stream notice and filtering returned one combined entry after returning to recent history and refreshing. A narrower event context that excluded the initial TTY output correctly returned no rows; it was not treated as a collection failure.
- Return to recent history removed the pinned context and re-enabled relative time controls.
- A completed demo session's exact two-second available window was preserved between resource and log tabs. Its short window did not invent observations outside its boundaries.
- An earlier completed session's short-lived worker, absent from current inventory, opened as Recorded and queried Docker, which returned the actual not-found/no-archive explanation. No false 1970 inventory-seen date or current reading was shown.
- Screenshot: outputs/investigation-tools.jpg. Browser viewport was unchanged. The isolated demo containers were removed after verification; its completed session and resource/event evidence remain within retention, while Docker logs are no longer available after removal.

Limits: no new capacity or complete-coverage guarantee; tests do not establish that all Docker logging configurations support reads. Clipboard failure has a manual-copy fallback but OS-level denial was not forced. Search covers only the currently loaded bounded snapshot, and copied lines can contain partial output markers. Earlier live data and logs may differ in retention; correlation is not causality. Local alerts and comparisons remain queued.

## October 28, 2026 — milestone 3, batch 4: project alerts

Owner selected per-project opt-in with event alerts and 30-second sustained resource evidence. Implemented rule settings, inbox, global undismissed review badge/banner, dismissal, recovery/retirement states, retained investigation context and migration 005. No external messages, browser push/OS notification permission, new service, queue or log archive was introduced.

Verification:

- Full Go suite and vet passed. Condition tests cover sustained duration, duplicate/out-of-order observations, known recovery, unknown data and gaps. HTTP checks cover scope, body/threshold/media/origin validation, result bounds and non-caching.
- All 20 frontend tests passed, including factual/escaped alert presentation, stale evidence versus dismissal/recovery, rule-change wording, bounded investigation ranges and preserving small thresholds. A live display check found 0.01% rounded to 0%; significant-digit threshold formatting corrected this, with a regression assertion. TypeScript/Vite and Docker builds passed. Final embedded frontend asset: index-1xp51WKJ.js.
- All nine real PostgreSQL integration tests passed in isolated temporary schemas. They cover opt-out defaults, duration/gaps/unknown limits, raw-sample idempotence, episode suppression, dismissal across adapter reopening, known recovery/new breach, disabled settings, event replay/source-ID deduplication, event-only identities, unhealthy grouping, lifecycle retirement versus health recovery, standalone/project scope, expiry and prior telemetry/session/event preservation. Five migration records are now expected. Retention deletes also recheck mutable timestamps outside the bounded subquery to protect concurrently refreshed rows.
- Isolated observatory-alert-demo was limited to 0.25 cores/32 MiB. Browser enabled only that project with deliberately low demo thresholds (CPU 5%, memory 0.01%). Other scopes remained disabled. The bounded 85-second CPU workload produced an alert after a measured 39.894248-second span of high observations (at least the required 30 seconds, with collection timing jitter). This verifies behavior, not load capacity or a latency guarantee. Memory readings also breached the deliberately tiny test threshold; no unsafe memory stress was needed.
- Real Docker unhealthy and intentional worker exit 7 produced one alert each. Four live kinds were verified: CPU, memory, unhealthy and nonzero exit. OOM behavior uses event/database fixtures; no live OOM was forced. Normal exits do not create nonzero-exit alerts, and intentional stops can legitimately be nonzero.
- Browser dismissed the ongoing unhealthy episode: needs-review decreased from four to three while the episode stayed unresolved. Restart/replay preserved the four IDs and the dismissal. Touching the healthy fixture flag recovered that occurrence; a new unhealthy event produced a new undismissed ID. CPU/memory did not multiply their episodes. CPU eventually recovered when load ended. The final demo history has five occurrences.
- A stale resource episode remained unresolved with an explicit missing/stale-evidence notice. Browser disabled the demo and restored 80% default thresholds; its open memory episode closed as Rule changed, not Recovered. Alert-to-resource/log navigation kept its fixed context. Default application projects (rtchat and go-observability) remained opted out.
- Demo alerts were acknowledged and its containers/network removed. Historical alerts remain available with Include dismissed. Existing user workloads and PostgreSQL volume were preserved. Screenshot: outputs/local-alerts.jpg.

Limits: no long-running retention/capacity benchmark, deliberately induced database outage, live OOM or proof of complete event delivery. Expiry and out-of-order/replay edge cases use database fixtures. Pending duration relies on sampled evidence and does not prove continuous coverage; resource/event collection failures can lose observations. Current active resource conditions need known recovery and are not silenced by missing data; badge counts represent undismissed reviews, including resolved alerts. New configuration resets episode evidence as documented. Session comparison is the only remaining batch.


## October 30, 2026 — milestone 3, batch 5: session comparison

- Added a React comparison view using two existing session-summary reads. No backend/schema change. Same exact scope, distinct completed IDs; member rows join only by full container ID. Unknown, zero, nonmembership and limited-list absence have different presentation.
- All 25 frontend tests passed. Five new tests exercise identity/replacement matching, invalid active/identical/cross-scope pairs, escaped names, explicit zero versus null, retention/member limits, raw lifecycle counts and per-run investigation availability. TypeScript/Vite production and Docker builds passed. Go suite and vet passed with generated assets. No new database migration: prior nine isolated integration checks were not rerun in this frontend batch.
- Real isolated observatory-comparison-demo: baseline d6f3239e695240cf521a07b5d89bbcc8 lasted 31 seconds with one ID/three samples; comparison 01e86f6cd1b246c9dfa8efecc6c8f66b lasted 36 seconds with two IDs/four samples. Shared ID CPU mean/peak was 0%/0% for A and approximately 18.9%/25% for B. Replacement’s first CPU reading remained unknown, not zero. B recorded eight lifecycle events and a factual nonzero exit during intentional recreation; A recorded none. The fixture was limited to 0.25 core/32 MiB and removed afterwards. Raw API evidence: work/comparison-live.json (ignored development evidence).
- Browser confirmed same-scope B options while browsing All projects, old results/B clearing after baseline change, empty same-scope options notice, run-B fixed resource/log bounds, and removed-container log explanation. Completed comparisons still opened after fixture cleanup. Narrow browser layout revealed inherited table nowrap; explicit normal wrapping and scoped spacing corrected it. Final frontend asset: index-BgE8RUaY.js. Screenshot: outputs/session-comparison.jpg.

Limits: two independent database snapshots with separate check times, not an atomic comparison/archive. No inferred workload equality, time-weighted means, service aggregation, summed project maxima, duration normalization, score, automatic verdict or capacity benchmark. First/last observations do not quantify gap coverage. Retention and limited Docker replay still affect evidence. Frontend failure/cancellation paths are implemented but no forced live storage outage or 500-member load test was performed.


## October 30, 2026 — first public release preparation

Owner selected MIT and deferred repository creation/naming. Added MIT LICENSE (Observatory contributors), contribution/security guidance, third-party notices for 31 application/runtime dependencies and a regeneration tool. No public repository, registry upload or GitHub Release was created.

Release packaging is separate from the developer Compose stack: prebuilt image, internal-only PostgreSQL, localhost dashboard, independent fixed project/volume, required per-install password, configurable host port/socket, app read-only filesystem, dropped capabilities and no-new-privileges. Socket access remains powerful and is documented. Dockerfile builds native tooling once and cross-compiles amd64/arm64 runtimes, embeds version/revision metadata and license notices; --version needs no database.

Verification:

- Three packaging tests passed: fixed file allowlist/no .env or working data, deterministic ZIP/checksum, version/reference rejection and digest acceptance. Initial test execution could not access this session’s temporary folder; running with the appropriate temporary-folder access passed. The final ZIP was regenerated from current templates/notices. Local test image reference only: observatory-release-test:0.1.0. This is not a public downloadable installation.
- Both Linux amd64 and arm64 images built and loaded as a local multi-platform image. On Windows Docker Desktop, native amd64 and emulated arm64 each passed a disposable clean release start, automatic five-migration initialization, /api/health readiness, dashboard HTTP response and version output. The app ran with release read-only/capability settings and collected through the mounted socket.
- Each architecture test stopped its app, created a binary-format pg_dump, copied it without shell binary redirection, restored into a separate disposable database and checked a seeded value plus migration count. Recreating the app preserved the seeded data and returned to readiness. Both random test projects, networks and database volumes were removed; developer app/database and user workloads were preserved.
- Full Go suite (including the nine real PostgreSQL integration checks in isolated schemas) and vet passed. All 25 frontend tests passed. The frontend was unchanged.
- Workflow YAML parsed/formatted successfully. actionlint 1.7.12 validated both CI and release workflows without errors (optional external shellcheck/pyflakes disabled). CI prepares native amd64/arm64 checks; release publication requires an explicit manual invocation and an existing numbered source tag. Only the future publishing job has package-write permission. Bundle uses the actual published multi-platform digest.

Remaining external checks: create the public repository when the owner is ready, establish private reporting, execute native GitHub CI, review/pin action/base-image versions under the intended release policy, publish the image/package, verify public anonymous pulls and attach a reviewed bundle to a GitHub Release. Native macOS/Apple Silicon/Linux ARM host compatibility has not been established by emulation. No capacity/security audit or universal compatibility claim. Installation/backup/restore/upgrade/removal contract: packaging/INSTALL.md; release operation: docs/releases.md.


## November 2, 2026 — Observatory 0.1.0 published

Private vulnerability reporting was enabled and verified through the GitHub API. Public reporting entry: https://github.com/JoelJokyu/observatory-docker-monitor/security/advisories/new. The owner had already configured repository topics; these were left as configured.

Source tag v0.1.0 points to adfd66e5bca98806e7b436c32b7d9dea1819f2ae. Tagged-source Checks and Prepare numbered release succeeded on native amd64 and arm64. Build/publication run: https://github.com/JoelJokyu/observatory-docker-monitor/actions/runs/41453851351. The public multi-platform image is ghcr.io/joeljokyu/observatory-docker-monitor:0.1.0, index digest sha256:6f7cba98bec4e34e0f0b2f3d788520934aa64db73d7a6200a76363e345d05d23. Bundle release.json and .env.example pin that digest. OCI provenance/SBOM were requested by the release workflow; no separate signing/audit guarantee is claimed.

Additional anonymous-install verification was added on main without changing the source tag. Fresh native Linux amd64 and arm64 machines pulled the exact published digest using an empty registry-auth configuration and passed clean Compose startup, collection/database readiness, dashboard/version, five migrations, seeded-data persistence, binary-safe backup/restore and app recreation. Both disposable projects/volumes were cleaned up. Verification run: https://github.com/JoelJokyu/observatory-docker-monitor/actions/runs/41460009960.

Published release: https://github.com/JoelJokyu/observatory-docker-monitor/releases/tag/v0.1.0. It contains observatory-0.1.0.zip and SHA256SUMS. Both assets were downloaded without authentication after publication, and the downloaded ZIP matched its checksum and the verified build artifact: 0444c4be5c2942100474324ff5c1419d03332096ee8a46b565784c53cf82e7a4. This is the real registry-pinned public bundle, distinct from the earlier local test ZIP under dist/.

Known limitation: Windows Docker Desktop 29.9.2 on this computer returned denied for a full GHCR image pull, including with a task-owned empty auth configuration. Anonymous HTTP reads of index/tag/digest, platform manifest and config blob succeeded; full anonymous installations also passed on both native Linux runners. The owner confirmed that the container package was already public. The cause of the local Docker Desktop error is unresolved; no package permissions, saved registry credentials or host security/network settings were changed to bypass it. Docker Desktop was started for the check. The failure is disclosed in the public release notes; public-image installation on this Windows host is not claimed verified. Earlier locally built-image checks are historical evidence only.

CVE-2026-98432 is disclosed in release notes, SECURITY.md and installation guidance. The owner chose to await the Linux-side fix; no application MPTCP, kernel or host settings were changed. This release does not claim immunity. macOS host verification remains unavailable.

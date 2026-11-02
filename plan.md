# Observatory — Project Plan



Updated: October 28, 2026



## Product direction — approved



Build an open-source, local-first observability tool for solo developers, students with their first substantial application, and small engineering teams. A developer runs Observatory alongside their Docker applications to inspect container resource use and investigate local problems.



Product promise: understand what your Docker applications are doing locally without setting up a large monitoring stack.



The initial target remains one local Linux Docker Engine, including Docker Desktop in Linux-container mode. The initial design workload is approximately 5–10 containers with seven days of resource history. These are design targets, not verified capacity claims.



Open-source distribution and a hosted service are separate decisions. The direction is source distribution and self-hosting. License choice, repository publication, contribution policy, and release readiness remain to be discussed.



## Collaboration contract



The owner is the architect and makes final product, architecture, technology, data-flow, reliability, and scaling decisions. The assistant may research, propose alternatives, explain tradeoffs, implement approved designs, create tests, and improve the UI.



For significant decisions: explain the problem and constraints, present a few viable approaches, recommend when appropriate, obtain the owner's choice, and record the rationale before dependent implementation. Explain routine implementation choices that materially affect behavior. Do not silently introduce storage systems, brokers, service boundaries, or delivery guarantees.



Use manageable milestones rather than a fixed timeline. Keep manual testing to one step at a time when working with the owner.



## Workspace and existing source



The authoritative application source is now the root of this selected workspace:



The current Observatory source checkout.



The approved baseline was copied here without dependency folders, secrets, or Git metadata. The old generated workspace remains an unchanged baseline, not an active parallel checkout. Compose retains its project name and named database volume. No database reset or data relocation was performed.



## Status conventions



- Proposed: recommendation or scope awaiting a decision.

- Approved: explicitly accepted by the owner.

- Implemented: present in code; does not imply verification.

- Verified: backed by checks or observations, with the source of evidence stated.

- Accepted: owner agrees that the milestone meets its intended purpose.



## Approved architecture baseline



| Decision | Direction | Rationale |

| --- | --- | --- |

| Runtime | Go monolith with separate internal responsibilities | Keeps a single-Engine product operationally simple. |

| HTTP | Go net/http | Fits the owner's preference and current requirements. |

| Resource collection | Docker Engine API through an established Go client | Reuses Docker's interface while keeping collection behavior understandable. |

| Storage | Plain PostgreSQL | Stores container identity and historical samples together; specialize only when evidence warrants it. |

| Frontend | React and TypeScript | Explicitly requested by the owner. |

| UI | Dark/light themes and a container detail side panel | Accepted as the first presentable milestone. |

| Deployment | Docker Compose, one Go app and PostgreSQL | Frontend builds into the Go binary; no separate frontend runtime service. |



No broker, separately deployed collector, distributed storage, or hosted control plane has been selected.



## Milestone 1 — Presentable local Docker monitoring



Status: implemented and accepted by the owner after a successful local rebuild and UI review.



Implemented capabilities:



- Discover running and stopped containers on one Engine.

- Collect CPU, memory, network receive/send totals, block read/write totals, and PIDs where reported.

- Calculate I/O rates from valid consecutive observations; preserve unknown values for missing counters, initial rates, observed resets, or changed counter identities.

- Preserve distinct histories for recreated container IDs.

- Store resource samples in PostgreSQL with versioned migrations and bounded seven-day retention cleanup.

- Expose inventory, history, and platform health through HTTP APIs.

- Display resource and I/O-rate charts with a container side panel, time windows, search, filters, dark/light themes, and stale-data indications.



Evidence already recorded:



- Owner-observed discovery, stop/start behavior and chart gaps, app restart persistence, container recreation identity/history, and database recovery without app restart.

- A Docker stats CPU/memory spot comparison matched the dashboard.

- A bounded half-core CPU workload produced a reported 49.8% CPU bucket mean.

- Go tests and static checks passed; PostgreSQL lifecycle and schema-upgrade preservation tests passed against a real local database in isolated schemas.

- React/TypeScript production build and frontend behavior tests passed.

- The owner reported the rebuilt UI was much better and accepted milestone one.



Limits: UI acceptance is not an independent verification of every new I/O counter. No sustained capacity, latency budget, long-duration retention behavior, or release security guarantee has been established.



## Milestone 2 — Investigate a container using resources and logs



Status: implemented, verified, and closed after the owner's final About-modal adjustment.



Desired investigation: select a container, inspect a resource change, choose its time period, and inspect that container's available logs from the same period. Correlating a time window is useful evidence, not automatic proof of causality.



### Proposed minimum experience



- A Logs tab in the existing container side panel.

- Reuse the selected container ID and investigation time window.

- Show available timestamps and distinguish stdout/stderr when Docker exposes that distinction.

- Treat stderr as a stream, not automatically as an error severity.

- Bound requests and displayed output by agreed time, line, byte, and duration limits.

- Clearly distinguish no matching output, an unavailable container, unsupported log access, truncation, and a collection/connection failure.

- Render log content as untrusted plain text; preserve multiline output without interpreting it as HTML.

- Define how TTY output is represented when separate streams are unavailable.

- Test using a deterministic demo with resource pressure and identifiable log events.



Persistent search, live following, a streaming transport, pagination, JSON interpretation, and severity inference are deferred. The first implementation displays bounded plain-text lines and explicit stream provenance. In particular, searching loaded lines must not be presented as searching an entire retained log history.



### First architecture decision — approved: Docker-backed on-demand snapshots



Problem: determine who owns log retention and where log queries execute.



| Approach | Data path | Benefit | Tradeoff |

| --- | --- | --- | --- |

| Read from Docker on demand | Dashboard → Go API → Docker logs API | Smallest operational change; no duplicate log store or new ingestion lifecycle | Availability and retained coverage depend on Docker; no independent archive when Docker can no longer serve the logs. |

| Ingest and retain logs in Observatory | Docker → collection pipeline → chosen storage → Go query API | Can preserve already collected logs independently and support queries over Observatory's retained data | Requires decisions on storage, retention, checkpoints, duplicates, loss, backpressure, restarts, and resource limits. |



Approved decision: use bounded on-demand Docker reads for this local-first milestone. The owner confirmed that independent log preservation is unnecessary for active testing. Ingestion remains a future alternative if requirements change.



Docker's log-reading support depends on the logging driver and local cache configuration. Unsupported access must be explained without modifying the user's Docker settings automatically. The resource history retention window does not guarantee equivalent Docker log coverage.



Sources:



- https://docs.docker.com/engine/logging/dual-logging/

- https://docs.docker.com/engine/logging/configure/

- https://docs.docker.com/reference/api/engine/version-history/



### Subsequent decisions, in order



1. Finalize required log lifetime and choose on-demand reads versus ingestion.

2. Define initial query semantics and resource bounds, including truncation and supported log sources.

3. If ingestion is chosen, decide storage and failure/recovery semantics before writing the pipeline.

4. Define the Go API contract and the resource-chart-to-log interaction.

5. Decide whether live following belongs in this milestone and select a transport only if it does.

6. Implement the approved design in the existing monolith, then verify one scenario at a time.



### Proposed acceptance criteria



- A developer can inspect available logs for an existing container in the selected resource-history time window.

- The interface states missing coverage, unsupported access, and limits honestly.

- Query cancellation and agreed bounds prevent an unbounded response or stream.

- Special characters and multiline output are rendered safely.

- Container restarts, removal/recreation, and Docker disconnection behave according to the approved log-retention model.

- Prior resource monitoring behavior and stored history remain intact.

- A repeatable demo illustrates the investigation without fabricated performance or correctness claims.

- The owner reviews and accepts the milestone.



## Open-source readiness work



Prepare these alongside feature work, without treating them as permission to change architecture:



- Reconcile the project location and establish one authoritative source checkout.

- Review the existing code and document component responsibilities.

- Verify setup from a clean environment and list supported environments.

- Provide reproducible demo workloads and troubleshooting instructions.

- Document measurement definitions, failure behavior, and known limitations.

- Choose a license with the owner before claiming an open-source release.

- Establish repository organization, contribution guidance, and release checks.



Release security decisions remain open, including Docker socket isolation and access permissions. Current local bindings and broad socket privileges are development assumptions, not production hardening.



## Deferred directions



Application tracing, richer endpoint metrics, application-health probes, multi-Engine monitoring, team roles, multi-tenancy, Kubernetes support, and high availability remain candidates. Select future work by a concrete investigation or demonstrated need.



OpenTelemetry remains an evaluation candidate for application telemetry; it has not been selected as a complete platform architecture.



## Immediate next action



Milestone 3 is approved below. Batches 1 and 2 are implemented and ready for owner review; investigation tools are next. The source workspace is reconciled; logs remain Docker-backed without independent storage.



## Milestone 2 decision record — approved October 24, 2026



The owner selected Docker as the source of logs because the tool is intended for active local testing and independent log preservation is not required. Running and stopped containers are supported when Docker can still serve their logs. Removed containers keep resource history but have no Observatory log archive.



The owner approved a bounded snapshot for the selected time window and an explicit Refresh button. Live following and log ingestion are deferred. No PostgreSQL log schema or new storage component is introduced.



Initial implementation bounds, explained before coding: default/latest 500 lines, maximum 1,000 requested lines, 512 KiB decoded log text, 1 MiB wire read, an eight-second backend deadline, and at most four concurrent snapshot requests. Responses must disclose partial output and the difference between truncation and a tail-limit indication. These are adjustable implementation defaults, not independently approved capacity guarantees.



The application baseline has been copied into the selected workspace root, excluding generated dependency folders, .git, .env, and TypeScript build caches. This workspace is now the authoritative development location. The old workspace remains a preserved baseline and must not be edited as a parallel active copy. Compose keeps its existing project name and named database volume; no database data is moved or reset by the source copy.

## Milestone 2 verification summary



The Go unit/API suite and static checks passed. New log tests cover framed and TTY decoding, split records, limits, malformed/oversized frames, cancellation, error classification, exact query windows, non-caching, and concurrency admission. Eight frontend tests passed, including plain-text rendering and full-clock timestamp formatting. PostgreSQL lifecycle/upgrade regression tests passed against isolated schemas in the existing local database. The production React and Docker builds succeeded.



Live Docker/API checks returned eight demo log lines from a stopped container with stdout/stderr labels; time filtering isolated two completion events; an earlier empty window returned no lines; a TTY container returned two combined lines; a disabled logging driver returned 422. The demo's resource history recorded three buckets and a peak CPU reading of approximately 49.91% under the deliberate half-core limit. Thirty earlier chat-history buckets were still accessible in a two-day query. These are scoped functional observations, not performance/capacity benchmarks.



Browser checks verified the Logs tab, plain-text script-like content, full local timestamps, combined TTY notice, unsupported-driver explanation, explicit Refresh, and selected time-window changes. Screenshot evidence is in outputs/logs-snapshot.jpg. Demo cleanup and the removed-container outcome are recorded in docs/verification.md. The owner still needs to accept the milestone.

Final cleanup verification: removing the isolated demo caused Docker log access to return 404 for the removed workload, while its three resource-history buckets remained available. The production app and existing database are running from this workspace. The owner can now review the Logs tab without rebuilding.



## Final batch UI adjustment — complete



The owner requested a simple About popup as the final change before closing this batch. The below-page information card was replaced with a centered native modal. It supports a close button, Escape, and backdrop dismissal; blocks background scrolling; and restores keyboard focus to the About button. An open container detail panel is preserved when About closes.



TypeScript/production and Docker builds passed. Browser checks verified the centered view, all dismissal methods, focus restoration, and preservation of the underlying container panel. Screenshot: outputs/about-modal.jpg. The local app was rebuilt and restarted with its existing database intact. This closes the requested implementation batch; the next milestone remains the owner's decision.





## Milestone 3 — Local testing sessions — approved October 26, 2026



The owner approved combining project grouping, events, named sessions, log investigation, local alerts, and session comparison into one milestone delivered in working batches. Sessions observe tests the developer runs; traffic generation is outside this milestone. Logs remain Docker-backed snapshots with no independent archive. Milestone 2 is closed following the owner's final UI review.



### Delivery batches



1. **Projects and events (implemented and verified):** group inventory using Compose project/service labels, retain standalone containers, add a project-filtered event timeline and a container Events tab. Capture selected lifecycle/health events in existing PostgreSQL with seven-day retention, replay deduplication, bounded queries, reconnect status and explicit coverage limitations.

2. **Named test sessions (implemented and verified):** start/finish a named session scoped to a project, record its boundaries and container membership, and show a resource/event summary. Decide treatment of newly created/replaced containers and session retention before implementation.

3. **Investigation tools (implemented and verified):** select an event/session time window, navigate to resources and available Docker logs, search/filter loaded log snapshots and copy relevant lines. Do not imply searching unavailable or archived logs.

4. **Local alerts (implemented and verified):** in-app notifications using recorded events and sustained resource thresholds. Define threshold duration, unknown-data behavior, dismissal and repeat suppression before implementation. No external messaging.

5. **Session comparison (implemented and verified):** compare two completed sessions with explicit duration, coverage and measurement definitions; avoid misleading averages across gaps or different workloads. Agree on comparisons before implementation.



### Batch 1 implementation defaults



Use the approved Go monolith and PostgreSQL. No new services. Project identity is Docker's exact Compose project label on the single monitored Engine; service names are metadata, and container IDs remain distinct after recreation. Unlabelled containers are Standalone, including stopped/removed identities within retention.



Capture create/start/stop/restart/die/destroy/kill/oom/pause/unpause/rename and standard health-status changes. Persist only selected identity, action, timestamp, exit-code and signal fields; do not persist arbitrary labels, commands, environment values or health-check output. Events are independently identified so short-lived containers need not have appeared in inventory first.



Subscribe to Docker events, overlap replay on reconnect, and deduplicate replay using event identity. Renew the stream every 30 seconds to bound stalled reads/handshakes. Retry failures without claiming complete delivery. Docker only replays a limited recent buffer: recorded history is not a complete audit trail. Connection failures appear separately from resource collection. Initial startup requests the preceding minute when no saved event exists; restart requests overlap the most recent saved event. Event query defaults to one hour/latest 200, maximum 500; report when more matches exist. Seven-day event retention shares the existing bounded cleanup cycle. These are implementation defaults, not capacity guarantees.



### Batch 1 acceptance



A developer can filter their app's containers and events by project, inspect one container's lifecycle, distinguish connection trouble from no recorded events, and see removed-container events without losing resource history. Verify migration/replay deduplication, scope/range/limit filters, retention, cancellation/retry, and real create/exit/remove scenarios. Review UI with keyboard and narrow-layout checks. Later batches remain queued until this batch is reviewable.


### Batch 1 delivery — implemented and verified October 26, 2026

Project-scoped inventory/summary and the workspace/container event timelines are deployed locally. Go tests/static checks, eleven frontend tests, PostgreSQL lifecycle/upgrade/event integration checks, production builds, and live Docker/API checks passed. Browser checks verified project scopes, per-container Events, keyboard arrows, Escape dismissal and return of focus to the event's Inspect button after the next animation frame. Narrow-screen layout was reviewed at the browser's existing size; no viewport override was used.

The isolated demo and standalone fixture have been removed; their recorded event evidence remains selectable within retention. Existing database volume and chat workloads were preserved. Screenshot: outputs/event-timeline.jpg. Detailed evidence and limits: docs/verification.md and docs/events.md.

Batch 1 is ready for owner review. Batch 2 (named sessions) is next; session membership/recreation and retention semantics will be discussed before its storage design is implemented.


## Milestone 3, batch 2 — named test sessions — approved October 26, 2026

The owner approved starting this batch, automatically including new/replacement container IDs in the selected Compose project, and expiring completed sessions seven days after finishing while marking aged-out measurements unavailable.

Implementation defaults explained before coding: one active session per project (or the explicit Standalone scope); sessions persist across Observatory restart and remain active until explicitly finished. Elapsed duration includes downtime. Sessions observe externally run tests and do not start, stop or load the user's containers. No logs are stored.

Use existing PostgreSQL and the Go monolith. Migration 004 adds session boundaries and independent member identity snapshots. Capture present project members at start, then include matching observed inventory and in-window Docker events, including late replay after finish. Full container IDs stay separate across recreation. Serialize membership/start/finish transactions with one advisory lock so concurrent commits do not skip membership. A partial unique index enforces one active session per scope; finishing is idempotent and preserves the first finish timestamp.

Summaries are calculated from currently retained telemetry rather than archived copies: per-container sample count, first/last sample, known CPU count, CPU sample mean/observed peak and observed peak memory, plus lifecycle action counts and nonzero exits. No project-wide peak is inferred by summing independent container maxima. Unknown measurements stay null. Report the available time range and retention loss explicitly; sampling gaps and bounded Docker replay prevent a complete-coverage claim. Running sessions older than seven days stay active but lose older telemetry and disclose that loss.

GET list defaults to latest 100, with truncation notice; details show at most 500 members with an explicit limit indication while summary counts cover all members. Start/finish require small JSON bodies and reject cross-site browser writes. Session start needs fresh inventory and at least one currently observed container in the selected scope; existing sessions can be finished during Docker trouble if PostgreSQL is reachable. Names are 1–80 characters. No sessions across All projects in this batch; choose one Compose project or Standalone.

Acceptance: start/name a session, observe resources and container events during a test, include a replacement/short-lived container, restart Observatory without losing an active session, finish and reopen it, verify exact boundaries and no fabricated values, and exercise retention with isolated database fixtures. Log navigation, alerts and comparison remain later batches.



### Batch 2 delivery — implemented and verified October 26, 2026

The named-session workflow is deployed locally. The owner approved automatic membership for new/replacement containers and expiry seven days after finish. Sessions persist across Observatory restart, keep container IDs independent, and expose sample-based resource statistics and recorded lifecycle counts with coverage/retention notices. The Sessions navigation uses the existing project selector; starting uses a centered native modal, and finished sessions can be reopened.

Verification: the full Go suite and vet passed, all 14 frontend tests passed, and all six PostgreSQL integration tests passed in isolated schemas. These cover concurrency, replay/membership boundaries, unknown values, expiry, name refresh and schema upgrade preservation. Production frontend/Docker builds passed. A real demo session started in the browser, included an original API plus replacement and short-lived worker, survived restart, and finished in the browser with three IDs, 82 observations and 14 lifecycle events. Finish retry kept its timestamp; a worker created after finish and subsequent cleanup events were excluded. All isolated demo containers were removed. The active app/database and existing volume remain intact.

Browser review confirmed list/start/name/finish/reopen behavior, duplicate-start prevention, Escape cancellation without creating a session, corrected replacement names, and the completed summary at the existing narrow browser size. Screenshot: outputs/test-session.jpg. Contracts: docs/sessions.md. Evidence and limits: docs/verification.md.

Batch 2 is ready for owner review. Batch 3 remains queued: event/session time-window navigation and search/filter/copy for loaded Docker log snapshots. Alerts and comparison are still later batches.

## Milestone 3, batch 3 — investigation tools — October 28, 2026

Owner authorized this batch. Add View resources/View logs to workspace events and each session member, preserving full container identity and returning to the existing side panel. Event context defaults to two minutes either side, clipped to now and seven-day retention; the label discloses this context. Sessions reuse their available_from/to snapshot, including partial retention disclosures. Investigation ranges stay fixed across resources/logs and refresh; Return to recent history restores ordinary relative windows. Recorded identities missing from inventory can still open history, while Docker log availability remains explicit.

Log search is literal, case-insensitive matching of messages in the loaded snapshot only. Stream filters include stdout, stderr and combined TTY. Copy matching lines exports only displayed matches with timestamp/stream provenance and partial-line markers, preserving multiline text. Plain-text rendering, output limits, cancellation and explicit Refresh remain. No new log storage, backend service, API or migration is introduced. Verification is pending below.


### Batch 3 delivery — implemented and verified October 28, 2026

Deployed event/session links into fixed resource/log windows, snapshot message search, stream filters and Copy matching lines. Pinned historical views hide live reading cards. Recorded-only identities retain investigation access without fabricated current inventory metadata. No backend, schema or log-retention change.

All 17 frontend tests passed; TypeScript/production and Docker builds passed. Go tests and vet passed after rebuilding embedded assets (an initial concurrent build/test attempt was retried after the asset output stabilized). Browser verification covered matching event resource/log bounds, exact session bounds across tabs, frozen selectors and unpin, case-insensitive combined filters, copied stderr provenance, no-match wording, combined TTY output, script-like plain text, and a removed event-only session member receiving Docker's actual not-found explanation. The final historical view hides current-reading cards. Screenshot: outputs/investigation-tools.jpg. Behavior: docs/investigation.md.

The isolated demo session is completed; its container fixtures are removed after verification. Existing app/database and user workloads are preserved. Batch 3 is ready for owner review. Remaining batches: 4 — local alerts; 5 — session comparison.

## Milestone 3, batch 4 — local alerts — in progress

The owner authorized starting Batch 4. Alert scope/default activation is awaiting the owner's choice: opt in per project (recommended) versus all-Engine activation by default. Proposed initial signals are nonzero exit, OOM and unhealthy events, plus adjustable CPU/memory thresholds sustained for 30 seconds. Nonzero exits must not be called unexpected crashes without evidence. Missing observations interrupt pending threshold evidence and must not resolve an active condition. Dismissal silences that occurrence; recovery followed by a new breach can alert again. These proposed semantics require confirmation before production wiring.

Scope-independent preparation: a deterministic condition evaluator and tests cover sustained consecutive observations, repeat suppression, known recovery, missing observations, collection gaps, duplicate samples and out-of-order replay. No alert migration, settings defaults, runtime integration or deployment has been made yet. Existing app behaviour is unchanged.

## Milestone 3, batch 4 — project alerts — approved October 28, 2026

Owner selected project-by-project opt-in. Initial signals: nonzero exit, OOM and unhealthy events; CPU and memory thresholds sustained for at least 30 seconds of consecutive observed evidence. The proposed missing-data/dismissal/recovery semantics were included in the owner's choice. Standalone containers may be explicitly opted in as their existing separate scope; there is no all-Engine default.

Implementation defaults: disabled until configured, CPU threshold 80% of one core, memory threshold 80% of Docker's reported limit, each signal independently switchable. Thresholds are adjustable; duration is fixed at 30 seconds. Consecutive observations may be at most 1.5 configured sampling intervals apart. Unknown readings or longer gaps reset pending evidence. Existing threshold episodes are not recovered by missing data and show stale evidence until fresh known recovery. Dismissal acknowledges an occurrence without changing condition state; fresh recovery followed by a new sustained breach creates another occurrence. Unhealthy repeats group until a healthy event or a container lifecycle change. Nonzero-exit/OOM alerts represent individual recorded events and deduplicate their source event IDs; they do not infer crashes.

Use migration 005 in existing PostgreSQL for per-project rules, alert episodes and serialized condition state. Alert evaluation commits with the corresponding sample/event under an advisory lock. A failed transaction does not commit partial alert evidence. Existing collection health reports persistence failure; there is no new queue or reliable-delivery promise. Settings changes retire previous resource/health episodes as rule_changed and reset state/activation time, without calling it recovery. Only observations at/after that save create alerts; no earlier history is backfilled. Active settings/condition/dismissal survive process restart.

Inbox: latest 100 matching alerts, needs-review count, project filter and Include dismissed. Global badge/banner counts undismissed alerts across all projects (including recovered ones that have not been reviewed). Poll every ten seconds independently of successful inventory polling. Inbox failure is visible, not a fabricated zero. Dismiss is idempotent; existing same-origin/JSON protections guard new writes. Entries expire after seven days without new high/event evidence; active fresh resource episodes update their latest evidence and remain visible. Rule configuration persists independently. Bounded cleanup removes expired alert/condition rows.

View resources/logs uses the Batch 3 fixed-window investigation mechanism. Event counts, stale evidence, exact timestamps and limitations stay explicit. No external messages, OS notifications, new services or log retention are introduced. Verification is in progress below.


### Batch 4 delivery — implemented and verified October 28, 2026

Project opt-in rules, in-app review badge/banner, alert inbox, dismissal/history, fixed-window investigation links and persisted threshold/health state are deployed. Migration 005 preserves existing resource/session/event data. Defaults remain off for every application project; the isolated demo rule was reset to disabled defaults after review.

Verification: Go suite and vet passed; all 20 frontend tests passed; all nine real PostgreSQL integration tests passed in isolated schemas, including migration/history preservation. Production frontend/Docker builds passed. Live demo captured one CPU episode after 39.894248 seconds of high recorded evidence, memory pressure under a deliberately tiny demo threshold, an unhealthy health check and a factual exit-7 event. Duplicate observations/restart retained occurrence IDs; browser dismissal persisted separately from health recovery; a later unhealthy event created a new occurrence. CPU recovered after the bounded load ended. Missing/stale memory evidence was not marked recovered. Disabling settings retired the open resource episode as Rule changed. OOM mapping was tested with database/event fixtures, without deliberately forcing a live OOM.

Browser checks covered initial opt-out, rule save, independent signal controls, accurate 0.01% display, badge/review counts, Include dismissed, investigation resources/log tabs and fixed time context. A display rounding defect for tiny thresholds was corrected and regression-tested. Final screenshot: outputs/local-alerts.jpg. Full behavior/API: docs/alerts.md. Detailed evidence/limits: docs/verification.md.

All isolated demo containers are removed and its five historical alerts are acknowledged; existing application workloads and database volume remain intact. Only Batch 5 — session comparison — remains in this milestone. Batch 4 is ready for owner review.


## Milestone 3, batch 5 — session comparison — approved October 30, 2026

Owner approved comparing two completed sessions from the same project or explicit Standalone scope. Show per-container CPU sample mean/peak and memory peak alongside duration, observation counts and lifecycle events. Full container IDs remain separate across replacements. No automatic performance score or verdict.

Use the existing bounded summary API and retention definitions; no schema change or new service. Select A (baseline) and B in Test sessions. Match by full ID only, with missing membership distinct from unknown readings. Show each run’s wall-clock duration, configured sample interval, retained time window, summary check time, sample/known-CPU counts and first/last observations. Explain missing data, unequal workloads/durations, sample means without time weighting, independent observed peaks and raw event counts. Limited member lists cannot prove absence. Investigation links preserve each run’s own exact available window.

Summaries are two independent database reads, not a shared snapshot or archive. Refresh comparison explicitly; ordinary inventory polling does not move a loaded comparison. Selection changes cancel pending reads and clear old results. Missing/expired/failed reads show no partial comparison. Session options remain limited to the existing latest 100 with project narrowing. Verification pending.


### Batch 5 delivery — implemented and verified October 30, 2026

Completed-session comparison is deployed locally: baseline A and same-scope B, per-full-ID resource evidence, independent retained windows/check times, raw lifecycle counts, explicit unknown/zero/absent/limited-list meanings and links into each run’s own fixed resource/log context. No new API, migration or service. The milestone’s five planned batches are implemented and ready for owner review.

Verification: 25 frontend tests passed, including five comparison tests for full-ID matching, invalid pairs, escaping, zero/unknown, retention/limits and investigation availability. TypeScript/production and Docker builds passed; Go suite and vet passed. The unchanged database contract was exercised by live real-session reads; this batch did not rerun the nine prior isolated database integration tests.

An isolated 0.25-core/32-MiB demo recorded completed baseline d6f3239e695240cf521a07b5d89bbcc8 (31 seconds, one ID, three samples) and comparison 01e86f6cd1b246c9dfa8efecc6c8f66b (36 seconds, two IDs, four samples). The shared ID showed baseline CPU 0% versus comparison mean 18.9%/peak 25%; the replacement remained a separate row with its first CPU unknown. Eight lifecycle events and one factual nonzero exit were recorded during replacement. These are demonstration observations, not a benchmark.

Browser checks confirmed same-scope choices from All projects, clearing B/results after changing A, fixed run-B resource/log windows, removed-container log explanation, narrow-layout wrapping and reopening both completed runs after demo removal. Demo containers/network were removed; existing workloads and database volume remain intact. Behavior: docs/comparison.md. Proof: outputs/session-comparison.jpg. No capacity benchmark or complete-coverage claim.


## Milestone 4 — first public release preparation — October 30, 2026

Owner approved release preparation, selected MIT and deferred GitHub repository/name creation until later. Retain Observatory as the working name and use Observatory contributors in the copyright notice. No repository or public release is created during local preparation.

Scope: separate prebuilt-image end-user Compose bundle from the existing source-build workflow; preserve the developer stack and volume. Add numbered/digest-pinned packaging, Linux amd64/arm64 builds, image version/revision metadata, MIT/dependency notices, contribution guidance, installation/backup/restore/upgrade/removal documentation and manual publication automation. Default release dashboard remains on localhost and PostgreSQL is internal-only. State Docker socket authority and unsupported/untested hosts accurately.

Local checks: packaging allowlist/reproducibility/image validation tests; native amd64 and emulated arm64 clean release starts, automatic migration, dashboard/readiness/version, data preservation, backup/restore and disposable cleanup. GitHub-native matrix, public anonymous pull, actual release upload and macOS/native host checks remain for when the repository/test machines are available. No universal compatibility or published-distribution claim. Verification in progress.


### Milestone 4 local preparation — implemented and verified October 30, 2026

Prepared the separate prebuilt-image user bundle, deterministic ZIP/checksum packager, digest/version validation, MIT/dependency notices, multi-platform Docker build with version metadata, contribution/security documents, full install/maintenance instructions and manual release/native CI workflows. Existing source-build Compose/project/volume stays available for contributors. Runtime release settings bind localhost, keep the database internal and limit the app filesystem/capabilities without claiming the socket is read-only API access.

Both amd64 and emulated arm64 passed isolated clean installs, migrations/readiness/dashboard/version, backup/restore of seeded data and app-recreation persistence. Test projects/volumes were removed. Three packaging tests, all 25 frontend tests, Go/vet plus the nine real database integration checks passed; actionlint validated workflow structure. Local test bundle: dist/observatory-0.1.0.zip, with a local-only test image reference. No public release exists yet.

Owner explicitly deferred repository creation; later publication work is documented in docs/releases.md. Remaining: final repository/identity, private reporting setup, native hosted CI/OS checks, public image/anonymous install and reviewed GitHub Release. Local release preparation is complete; public publication and unverified host support remain pending by design. See docs/verification.md for evidence and limits.


## Public source repository — October 30, 2026

Owner created https://github.com/JoelJokyu/observatory-docker-monitor and supplied it for the initial upload. Repository name is observatory-docker-monitor; the application remains Observatory. Preparing the first main-branch source commit with MIT notices, release tooling and existing tests. Working data, environment files, dumps, local distribution bundles, downloaded dependencies and screenshots are excluded. Prebuilt images/downloadable releases are still unpublished. GitHub workflow execution and private reporting will be checked separately.


## Version 0.1.0 publication — November 2, 2026

Owner authorized private vulnerability reporting and the first numbered release; topics were already configured by the owner. GitHub confirmed private reporting enabled. Initial native amd64 and arm64 Checks run 41083633644 succeeded. Prepare the exact reviewed source tag v0.1.0, run its release workflow, verify the public multi-platform image and digest-pinned bundle, then publish GitHub release assets. Public source is already on main.

Owner chose to await the Linux-side fix for CVE-2026-98432 rather than change application MPTCP behavior now. Release notes, installation guide and SECURITY.md disclose this kernel issue and avoid an immunity claim. No host/kernel/network settings are changed. macOS host verification remains outside available evidence. Publication verification pending below.


### First numbered release — published and verified November 2, 2026

Private vulnerability reporting is enabled and the reviewed v0.1.0 tag/release is public: https://github.com/JoelJokyu/observatory-docker-monitor/releases/tag/v0.1.0. The numbered workflow built both image architectures, published the image, and produced the digest-pinned installer ZIP/checksum. Full anonymous downloads, ZIP checksum and fresh native Linux amd64/arm64 installations with backup/restore all passed. Added reusable manual Verify published image workflow for future versions. Documentation and evidence are in docs/releases.md and docs/verification.md.

The local Windows Docker Desktop full-image pull still returned denied; its cause is unresolved and disclosed in the public release notes. The owner confirmed package visibility was already public. No misleading universal compatibility or kernel-fix claim is made. CVE-2026-98432 remains a vendor-kernel matter as chosen by the owner, with no application/host mitigation change in this release. Private reporting and the first public version are delivered; further feature work is outside this release.

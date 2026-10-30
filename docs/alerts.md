# Local project alerts

The owner selected opt-in alerts per project. Open the bell, choose a Compose project (or the explicit Standalone bucket), then Configure alerts. All scopes start disabled. There is no all-Engine default and no external delivery or OS-notification permission.

## Rules

Defaults shown before opting in: CPU 80% of one core, memory 80% of Docker's reported limit, nonzero exits, OOM and unhealthy events. Each signal can be switched independently. CPU can exceed 100% for multicore work; memory excludes inactive file cache and the reported limit may be inherited from the Engine. Thresholds are configurable; duration is fixed at 30 seconds.

A resource episode needs distinct, ordered, known high observations spanning at least 30 seconds, with no gap exceeding 1.5 configured sampling intervals. Unknown CPU/limit values reset pending evidence. A long gap restarts pending evidence on the next observation. These are sampled observations, not proof of continuous coverage. The first valid high observation starts the span; with a ten-second interval, at least four observations are needed, and timing jitter can delay firing further.

After firing, repeated high readings update the same episode. Missing readings never mark it recovered; stale evidence is labelled in the inbox. A fresh known reading below the threshold recovers the episode. Another sustained breach can then create a new occurrence. Container IDs remain independent through recreation.

Unhealthy Docker events create one episode until a healthy event. Later start/exit/removal retires that health episode as Container lifecycle changed, not Healthy. A container without Docker health checks cannot produce this signal. Nonzero exits and OOM are factual recorded events; nonzero exits can result from intentional stops. Distinct exits/OOMs are separate occurrences; replay of the same event does not duplicate them.

## Dismissal and configuration

Dismiss acknowledges an occurrence and removes it from the default needs-review inbox. It is retry-safe, persists across process restart and does not resolve a condition. Dismissed high/unhealthy episodes remain suppressed until recovery/lifecycle change and a new breach. Include dismissed reveals the history. Recovered but undismissed alerts still need review; the badge is a review count, not a count of currently unhealthy containers.

Saving settings closes open resource/health episodes as Rule changed and clears condition state. Only observations at/after that save can create new alerts; previous history is not backfilled. Re-saving identical settings also resets that evidence. Existing factual event alerts and dismissal history are preserved. Disabled scopes generate no new alerts. Configured scopes remain selectable even after old container/event history expires.

## Persistence and failure behaviour

Migration 005 adds alert_rules, local_alerts and alert_conditions in the existing PostgreSQL database. Rule/state/evaluation changes serialize under an advisory transaction lock. Resource alert evaluation commits in the sample transaction; event alert evaluation commits in the existing event/member transaction. Source event IDs are unique for factual-event deduplication; health/resource episode state rejects repeated/out-of-order observations. Condition JSON preserves original timestamp precision.

Rules, pending/active conditions and dismissals survive restart. Missing observations across downtime do not count toward a pending duration; active episodes require known recovery rather than being reset by the restart. Persistence failure rolls back the associated observation/alert transaction and degrades the corresponding collector through its existing error path. There is no new broker, durable queue, automatic lossless backfill or reliable-delivery guarantee. Docker's finite replay can leave missing events.

The inbox polls every ten seconds independently of successful inventory polling and reports query failures without inventing a zero count. Resource readiness is not an overall guarantee about event coverage. Alert View resources/logs links reuse the fixed investigation window with source identity and recorded evidence; log availability remains Docker's responsibility. No log content is copied into an alert.

Seven-day retention uses latest high/event evidence. Ongoing resource episodes refresh that timestamp; stale episodes and factual events disappear from queries after seven days without new evidence. Bounded cleanup removes expired entries and old condition rows. Rule settings persist independently. Names, times and evidence are recorded facts, not diagnoses.

## API

- GET /api/alerts: latest 100 matching alerts and full needs_review count, has_more, server time, sample interval and collector state. Optional project=NAME or standalone=true, and include_dismissed=true. The global badge uses the unfiltered count.
- GET /api/alert-rules?project=NAME (or standalone=true): stored rule or disabled defaults; this read does not opt in.
- PUT /api/alert-rules with the same scope: enabled, cpu_enabled, memory_enabled, cpu_threshold, memory_threshold, nonzero_exit, oom, unhealthy. CPU must be finite, above zero and at most 100000%; memory above zero and at most 100%. JSON bodies are capped at 2 KiB; unknown fields are rejected. No all-project rule endpoint.
- POST /api/alerts/{32-hex-id}/dismiss with JSON {}: idempotent acknowledgement; 404 for missing/expired IDs.

The new writes reuse same-origin/JSON guards and reject cross-site browser writes. They are not authentication. Queries/mutations have five-second deadlines and no-store responses. Existing localhost deployment assumptions remain unchanged.

## Isolated demo

Start docker compose -f demo/alerts.compose.yaml up -d. Opt in only observatory-alert-demo. For a quick functional check use CPU 5% and memory 0.01% thresholds in this fixture, whose CPU is limited to 0.25 cores and memory to 32 MiB. Those tiny demo thresholds are not product defaults or recommended application thresholds. The brief profile intentionally exits 7 when run with docker compose -f demo/alerts.compose.yaml run --rm brief. Removing /tmp/healthy from the API fixture produces an unhealthy event; touching it restores health. Disable the demo rule before cleanup, then run docker compose -f demo/alerts.compose.yaml down --volumes. No user application needs to be stopped or reconfigured.

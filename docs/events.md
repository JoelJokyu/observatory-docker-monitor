# Projects and container events

Batch 1 of the approved local testing sessions milestone. No additional runtime service or database is required. Migration 003 adds Compose metadata and a container_events table; earlier samples remain intact.

## User experience

Choose a project above the container list to scope its summary cards, state filters and inventory. The clock icon in the left navigation opens the event timeline using the same project selection. A container's side panel also has an Events tab. Timelines refresh with successful inventory polls (normally ten seconds), and have their own Refresh and relative time-window controls.

Project identity is the exact `com.docker.compose.project` label on the monitored Engine; the service label is displayed as context. Unlabelled containers appear under Standalone. Legacy removed containers cannot have their old labels recovered: blank metadata for those records also appears under Standalone. No project is inferred from a container's name. Project options include retained event metadata, so a short-lived project's events stay selectable even if its containers never appeared in an inventory poll. Container histories still follow IDs, including across recreation.

The timeline shows recorded observations, not inferred causes. A nonzero exit code is highlighted but does not prove a crash; a kill event is labelled "Signal sent", since normal shutdowns can send signals. Docker health events exist only when the workload defines a health check. An OOM event is distinct from merely high memory use.

## Collection and limits

The independent event collector subscribes through the existing Docker SDK. It filters to container events and persists only selected lifecycle actions, standard healthy/unhealthy/starting changes, identity/project/service metadata, Docker timestamps, exit codes and signals. It excludes arbitrary labels, exec commands, environment values, health-check output and log contents. Events have no inventory foreign key, allowing collection before inventory discovery and after removal.

The initial request starts one minute before startup when there are no saved events. Otherwise it overlaps the latest saved timestamp by one second. A hash of normalized metadata including the original nanosecond timestamp deduplicates replay before PostgreSQL rounds timestamps to microseconds. Successful writes advance the in-memory replay cursor; failed writes cancel the stream and retry from the last successful position. Database cursors are read again on process restart. Connections renew after thirty seconds, and failed connections/writes retry after two seconds. Cancellation closes SDK subscriptions through their contexts.

Docker only replays its limited recent buffer (documented as the last 256 events). Outages, application downtime, clock changes, event volume, and slow database writes can leave gaps. There is no durable queue or complete-delivery promise. Every timeline displays the coverage limitation, with separate event connection/interruption status. An empty window means no stored matches, not proof that no events occurred. Event status is separate from resource readiness; `/api/health` continues to describe database/resource collection.

Seven-day cleanup deletes events in bounded batches on the existing minute cleanup cycle. Physical cleanup can lag; queries remain limited to seven days. No application log retention has been introduced.

## API

`GET /api/events?from=RFC3339&to=RFC3339&project=chat&container_id=FULL_ID&limit=200`

- Default window: preceding hour. Maximum: ordered range within the last seven days. Bounds are inclusive from, exclusive to.
- Project/container filters combine. Omit them for all recorded events. `standalone=true` selects blank-project metadata and cannot be combined with a project name.
- Container IDs must be full 64-character hexadecimal IDs. Unknown/removed IDs return matching recorded events or an empty array.
- Default limit 200, allowed 1–500. Queries fetch one extra record to set `has_more`, then return the latest matches in descending time/ID order. Equal timestamps use ID as a stable tie-breaker, not a causal-order guarantee. No pagination in this batch.
- Response: `events`, `from`, `to`, `limit`, `has_more`, event `collector` status and `coverage_notice`. Event fields: `id`, `container_id`, `name`, `project`, `service`, `action`, `at`, optional `exit_code` and `signal`.
- Invalid filters return 400; unavailable storage/configuration returns 503. Queries have a five-second deadline. Responses are `no-store`.
- `/api/containers` additionally returns `project`/`service` per container and a `projects` array from retained inventory/events.

## Repeatable demo

The demo uses cached `alpine:latest`, adds no published ports, and limits memory/CPU:

```powershell
docker compose -f demo/events.compose.yaml up -d
```

Select `observatory-event-demo` and open the timeline. The `brief` service intentionally exits with code 3; the `api` service initially becomes healthy and exits after two minutes. To observe an unhealthy transition while it runs:

```powershell
docker compose -f demo/events.compose.yaml exec api rm /tmp/healthy
```

Remove only this demo afterward:

```powershell
docker compose -f demo/events.compose.yaml down --volumes
```

Its recorded events remain within retention. Docker may no longer serve its logs after removal.

Sources: [Docker events](https://docs.docker.com/reference/cli/docker/system/events/), [Compose canonical labels](https://docs.docker.com/reference/compose-file/services/#labels), and the pinned Docker SDK source in this project.

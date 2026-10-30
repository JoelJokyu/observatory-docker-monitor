# Container log snapshot API

`GET /api/containers/{full-64-character-id}/logs`

Query parameters:

| Parameter | Default | Contract |
| --- | --- | --- |
| from | One hour ago | RFC3339 timestamp; lower bound passed to Docker. |
| to | Now | RFC3339 timestamp; upper bound passed to Docker. |
| limit | 500 | Integer 1–1000; requested Docker tail count. |

The range must be ordered and within the last seven days, matching the resource-investigation UI. It does not guarantee retained log coverage. The endpoint reads Docker independently of PostgreSQL and does not implement follow/streaming.

A successful response identifies the container, exact range, fetch time, source `docker`, logging-driver name, TTY mode, requested tail limit, returned text size, and entries. Each entry has a nullable Docker timestamp, stdout/stderr/combined stream, plain-text message, and a partial flag. Empty output is `entries: []`.

`limit_reached` means the tail count was reached and earlier lines may exist. `truncated` and `truncation_reason` identify an actual text/wire/decoder line budget. Truncation can omit newer content from the selected tail; narrow the range when investigating oversized output. UTF-8-invalid bytes are replaced for safe JSON text transport.

Responses use `Cache-Control: no-store`. Logs are not archived or backfilled.

Failure codes:

| HTTP | Code | Meaning |
| --- | --- | --- |
| 400 | invalid_query | Invalid ID, range, or line limit; Docker is not queried. |
| 404 | container_not_found | Docker no longer has that container. |
| 422 | logs_unsupported | The logging driver is `none` or Docker reports unreadable output. |
| 502 | invalid_docker_response | Malformed, incomplete, or system-error log stream. |
| 503 | docker_unavailable | Docker could not serve the snapshot. |
| 503 | logs_busy | Four snapshots are already in progress; Retry-After is two seconds. |
| 503 | logs_not_configured | The app has no log reader configured. |
| 504 | logs_timeout | The snapshot exceeded its deadline. |

Cancellation closes Docker's response reader. Stream labels do not establish severity. JSON log content is intentionally not interpreted as metadata in this milestone.

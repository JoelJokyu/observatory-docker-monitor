# Named local test sessions

Batch 2 of the approved local testing milestone. The owner selected automatic membership for new/replacement containers in the chosen Compose project and expiry seven days after finishing, with unavailable older telemetry marked explicitly.

## Workflow

Open the flask icon (**Test sessions**), choose a Compose project or **Standalone containers**, and click **Start session**. Name the test, run your application test yourself, then click **Finish session**. Sessions do not generate traffic or operate your application containers. All projects is a browsing scope; starting requires choosing one project or the Standalone scope.

One session may be active per scope. The database enforces this even across tabs or concurrent requests. A session persists across Observatory restarts and stays active until explicitly finished; wall-clock duration includes periods when Observatory or Docker was unavailable. Stopped containers that still exist can be present members, but resource observations require a running container.

The start boundary and finish boundary come from PostgreSQL's clock. Finish retries preserve the original finish time. If a request times out, refresh the list/summary before retrying, since the database may have committed the change even if the response was lost. New sessions require recent inventory and at least one present container in the scope. Existing sessions can be finished without healthy Docker collection if the database is reachable.

## Membership and boundaries

Migration 004 adds test_sessions and session_members in the existing PostgreSQL database. Members have independent container IDs plus name/service snapshots. There is no foreign key to inventory, so removal of an inventory identity does not erase session membership.

Starting captures currently present containers in the selected scope. Later successful inventory observations and matching in-window Docker events add members, including containers created/replaced during a session and short-lived containers missed by inventory. Late event replay after finish can still add members/evidence when its event timestamp falls inside the session. Members are never merged by name. Successful in-window inventory refreshes their displayed name/service, so a temporary Compose replacement name can resolve to the final observed name; initial identity and join boundary remain intact.

An advisory transaction lock serializes inventory/event membership writes with session start/finish; the active-scope partial unique index prevents overlapping active sessions. Event metadata and its membership insert commit together, so replay can safely retry both. A start snapshot includes present targets; later observations use inclusive start/exclusive finish bounds. Summary resource/event queries use the same half-open session interval, clipped to available seven-day history. Docker clock skew, unavailable collection and limited event replay still prevent a complete-membership or complete-coverage guarantee.

Standalone means all observed blank-project containers, not a single application inferred from names. Legacy removed containers whose labels are unknown cannot be retrospectively assigned to a Compose project.

## Summary definitions

- Elapsed wall time: start to finish, or start to the summary's check time while active; includes downtime.
- Containers observed: retained session member IDs, including members with no resource readings.
- Resource observations: count of raw recorded samples in the available session interval.
- Events: selected recorded lifecycle events for members in the same interval; nonzero exits count Docker die events with a known exit code other than 0. They do not automatically prove a crash.
- Per-container CPU mean: arithmetic average of known CPU samples, without time weighting. Unknown values are excluded; their count is shown through the known CPU observation count.
- Per-container peak CPU/memory: highest raw recorded reading. A brief spike between collection times can be missed. Container maxima are not summed into an invented project-wide peak.
- First/last sample: observed resource bounds, not proof that all intermediate readings exist.
- Missing readings: null and displayed as a dash; explicit zero remains zero.

Summaries are queries over retained telemetry, not immutable archives. Late in-window replay can update them, and older telemetry expires normally. The response provides one consistent database snapshot, an as_of clock value, and the available_from/to bounds. When a session began more than seven days ago, history_expired is true and the UI explains that older measurements/events are unavailable. Session metadata/membership remain available for seven days after finish; therefore they can outlive some underlying telemetry. Active sessions are not silently finished/expired, but their older telemetry still ages out. Bounded cleanup removes expired session metadata and cascades its members.

Logs remain on-demand Docker snapshots, with no independent archive. Session-to-log navigation is implemented in batch 3. Local alerts are batch 4; completed-session comparison is batch 5, described in [comparison.md](comparison.md).

## API

- `GET /api/sessions?project=NAME` or `?standalone=true`: current active/completed sessions, active first and then newest start, maximum 100 with has_more. Omit the scope to browse all. Expired completed sessions are omitted before physical cleanup.
- `POST /api/sessions`: JSON `{ "name": "baseline", "project": "chat", "standalone": false }`. For Standalone use an empty project and standalone true. Names must contain 1–80 characters after trimming, with no control characters. Returns 201 and session metadata.
- `GET /api/sessions/{id}`: summary, resource collector status and coverage notice; summary member detail is limited to the first 500, with containers_limited, while totals count all members.
- `POST /api/sessions/{id}/finish`: JSON `{}`; returns the original finish boundary on retries.

IDs are opaque 32-character hex strings. Bodies are capped at 2 KiB; unknown properties and trailing JSON are rejected. New write routes require application/json, reject cross-site fetches and require a matching scheme/host when Origin is present. This guards local browser writes; it is not authentication. Existing localhost deployment assumptions remain unchanged. All responses are no-store. Invalid input is 400, unsupported media 415, cross-site write 403, active-scope/freshness conflict 409, missing/expired session 404, and unavailable storage 503. List/mutations use five-second deadlines, summaries eight seconds.

## Isolated demonstration

```powershell
docker compose -f demo/sessions.compose.yaml up -d
```

Select observatory-session-demo and start a session. Its API fixture is resource-limited and sleeps for ten minutes. Recreate it during the session to observe two separate IDs:

```powershell
docker compose -f demo/sessions.compose.yaml up -d --force-recreate --no-deps api
docker compose -f demo/sessions.compose.yaml run --rm brief
```

The brief worker intentionally exits with code 7; that expected command failure produces useful session evidence. Finish the session, then remove only the demo:

```powershell
docker compose -f demo/sessions.compose.yaml down --volumes
```

# Logs investigation demo

From the Observatory project root, start the app and the isolated demo:

```powershell
docker compose up --build -d
docker compose -f demo/compose.yaml up -d --pull never
```

The demo reuses the PostgreSQL image already used by Observatory. It runs shell commands only, not another database. It publishes no ports and does not touch Observatory's data volume or the chat application.

The `workload` container prints identifiable stdout/stderr and multiline messages, runs 25 seconds of CPU load limited to half a core and 64 MiB memory, prints completion events, then exits. Select it while running to inspect resource history and the Logs tab in the same time window. Refresh logs explicitly after a new event. Docker timestamp labels use local display time.

The script-like test line is deliberately untrusted plain text and must not execute in the dashboard. stderr is a stream, not an inferred error severity. The separate `tty` container demonstrates combined output; `disabled` demonstrates a logging driver that cannot provide readable output.

Containers remain after exit so stopped-container logs can be inspected. Repeat the workload when needed:

```powershell
docker compose -f demo/compose.yaml up -d --force-recreate workload
```

This creates a new Docker ID and a separate resource history. The old container's logs are no longer available from Docker. Cleanup targets only the isolated `observatory-log-demo` project:

```powershell
docker compose -f demo/compose.yaml down --volumes
```

The app/database stay running. Named Observatory data volumes are not part of the demo project. Historical demo resource samples can remain until ordinary retention cleanup; no independent demo log archive is created.

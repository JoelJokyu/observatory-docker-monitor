# Investigation tools

Batch 3 adds frontend navigation over the existing history and Docker-log APIs. No database migration, new service, log archive or new API is introduced.

Events in the workspace and container timelines offer View resources and View logs. They open the same full container ID in the detail panel, with context up to two minutes before/after the event, clipped to the present and seven-day resource retention. Unavailable/invalid event times cannot open a context. This is a labelled convenience window, not a causal assertion.

Each session member offers the same actions using that summary's exact available_from/to bounds. Active-session navigation captures the window visible at the time of the action; it does not grow with the ongoing session. Retained portions of older sessions are labelled. Each replacement ID is investigated independently.

An investigation pins the absolute range across resource and log tabs, resource polling, and explicit log Refresh. Relative selectors are disabled and labelled Fixed investigation window. Current-reading cards are hidden while pinned so they are not confused with historical readings. Return to recent history removes the pin and restores normal relative controls. The context states exact local times. Existing bucket means and chart coverage limitations still apply. Other container tabs retain their existing semantics; the Events tab has its own relative timeline controls.

If inventory no longer contains a member, its recorded ID/name/project/service can still open history. Such identities are labelled Recorded, with current presence unknown, rather than claiming current measurements. Log access for these identities asks Docker directly and displays its actual unavailable/not-found response. Known removed inventory records continue to show the existing removed-container notice. No invented inventory-seen time is displayed for a recorded-only identity.

Log tools operate on the bounded snapshot already loaded:

- Literal, case-insensitive search over message text, including multiline messages; no regular expressions or severity inference.
- Stream filters: all, stdout, stderr and combined TTY. stderr is not an error classification.
- Search and stream filters combine without a new Docker request.
- Copy matching lines copies only the filtered entries, with original timestamp/stream provenance and a marker for partial lines. Multiline text and characters are preserved as plain text. Copy failure offers manual copying; nothing is sent to an external service.
- No-match results are distinct from an empty Docker snapshot and explicitly describe the loaded-snapshot scope. TTY, truncation/tail limits and missing coverage notices remain visible.
- Explicit Refresh still uses existing time/line/byte/deadline bounds. Ordinary inventory polling does not refetch logs.

The local demo in demo/investigation.compose.yaml emits stdout, stderr, multiline/script-like plain text and TTY output with resource limits and no published ports. Start with `docker compose -f demo/investigation.compose.yaml up -d`; remove only it with `docker compose -f demo/investigation.compose.yaml down --volumes` after review. Removed demo logs are no longer available from Docker.

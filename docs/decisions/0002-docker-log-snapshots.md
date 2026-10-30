# ADR 0002 — Docker-backed log snapshots

Status: approved by the owner, October 24, 2026. Implementation and verification are tracked separately.

## Context

Observatory is a local-first tool for developers actively testing Docker applications. The owner does not require an independent log archive and approved a bounded snapshot with an explicit Refresh button.

## Decision

Read stdout/stderr from Docker on demand through the existing Go monolith. Retain no log copy in PostgreSQL. Support stopped containers when Docker can still serve their output. Reuse the selected resource-history window; each snapshot records and displays its exact absolute bounds and fetch time. Changing the selected duration or reopening the tab fetches a snapshot. Resource polling alone does not refetch logs.

Live following, independent retention, ingestion checkpoints, and persistent log search are deferred.

## Alternatives

Ingesting and storing logs would preserve already collected output beyond Docker's retention or container deletion. It would also require a delivery lifecycle, storage and query model, retention, resource limits, duplicate handling, and recovery decisions. Those costs do not currently serve the approved active-testing use case.

## Implementation bounds

Default Docker tail: 500 lines; API maximum: 1,000. Raw decoded text budget: 512 KiB. Returned message text is also capped at 512 KiB after UTF-8 normalization. Wire budget: 1 MiB including framing. Backend timeout: eight seconds, with client cancellation propagated to Docker. At most four concurrent snapshots; excess requests receive a retryable response rather than an unbounded queue.

The decoder reads Docker's documented eight-byte stream header and processes payload in fixed chunks. It does not allocate from a potentially very large frame length. This bounded reader is deliberate: the SDK StdCopy implementation grows its buffer to fit a declared frame. Tests check compatibility with the SDK decoder as well as split, malformed, oversized, and TTY streams.

## Consequences

- Docker's driver, cache, rotation, and container lifetime govern log availability. A seven-day resource window does not imply seven days of logs.
- The selected time interval is passed to Docker without reinterpreting application timestamps. Docker-provided timestamps are displayed where present. Continuation lines may have no timestamp.
- Non-TTY streams retain stdout/stderr labels. TTY output is combined; stderr does not imply error severity.
- Multiline content is displayed as text lines without JSON/severity inference. Each stream buffers incomplete lines until newline/EOF. Across interleaved streams, entries follow line-completion order; no exact cross-stream causal ordering is claimed.
- A snapshot reaching the requested tail count is marked as possibly tail-limited, not certainly truncated. Byte, wire, and decoder line limits explicitly mark truncation. A byte-limited response can contain only a prefix of the requested tail, so the UI does not promise it contains the newest output.
- Removed containers retain their resource history but have no Observatory log archive. An empty result, removed container, unsupported driver, timeout, busy capacity, and invalid upstream response have distinct outcomes.
- Log content is rendered as untrusted text. Raw upstream errors and log bodies are not copied into platform diagnostic logs.
- No schema change, broker, separately deployed component, or automatic Docker logging reconfiguration is introduced.

# Comparing local test sessions

Open **Test sessions → Compare sessions**. Choose completed run A as a baseline, then completed run B from the same Compose project or the same explicit Standalone scope. Active sessions are excluded. When browsing All projects, choosing A restricts B to that scope. Dropdowns show names and start times; the existing list returns at most 100 sessions, so choose a project to narrow it.

The comparison shows:

- Wall-clock elapsed duration (including downtime), configured sampling interval, container IDs, raw sample count and recorded event/nonzero-exit totals for each run.
- The retained time range and the independent summary check time for each run. History loss is disclosed; being within retention does not establish complete coverage.
- Per full container ID: CPU arithmetic sample mean, observed CPU maximum, observed memory maximum, total samples/known CPU samples and first/last reading times. CPU is percent of one core, and memory uses the existing cache-excluding measurement definition.
- Selected lifecycle event counts side by side. Zero means no matching recorded events, not proof that none occurred. Nonzero exits can be intentional.

A and B use each full session’s available half-open time range. There is no alignment by elapsed offset, equal-duration cropping, interpolation, time weighting, rate normalization, summed project peak, automatic service aggregation, percent delta or performance verdict. A slower sampling interval, collection gaps, unequal duration, different requests/container limits or changed replicas can affect the values. Match workloads yourself before drawing conclusions.

Rows match **only full container ID**. A recreated container with the same name is a separate row. A container absent from a complete member list says “Not a member”; a member without known values displays a dash. Measured zero remains zero. If a list hits its 500-member limit, an absent ID is explicitly inconclusive; aggregate summary counts still include all members.

**View resources** and **View logs** use that cell’s session name, full identity and available time range, preserving the existing fixed investigation window. Docker logs remain on-demand and may be unavailable after removal or rotation.

## Reads and refresh

This feature uses two existing `GET /api/sessions/{id}` calls with a shared cancellation signal and a twelve-second client deadline. Each summary has its own internally consistent database snapshot and `as_of`; the pair is not one atomic cross-session snapshot. No new API, migration, service or stored report is added.

Choosing either run clears the old comparison and cancels pending reads. Both reads must succeed before results appear. Missing, expired, active, identical or cross-scope pairs cannot produce a comparison. A failed list refresh hides results until the list is available again. A selected session disappearing from the successfully refreshed list clears that selection. **Refresh comparison** re-reads both summaries; ordinary inventory polling does not update a loaded pair. Completed sessions can still change through late replay and normal history expiry; a loaded pair is a labeled view at its shown check times, not an immutable archive.

## Isolated demo

`docker compose -f demo/comparison.compose.yaml up -d` runs a bounded Alpine fixture in the separate observatory-comparison-demo project, limited to 0.25 cores and 32 MiB. Record and finish one session, then repeat with your own changed test. Recreate only this fixture during a run to check identity handling. `docker compose -f demo/comparison.compose.yaml down --volumes` removes the isolated demo; recorded sessions remain within normal retention.

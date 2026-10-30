import { useEffect, useState } from 'react';
import { ArrowLeftRight, RefreshCw } from 'lucide-react';
import { bytes, fetchJSON, numeric } from './format';
import {
  duration,
  type Detail,
  type SessionMember,
  type SessionSummary,
  type TestSession,
} from './SessionsPanel';
import { eventLabel } from './EventsPanel';
import { recordedContainer, type OpenInvestigation } from './investigation';

export function sameScope(a: TestSession, b: TestSession) {
  return a.project === b.project && a.standalone === b.standalone;
}
export function comparisonRows(a: SessionSummary, b: SessionSummary) {
  const left = new Map(a.containers.map((c) => [c.container_id, c]));
  const right = new Map(b.containers.map((c) => [c.container_id, c]));
  return [...new Set([...left.keys(), ...right.keys()])]
    .sort()
    .map((id) => ({ id, a: left.get(id), b: right.get(id) }));
}
function Reading({
  member,
  summary,
  onInvestigate,
}: {
  member?: SessionMember;
  summary: SessionSummary;
  onInvestigate?: OpenInvestigation;
}) {
  if (!member)
    return (
      <span className="session-member-meta">
        {summary.containers_limited
          ? 'Absent from returned members; list is limited'
          : 'Not a member of this session'}
      </span>
    );
  const c = member;
  return (
    <>
      <strong>{c.name || c.container_id.slice(0, 12)}</strong>
      <span className="session-member-meta">{c.service || 'No service label'}</span>
      <dl className="comparison-readings">
        <div>
          <dt>Mean CPU</dt>
          <dd>{c.cpu_mean == null ? '—' : numeric(c.cpu_mean) + '%'}</dd>
        </div>
        <div>
          <dt>Peak CPU</dt>
          <dd>{c.cpu_peak == null ? '—' : numeric(c.cpu_peak) + '%'}</dd>
        </div>
        <div>
          <dt>Peak memory</dt>
          <dd>{bytes(c.memory_peak_bytes)}</dd>
        </div>
        <div>
          <dt>Samples / known CPU</dt>
          <dd>
            {c.samples} / {c.cpu_observations}
          </dd>
        </div>
      </dl>
      <span className="session-member-meta">
        {c.first_sample && c.last_sample
          ? 'Observed: ' +
            new Date(c.first_sample).toLocaleString() +
            ' – ' +
            new Date(c.last_sample).toLocaleString()
          : 'No resource observations in this window'}
      </span>
      {onInvestigate && (
        <div className="investigation-actions">
          {(['resources', 'logs'] as const).map((tab) => (
            <button
              key={tab}
              className="refresh-button"
              disabled={Date.parse(summary.available_from) >= Date.parse(summary.to)}
              onClick={(e) =>
                onInvestigate(
                  {
                    container: recordedContainer(
                      c.container_id,
                      c.name,
                      summary.session.project,
                      c.service,
                    ),
                    range: { from: summary.available_from, to: summary.to },
                    tab,
                    label:
                      'Session · ' +
                      summary.session.name +
                      (summary.history_expired ? ' · retained portion only' : ''),
                  },
                  e.currentTarget,
                )
              }
            >
              {tab === 'logs' ? 'View logs' : 'View resources'}
            </button>
          ))}
        </div>
      )}
    </>
  );
}
export function ComparisonResults({
  left,
  right,
  onInvestigate,
}: {
  left: Detail;
  right: Detail;
  onInvestigate?: OpenInvestigation;
}) {
  const a = left.summary,
    b = right.summary;
  if (
    a.session.id === b.session.id ||
    !a.session.finished_at ||
    !b.session.finished_at ||
    !sameScope(a.session, b.session)
  )
    return (
      <p role="alert" className="detail-notice">
        Choose two different completed sessions in the same project or Standalone scope.
      </p>
    );
  const actions = [
    ...new Set([...a.actions.map((v) => v.action), ...b.actions.map((v) => v.action)]),
  ].sort();
  return (
    <div className="comparison-results">
      <div className="comparison-intro">
        <ArrowLeftRight size={20} />
        <div>
          <h3>Two runs, recorded evidence</h3>
          <p>
            {a.session.standalone ? 'Standalone containers' : a.session.project} · A is the
            baseline; B is the comparison.
          </p>
        </div>
      </div>
      <p className="detail-notice">
        Match your test workload and container limits before interpreting differences. Elapsed time
        includes downtime. Unequal duration, sampling gaps and changed replicas can affect these
        values; this is not a performance score.
      </p>
      <div className="comparison-runs">
        {[left, right].map((d, i) => {
          const v = d.summary;
          return (
            <article
              className="session-evidence"
              key={v.session.id}
              aria-label={'Run ' + (i ? 'B' : 'A')}
            >
              <span className="section-label">RUN {i ? 'B' : 'A'}</span>
              <h3>{v.session.name}</h3>
              <p>
                {new Date(v.session.started_at).toLocaleString()} –{' '}
                {new Date(v.to).toLocaleString()}
              </p>
              <dl className="comparison-readings">
                <div>
                  <dt>Elapsed duration</dt>
                  <dd>{duration(v.session.started_at, v.to)}</dd>
                </div>
                <div>
                  <dt>Configured sample interval</dt>
                  <dd>{v.session.sample_interval_seconds}s</dd>
                </div>
                <div>
                  <dt>Member IDs / samples</dt>
                  <dd>
                    {v.container_count} / {v.samples}
                  </dd>
                </div>
                <div>
                  <dt>Recorded events / nonzero exits</dt>
                  <dd>
                    {v.event_count} / {v.nonzero_exits}
                  </dd>
                </div>
              </dl>
              <p className="session-method">
                {v.history_expired
                  ? 'Older measurements and events are unavailable. Retained portion only.'
                  : 'Full time range is within retention; continuous coverage is not guaranteed.'}
              </p>
              <p className="session-method">
                Available: {new Date(v.available_from).toLocaleString()} –{' '}
                {new Date(v.to).toLocaleString()}
                <br />
                Summary checked: {new Date(v.as_of).toLocaleString()}
              </p>
              <p className="session-method">{d.coverage_notice}</p>
            </article>
          );
        })}
      </div>
      {(a.containers_limited || b.containers_limited) && (
        <p className="detail-notice">
          Member lists are limited to 500 per run. Totals include all members; absence from a
          limited list does not prove absence from the session.
        </p>
      )}
      <div className="session-evidence">
        <div className="session-section-heading">
          <h3>Resources by container identity</h3>
          <span>CPU · % of one core</span>
        </div>
        <p className="session-method">
          CPU means average known samples without time weighting. Peaks are observed maxima, not
          simultaneous project totals. A dash means unknown; zero is a measured zero. First/last
          observations do not prove continuous coverage. Replacement IDs remain separate, even when
          names match.
        </p>
        <div className="table-scroll">
          <table className="comparison-table comparison-resource-table">
            <thead>
              <tr>
                <th>Container ID</th>
                <th>A · {a.session.name}</th>
                <th>B · {b.session.name}</th>
              </tr>
            </thead>
            <tbody>
              {comparisonRows(a, b).map((row) => (
                <tr key={row.id}>
                  <th scope="row">
                    <code title={row.id}>{row.id.slice(0, 12)}</code>
                  </th>
                  <td>
                    <Reading member={row.a} summary={a} onInvestigate={onInvestigate} />
                  </td>
                  <td>
                    <Reading member={row.b} summary={b} onInvestigate={onInvestigate} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {!a.containers.length && !b.containers.length && (
          <p className="event-empty">No retained members in either run.</p>
        )}
      </div>
      <div className="session-evidence">
        <h3>Recorded lifecycle events</h3>
        <p className="session-method">
          Raw counts over each run’s available window, without duration normalization. Nonzero exits
          are factual exit codes and can be intentional. Missing events are possible with Docker
          replay gaps.
        </p>
        <div className="table-scroll">
          <table className="comparison-table">
            <thead>
              <tr>
                <th>Event</th>
                <th>A</th>
                <th>B</th>
              </tr>
            </thead>
            <tbody>
              {actions.map((action) => (
                <tr key={action}>
                  <th scope="row">{eventLabel(action)}</th>
                  <td>{a.actions.find((v) => v.action === action)?.count ?? 0}</td>
                  <td>{b.actions.find((v) => v.action === action)?.count ?? 0}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {!actions.length && (
          <p className="session-method">No recorded lifecycle events in either available window.</p>
        )}
      </div>
    </div>
  );
}
export default function SessionComparison({
  sessions,
  listError,
  hasMore,
  onInvestigate,
}: {
  sessions: TestSession[];
  listError: string | null;
  hasMore: boolean;
  onInvestigate?: OpenInvestigation;
}) {
  const [leftID, setLeftID] = useState(''),
    [rightID, setRightID] = useState(''),
    [pair, setPair] = useState<[Detail, Detail] | null>(null),
    [error, setError] = useState<string | null>(null),
    [busy, setBusy] = useState(false),
    [request, setRequest] = useState(0);
  const completed = sessions.filter((s) => s.finished_at),
    left = completed.find((s) => s.id === leftID);
  useEffect(() => {
    setPair(null);
    setError(null);
    setBusy(false);
    if (!leftID || !rightID) return;
    const controller = new AbortController();
    let active = true;
    const timeout = setTimeout(() => controller.abort(), 12000);
    setBusy(true);
    Promise.all([
      fetchJSON<Detail>('/api/sessions/' + leftID, controller.signal),
      fetchJSON<Detail>('/api/sessions/' + rightID, controller.signal),
    ])
      .then((value) => {
        if (active) setPair(value);
      })
      .catch((reason) => {
        if (active)
          setError(
            reason instanceof Error && reason.name !== 'AbortError'
              ? reason.message
              : 'Comparison timed out. Refresh to retry.',
          );
      })
      .finally(() => {
        clearTimeout(timeout);
        if (active) setBusy(false);
      });
    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [leftID, rightID, request]);
  useEffect(() => {
    if (listError) return;
    if (leftID && !sessions.some((s) => s.id === leftID && s.finished_at)) {
      setLeftID('');
      setRightID('');
      setPair(null);
    } else if (rightID && !sessions.some((s) => s.id === rightID && s.finished_at)) {
      setRightID('');
      setPair(null);
    }
  }, [sessions, listError, leftID, rightID]);
  // Summary reads are refreshed explicitly: inventory polling must not move a historical comparison.
  return (
    <div className="session-comparison">
      <div className="comparison-selectors">
        <label>
          Run A · baseline
          <select
            value={leftID}
            onChange={(e) => {
              setLeftID(e.target.value);
              setRightID('');
              setPair(null);
              setBusy(false);
            }}
          >
            <option value="">Choose a completed session</option>
            {completed.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} · {s.standalone ? 'Standalone' : s.project} ·{' '}
                {new Date(s.started_at).toLocaleString()}
              </option>
            ))}
          </select>
        </label>
        <label>
          Run B · compare
          <select
            value={rightID}
            disabled={!left}
            onChange={(e) => {
              setRightID(e.target.value);
              setPair(null);
            }}
          >
            <option value="">Choose another run in this scope</option>
            {completed
              .filter((s) => left && s.id !== left.id && sameScope(left, s))
              .map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} · {new Date(s.started_at).toLocaleString()}
                </option>
              ))}
          </select>
        </label>
        <button
          className="refresh-button"
          disabled={!leftID || !rightID || busy}
          onClick={() => setRequest((v) => v + 1)}
        >
          <RefreshCw size={14} /> Refresh comparison
        </button>
      </div>
      {hasMore && (
        <p className="detail-notice">
          Choose a project above to narrow the latest 100 sessions. Older sessions may not be in
          this list.
        </p>
      )}
      {completed.length < 2 && (
        <p className="session-scope-note">
          Finish two sessions in the same project to compare their recorded evidence.
        </p>
      )}
      {left && completed.filter((s) => s.id !== left.id && sameScope(left, s)).length === 0 && (
        <p className="session-scope-note">
          No other completed session in this scope. Finish another run first.
        </p>
      )}
      {busy && <p role="status">Loading both session summaries…</p>}
      {error && (
        <p role="alert" className="detail-notice">
          {error} No comparison is displayed. A session may have expired; refresh the session list.
        </p>
      )}
      {pair && !listError && (
        <ComparisonResults left={pair[0]} right={pair[1]} onInvestigate={onInvestigate} />
      )}
      {pair && listError && (
        <p className="detail-notice">
          Refresh the session list successfully to display the comparison.
        </p>
      )}
    </div>
  );
}

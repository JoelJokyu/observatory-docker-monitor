import { useEffect, useRef, useState } from 'react';
import {
  Activity,
  ArrowLeft,
  Check,
  Clock3,
  FlaskConical,
  Play,
  RefreshCw,
  Square,
  X,
} from 'lucide-react';
import { bytes, fetchJSON, numeric } from './format';
import SessionComparison from './SessionComparison';
import { eventLabel } from './EventsPanel';
import { recordedContainer, type OpenInvestigation } from './investigation';

export interface TestSession {
  id: string;
  name: string;
  project: string;
  standalone: boolean;
  started_at: string;
  finished_at: string | null;
  sample_interval_seconds: number;
}
export interface SessionMember {
  container_id: string;
  name: string;
  service: string;
  first_observed_at: string;
  samples: number;
  first_sample: string | null;
  last_sample: string | null;
  cpu_observations: number;
  cpu_mean: number | null;
  cpu_peak: number | null;
  memory_peak_bytes: number | null;
}
export interface SessionSummary {
  session: TestSession;
  as_of: string;
  available_from: string;
  to: string;
  history_expired: boolean;
  container_count: number;
  samples: number;
  event_count: number;
  nonzero_exits: number;
  containers: SessionMember[];
  containers_limited: boolean;
  actions: { action: string; count: number }[];
}
export interface Detail {
  summary: SessionSummary;
  coverage_notice: string;
  collector: { healthy: boolean; message: string; last_inventory: string | null };
}
export function duration(from: string, to: string): string {
  const seconds = Math.max(0, Math.floor((Date.parse(to) - Date.parse(from)) / 1000));
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  if (seconds < 86400)
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
  return `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`;
}
const pct = (n: number | null) => (n == null ? '—' : numeric(n) + '%');
export function SessionResults({
  summary,
  notice,
  onInvestigate,
}: {
  summary: SessionSummary;
  notice: string;
  onInvestigate?: OpenInvestigation;
}) {
  const v = summary,
    s = v.session;
  return (
    <>
      <div className="session-metrics">
        <div>
          <span>Elapsed time</span>
          <strong>{duration(s.started_at, v.to)}</strong>
          <small>Includes collection downtime</small>
        </div>
        <div>
          <span>Containers observed</span>
          <strong>{v.container_count}</strong>
          <small>Replacement IDs stay separate</small>
        </div>
        <div>
          <span>Resource observations</span>
          <strong>{v.samples}</strong>
          <small>Across available history</small>
        </div>
        <div>
          <span>Recorded events</span>
          <strong>{v.event_count}</strong>
          <small>
            {v.nonzero_exits} nonzero {v.nonzero_exits === 1 ? 'exit' : 'exits'}
          </small>
        </div>
      </div>
      {v.history_expired && (
        <div className="detail-notice session-retention" role="status">
          <Clock3 size={16} />
          <span>
            Part of this session is older than seven days. Those measurements and events are
            unavailable; the summary covers only the retained time range below.
          </span>
        </div>
      )}
      <div className="session-evidence">
        <div className="session-section-heading">
          <h3>Resource summary</h3>
          <span>Per container · recorded observations</span>
        </div>
        <div className="table-scroll">
          <table className="session-resource-table">
            <thead>
              <tr>
                <th>Container</th>
                <th>Samples</th>
                <th>Mean CPU</th>
                <th>Peak CPU</th>
                <th>Peak memory</th>
              </tr>
            </thead>
            <tbody>
              {v.containers.map((c) => (
                <tr key={c.container_id}>
                  <td>
                    <strong>{c.name || c.container_id.slice(0, 12)}</strong>
                    {onInvestigate && (
                      <div className="investigation-actions">
                        {(['resources', 'logs'] as const).map((tab) => (
                          <button
                            key={tab}
                            className="refresh-button"
                            disabled={Date.parse(v.available_from) >= Date.parse(v.to)}
                            onClick={(click) =>
                              onInvestigate(
                                {
                                  container: recordedContainer(
                                    c.container_id,
                                    c.name,
                                    s.project,
                                    c.service,
                                  ),
                                  range: { from: v.available_from, to: v.to },
                                  tab,
                                  label:
                                    'Session · ' +
                                    s.name +
                                    (v.history_expired ? ' · retained portion only' : ''),
                                },
                                click.currentTarget,
                              )
                            }
                          >
                            {tab === 'logs' ? 'View logs' : 'View resources'}
                          </button>
                        ))}
                      </div>
                    )}
                    <span className="session-member-meta">
                      {c.service ? c.service + ' · ' : ''}
                      {c.container_id.slice(0, 12)}
                    </span>
                    <span className="session-member-meta">
                      {c.first_sample && c.last_sample
                        ? new Date(c.first_sample).toLocaleString() +
                          ' – ' +
                          new Date(c.last_sample).toLocaleTimeString()
                        : 'No resource observations in this window'}
                    </span>
                  </td>
                  <td className="mono">{c.samples}</td>
                  <td className="mono">
                    {pct(c.cpu_mean)}
                    <span className="session-member-meta">{c.cpu_observations} known</span>
                  </td>
                  <td className="mono">{pct(c.cpu_peak)}</td>
                  <td className="mono">{bytes(c.memory_peak_bytes)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {!v.containers.length && (
          <p className="event-empty">No matching containers have been observed yet.</p>
        )}
        {v.containers_limited && (
          <p className="detail-notice">
            Showing the first 500 containers. Summary counts include all observed members.
          </p>
        )}
        <p className="session-method">
          CPU means average known samples, without time weighting. Peaks are the highest recorded
          readings for each container; brief spikes between samples may be missed. A dash means no
          known value.
        </p>
      </div>
      <div className="session-evidence">
        <div className="session-section-heading">
          <h3>Lifecycle summary</h3>
          <span>Observed events within the session</span>
        </div>
        <div className="session-action-grid">
          {v.actions.map((a) => (
            <div key={a.action}>
              <span>{eventLabel(a.action)}</span>
              <strong>{a.count}</strong>
            </div>
          ))}
        </div>
        {!v.actions.length && (
          <p className="session-method">
            No recorded lifecycle events in the available window. This does not prove that nothing
            happened.
          </p>
        )}
      </div>
      <div className="session-coverage">
        <Activity size={15} />
        <div>
          <p>{notice}</p>
          <span>
            Available window: {new Date(v.available_from).toLocaleString()} –{' '}
            {new Date(v.to).toLocaleString()}
          </span>
          <span>Summary checked: {new Date(v.as_of).toLocaleString()}</span>
        </div>
      </div>
    </>
  );
}
function StartSessionModal({
  project,
  onClose,
  onStarted,
}: {
  project: string;
  onClose: () => void;
  onStarted: (s: TestSession) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [name, setName] = useState(''),
    [busy, setBusy] = useState(false),
    [error, setError] = useState<string | null>(null);
  const pending = useRef(false);
  useEffect(() => {
    const el = dialog.current,
      trigger = document.activeElement;
    el?.showModal();
    return () => {
      el?.close();
      if (trigger instanceof HTMLElement && trigger.isConnected) trigger.focus();
    };
  }, []);
  const scope = project === 'standalone' ? 'Standalone containers' : project.slice(8);
  return (
    <dialog
      ref={dialog}
      className="workspace-modal session-modal"
      aria-labelledby="session-start-title"
      onCancel={(e) => {
        e.preventDefault();
        if (!pending.current) onClose();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget && !pending.current) onClose();
      }}
    >
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          if (pending.current) return;
          pending.current = true;
          setBusy(true);
          setError(null);
          const controller = new AbortController();
          const timeout = setTimeout(() => controller.abort(), 8000);
          try {
            const response = await fetch('/api/sessions', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                name,
                project: project.startsWith('project:') ? project.slice(8) : '',
                standalone: project === 'standalone',
              }),
              signal: controller.signal,
            });
            const body = await response.json();
            if (!response.ok) throw new Error(body.error || 'Unable to start session');
            onStarted(body as TestSession);
          } catch (reason) {
            setError(
              reason instanceof Error && reason.name !== 'AbortError'
                ? reason.message
                : 'The request timed out. Close this form and refresh the session list before retrying; the session may have started.',
            );
          } finally {
            clearTimeout(timeout);
            pending.current = false;
            setBusy(false);
          }
        }}
      >
        <div className="about-heading">
          <span>
            <FlaskConical size={17} />
            NEW TEST SESSION
          </span>
          <button
            type="button"
            className="icon-button"
            aria-label="Close new session"
            disabled={busy}
            onClick={onClose}
          >
            <X size={18} />
          </button>
        </div>
        <h2 id="session-start-title">Give your test a name</h2>
        <p className="session-modal-scope">
          Project: <strong>{scope}</strong>
        </p>
        <label htmlFor="session-name">Session name</label>
        <input
          autoFocus
          id="session-name"
          maxLength={80}
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. Chat app with 20 users"
          required
          disabled={busy}
        />
        <p className="session-method">
          New and replacement containers in this scope join automatically. Run your test as usual,
          then finish the session here. One session can be active per project.
        </p>
        {error && (
          <div className="detail-notice" role="alert">
            {error}
          </div>
        )}
        <div className="session-form-actions">
          <button type="button" className="refresh-button" disabled={busy} onClick={onClose}>
            Cancel
          </button>
          <button className="session-primary" disabled={busy || !name.trim()}>
            <Play size={14} />
            {busy ? 'Starting…' : 'Start session'}
          </button>
        </div>
      </form>
    </dialog>
  );
}
export default function SessionsPanel({
  project,
  refresh,
  now,
  canStart,
  onInvestigate,
}: {
  project: string;
  refresh: number;
  now: number;
  canStart: boolean;
  onInvestigate?: OpenInvestigation;
}) {
  const [sessions, setSessions] = useState<TestSession[]>([]),
    [hasMore, setHasMore] = useState(false),
    [selected, setSelected] = useState<string | null>(null),
    [detail, setDetail] = useState<Detail | null>(null),
    [error, setError] = useState<string | null>(null),
    [detailError, setDetailError] = useState<string | null>(null),
    [loading, setLoading] = useState(true),
    [version, setVersion] = useState(0),
    [creating, setCreating] = useState(false),
    [comparing, setComparing] = useState(false),
    [finishing, setFinishing] = useState(false);
  const finishPending = useRef(false);
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    const timeout = setTimeout(() => controller.abort(), 8000);
    setLoading(true);
    const q = new URLSearchParams();
    if (project === 'standalone') q.set('standalone', 'true');
    else if (project.startsWith('project:')) q.set('project', project.slice(8));
    fetchJSON<{ sessions: TestSession[]; has_more: boolean }>(
      '/api/sessions?' + q,
      controller.signal,
    )
      .then((value) => {
        if (active) {
          setSessions(value.sessions);
          setHasMore(value.has_more);
          setError(null);
        }
      })
      .catch((reason) => {
        if (active)
          setError(
            reason instanceof Error && reason.name !== 'AbortError'
              ? reason.message
              : 'Session list timed out. Refresh to retry.',
          );
      })
      .finally(() => {
        clearTimeout(timeout);
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [project, refresh, version]);
  useEffect(() => {
    setSelected(null);
    setComparing(false);
    setDetail(null);
    setSessions([]);
  }, [project]);
  useEffect(() => {
    if (!selected) {
      setDetail(null);
      return;
    }
    const controller = new AbortController();
    let active = true;
    const timeout = setTimeout(() => controller.abort(), 10000);
    setDetailError(null);
    fetchJSON<Detail>('/api/sessions/' + selected, controller.signal)
      .then((value) => {
        if (active) setDetail(value);
      })
      .catch((reason) => {
        if (active)
          setDetailError(
            reason instanceof Error && reason.name !== 'AbortError'
              ? reason.message
              : 'Session summary timed out.',
          );
      })
      .finally(() => clearTimeout(timeout));
    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [selected, refresh, version]);
  const activeSession = sessions.find((s) => !s.finished_at),
    current = detail?.summary.session;
  const scope =
    project === 'standalone'
      ? 'Standalone containers'
      : project.startsWith('project:')
        ? project.slice(8)
        : 'All projects';
  return (
    <section className="sessions-panel" aria-label="Test sessions">
      <div className="sessions-heading">
        <div>
          <div className="section-label">LOCAL TESTING</div>
          <h2>{selected ? 'Session overview' : 'Your test sessions'}</h2>
          <p>
            {selected
              ? 'Resources and events within one named test.'
              : 'Name a test, run it locally, and review what happened.'}
          </p>
        </div>
        <div className="events-controls">
          <button
            className="refresh-button"
            aria-label="Refresh sessions"
            onClick={() => setVersion((v) => v + 1)}
            disabled={loading}
          >
            <RefreshCw size={14} className={loading ? 'spinning' : ''} />
          </button>
          {!selected && (
            <button className="refresh-button" onClick={() => setComparing((v) => !v)}>
              {comparing ? 'Close comparison' : 'Compare sessions'}
            </button>
          )}
          {!selected && !comparing && (
            <button
              className="session-primary"
              disabled={!project || !canStart || !!activeSession || loading}
              onClick={() => setCreating(true)}
            >
              <Play size={14} />
              Start session
            </button>
          )}
        </div>
      </div>
      {error && (
        <div className="detail-notice" role="alert">
          {error} Previously loaded sessions may be stale.
        </div>
      )}
      {!selected && comparing && (
        <SessionComparison
          sessions={sessions}
          listError={error}
          hasMore={hasMore}
          onInvestigate={onInvestigate}
        />
      )}
      {!selected && !comparing && (
        <>
          <div className="session-scope-note">
            <FlaskConical size={16} />
            <span>
              {!project
                ? 'Choose a project above to start a session. You can browse all existing sessions here.'
                : activeSession
                  ? 'A session is already active in ' + scope + '. Open it to continue or finish.'
                  : !canStart
                    ? 'Start a container in this scope and wait for fresh inventory to begin.'
                    : 'Ready to observe ' +
                      scope +
                      '. New and replacement containers join automatically.'}
            </span>
          </div>
          <div className="session-list">
            {sessions.map((s) => (
              <button
                key={s.id}
                className="session-card"
                onClick={() => {
                  setDetail(null);
                  setSelected(s.id);
                }}
              >
                <span className={'session-state ' + (!s.finished_at ? 'active' : '')}>
                  <span className={!s.finished_at ? 'live-dot' : 'muted-dot'} />
                  {s.finished_at ? 'Completed' : 'Active'}
                </span>
                <strong>{s.name}</strong>
                <span>{s.standalone ? 'Standalone containers' : s.project}</span>
                <span className="session-card-footer">
                  <span>{new Date(s.started_at).toLocaleString()}</span>
                  <span>
                    <Clock3 size={12} />
                    {duration(s.started_at, s.finished_at ?? new Date(now).toISOString())}
                  </span>
                </span>
              </button>
            ))}
          </div>
          {!sessions.length && !error && (
            <div className="event-empty">
              <FlaskConical size={28} />
              <h4>{loading ? 'Loading sessions…' : 'No sessions here yet'}</h4>
              <p>Try a named test such as “baseline chat traffic” to collect a focused summary.</p>
            </div>
          )}
          {hasMore && (
            <p className="detail-notice">
              Showing the latest 100 sessions. Choose a project to narrow the list.
            </p>
          )}
          <p className="session-method">
            Completed sessions expire seven days after finishing. Resource and event history lasts
            seven days, so older portions can become unavailable sooner.
          </p>
        </>
      )}
      {selected && (
        <>
          <button
            className="session-back"
            onClick={() => {
              setSelected(null);
              setDetail(null);
            }}
          >
            <ArrowLeft size={14} />
            All sessions in this scope
          </button>
          {detailError && (
            <div className="detail-notice" role="alert">
              {detailError} Previously loaded results may be stale.
            </div>
          )}
          {detail && current ? (
            <>
              <div className="session-detail-heading">
                <div>
                  <span className={'session-state ' + (!current.finished_at ? 'active' : '')}>
                    <span className={!current.finished_at ? 'live-dot' : 'muted-dot'} />
                    {current.finished_at ? 'Completed' : 'Active'}
                  </span>
                  <h2>{current.name}</h2>
                  <p>
                    {current.standalone ? 'Standalone containers' : current.project} · Started{' '}
                    {new Date(current.started_at).toLocaleString()}
                    {current.finished_at
                      ? ' · Finished ' + new Date(current.finished_at).toLocaleString()
                      : ''}
                  </p>
                </div>
                {!current.finished_at ? (
                  <button
                    className="session-primary session-finish"
                    disabled={finishing}
                    onClick={async () => {
                      if (finishPending.current) return;
                      finishPending.current = true;
                      setFinishing(true);
                      setDetailError(null);
                      const controller = new AbortController();
                      const timeout = setTimeout(() => controller.abort(), 8000);
                      try {
                        const response = await fetch('/api/sessions/' + current.id + '/finish', {
                          method: 'POST',
                          headers: { 'Content-Type': 'application/json' },
                          body: '{}',
                          signal: controller.signal,
                        });
                        const body = await response.json();
                        if (!response.ok) throw new Error(body.error || 'Unable to finish session');
                        setVersion((v) => v + 1);
                      } catch (reason) {
                        setDetailError(
                          reason instanceof Error && reason.name !== 'AbortError'
                            ? reason.message
                            : 'Finish request timed out. Refresh to check whether it was saved before retrying.',
                        );
                      } finally {
                        clearTimeout(timeout);
                        finishPending.current = false;
                        setFinishing(false);
                      }
                    }}
                  >
                    <Square size={13} />
                    {finishing ? 'Finishing…' : 'Finish session'}
                  </button>
                ) : (
                  <span className="session-finished">
                    <Check size={14} />
                    Finished
                  </span>
                )}
              </div>
              {(!detail.collector.healthy ||
                !detail.collector.last_inventory ||
                now - Date.parse(detail.collector.last_inventory) >
                  Math.max(3 * current.sample_interval_seconds, 15) * 1000) &&
                !current.finished_at && (
                  <div className="detail-notice" role="status">
                    Resource collection needs attention. The session remains active; missing
                    observations are not filled in.
                  </div>
                )}
              <SessionResults
                summary={detail.summary}
                notice={detail.coverage_notice}
                onInvestigate={onInvestigate}
              />
            </>
          ) : (
            !detailError && <p className="event-empty">Loading session summary…</p>
          )}
        </>
      )}
      {creating && (
        <StartSessionModal
          project={project}
          onClose={() => {
            setCreating(false);
            setVersion((v) => v + 1);
          }}
          onStarted={(session) => {
            setCreating(false);
            setDetail(null);
            setSelected(session.id);
            setVersion((v) => v + 1);
          }}
        />
      )}
    </section>
  );
}

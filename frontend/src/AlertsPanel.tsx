import { useEffect, useRef, useState } from 'react';
import { AlertTriangle, Bell, Check, RefreshCw, Settings2, X } from 'lucide-react';
import { fetchJSON, numeric } from './format';
import { recordedContainer, type OpenInvestigation } from './investigation';
export interface LocalAlert {
  id: string;
  project: string;
  container_id: string;
  name: string;
  service: string;
  kind: string;
  evidence_from: string;
  at: string;
  last_observed_at: string;
  value: number | null;
  threshold: number | null;
  detail: string;
  resolved_at: string | null;
  resolution: string;
  dismissed_at: string | null;
}
interface Rule {
  project: string;
  enabled: boolean;
  cpu_enabled: boolean;
  memory_enabled: boolean;
  cpu_threshold: number;
  memory_threshold: number;
  nonzero_exit: boolean;
  oom: boolean;
  unhealthy: boolean;
  enabled_since: string;
}
interface Inbox {
  alerts: LocalAlert[];
  needs_review: number;
  has_more: boolean;
  sample_interval_seconds: number;
  server_time: string;
  collector: { healthy: boolean; last_inventory: string | null; message: string };
}
export function alertTitle(kind: string) {
  return (
    (
      {
        cpu: 'Sustained CPU usage',
        memory: 'Sustained memory usage',
        nonzero_exit: 'Nonzero exit',
        oom: 'Out of memory',
        unhealthy: 'Unhealthy container',
      } as Record<string, string>
    )[kind] ?? kind
  );
}
export function alertWindow(a: LocalAlert, now = Date.now()) {
  const from = Math.max(Date.parse(a.evidence_from) - 120000, now - 7 * 86400000),
    to = Math.min(Date.parse(a.last_observed_at) + 120000, now);
  return Number.isFinite(from) && Number.isFinite(to) && from < to
    ? { from: new Date(from).toISOString(), to: new Date(to).toISOString() }
    : null;
}
function scopeQuery(project: string) {
  const q = new URLSearchParams();
  if (project === 'standalone') q.set('standalone', 'true');
  else if (project.startsWith('project:')) q.set('project', project.slice(8));
  return q;
}
async function writeJSON(url: string, method: string, body: unknown) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 8000);
  try {
    const response = await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal: controller.signal,
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error ?? 'Alert change failed');
    return data;
  } finally {
    clearTimeout(timer);
  }
}
function RuleModal({
  rule,
  project,
  onClose,
  onSaved,
}: {
  rule: Rule;
  project: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [draft, setDraft] = useState(rule),
    [busy, setBusy] = useState(false),
    [error, setError] = useState<string | null>(null);
  const dialog = useRef<HTMLDialogElement>(null),
    pending = useRef(false);
  useEffect(() => {
    const el = dialog.current,
      trigger = document.activeElement;
    el?.showModal();
    return () => {
      el?.close();
      if (trigger instanceof HTMLElement && trigger.isConnected) trigger.focus();
    };
  }, []);
  const toggle = (
    key: 'enabled' | 'cpu_enabled' | 'memory_enabled' | 'nonzero_exit' | 'oom' | 'unhealthy',
    label: string,
  ) => (
    <label className="alert-toggle">
      <input
        type="checkbox"
        checked={draft[key]}
        disabled={busy}
        onChange={(e) => setDraft({ ...draft, [key]: e.target.checked })}
      />
      {label}
    </label>
  );
  return (
    <dialog
      className="workspace-modal session-modal"
      ref={dialog}
      aria-labelledby="alert-rule-title"
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
          try {
            const { project: _, enabled_since: __, ...body } = draft;
            await writeJSON('/api/alert-rules?' + scopeQuery(project), 'PUT', body);
            onSaved();
          } catch (e) {
            setError(e instanceof Error ? e.message : 'Unable to save. Refresh before retrying.');
          } finally {
            pending.current = false;
            setBusy(false);
          }
        }}
      >
        <div className="about-heading">
          <span>
            <Bell size={17} />
            PROJECT ALERTS
          </span>
          <button
            type="button"
            className="icon-button"
            aria-label="Close alert settings"
            onClick={onClose}
            disabled={busy}
          >
            <X size={18} />
          </button>
        </div>
        <h2 id="alert-rule-title">Alert settings</h2>
        <p className="session-modal-scope">
          {project === 'standalone' ? 'Standalone containers' : project.slice(8)}
        </p>
        {toggle('enabled', 'Enable alerts for this project')}
        <div className="alert-rule-section">
          <h3>Container events</h3>
          {toggle('nonzero_exit', 'Nonzero exits')}
          {toggle('oom', 'Out-of-memory events')}
          {toggle('unhealthy', 'Unhealthy health checks')}
          <p className="session-method">
            An intentional stop can produce a nonzero exit. Health alerts require a Docker health
            check.
          </p>
        </div>
        <div className="alert-rule-section">
          <h3>Resource thresholds · sustained for 30 seconds</h3>
          {toggle('cpu_enabled', 'Monitor CPU')}
          <label htmlFor="cpu-threshold">CPU threshold (% of one core)</label>
          <input
            id="cpu-threshold"
            type="number"
            min="0.01"
            max="100000"
            step="any"
            value={draft.cpu_threshold}
            disabled={busy}
            required
            onChange={(e) => setDraft({ ...draft, cpu_threshold: Number(e.target.value) })}
          />
          {toggle('memory_enabled', 'Monitor memory')}
          <label htmlFor="memory-threshold">Memory threshold (% of Docker-reported limit)</label>
          <input
            id="memory-threshold"
            type="number"
            min="0.01"
            max="100"
            step="any"
            value={draft.memory_threshold}
            disabled={busy}
            required
            onChange={(e) => setDraft({ ...draft, memory_threshold: Number(e.target.value) })}
          />
          <p className="session-method">
            100% CPU is one core. Memory excludes inactive file cache; Docker's limit may be
            inherited from the Engine. Missing readings reset pending evidence, without resolving an
            existing alert.
          </p>
        </div>
        <p className="session-method">
          Saving resets pending evidence and closes previous threshold/health episodes as “rule
          changed.” New alerts use observations after saving. Nothing is sent outside this app.
        </p>
        {error && (
          <div className="detail-notice" role="alert">
            {error}
          </div>
        )}
        <div className="session-form-actions">
          <button type="button" className="refresh-button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button className="session-primary" disabled={busy}>
            <Check size={14} />
            {busy ? 'Saving…' : 'Save alert settings'}
          </button>
        </div>
      </form>
    </dialog>
  );
}
export function AlertCard({
  alert: a,
  interval,
  now,
  onDismiss,
  onInvestigate,
  busy,
}: {
  alert: LocalAlert;
  interval: number;
  now: number;
  onDismiss: (id: string) => void;
  onInvestigate: OpenInvestigation;
  busy: boolean;
}) {
  const resource = a.kind === 'cpu' || a.kind === 'memory';
  const stale =
    resource && !a.resolved_at && now - Date.parse(a.last_observed_at) > interval * 1500;
  const range = alertWindow(a, now);
  const state = a.resolved_at
    ? a.resolution === 'recovered'
      ? 'Recovered'
      : a.resolution === 'rule_changed'
        ? 'Rule changed'
        : 'Container lifecycle changed'
    : resource
      ? 'Threshold episode'
      : a.kind === 'unhealthy'
        ? 'Last observed unhealthy'
        : 'Recorded event';
  return (
    <article className="alert-card">
      <div className="alert-card-heading">
        <span className="alert-glyph">
          <AlertTriangle size={18} />
        </span>
        <div>
          <h3>{alertTitle(a.kind)}</h3>
          <span className="alert-state">
            {state}
            {a.dismissed_at ? ' · Dismissed' : ''}
          </span>
        </div>
        {!a.dismissed_at && (
          <button className="refresh-button" disabled={busy} onClick={() => onDismiss(a.id)}>
            Dismiss
          </button>
        )}
      </div>
      <strong className="alert-target">{a.name || a.container_id.slice(0, 12)}</strong>
      <div className="event-meta">
        <span>
          {a.project || 'Standalone'}
          {a.service ? ' / ' + a.service : ''}
        </span>
        <code>{a.container_id.slice(0, 12)}</code>
      </div>
      <p>{a.detail}</p>
      {resource && (
        <p className="alert-reading">
          Latest high reading: {numeric(a.value)}% · Threshold:{' '}
          {a.threshold?.toLocaleString(undefined, { maximumSignificantDigits: 6 }) ?? '—'}%
        </p>
      )}
      {stale && (
        <p className="logs-notice">
          Evidence is stale or missing. This has not been marked recovered.
        </p>
      )}
      <div className="alert-times">
        <span>First alert: {new Date(a.at).toLocaleString()}</span>
        <span>Latest evidence: {new Date(a.last_observed_at).toLocaleString()}</span>
        {a.resolved_at && <span>Closed: {new Date(a.resolved_at).toLocaleString()}</span>}
      </div>
      <div className="investigation-actions">
        {(['resources', 'logs'] as const).map((tab) => (
          <button
            className="refresh-button"
            key={tab}
            disabled={!range}
            onClick={(e) => {
              if (range)
                onInvestigate(
                  {
                    container: recordedContainer(a.container_id, a.name, a.project, a.service),
                    range,
                    tab,
                    label:
                      'Alert · ' +
                      alertTitle(a.kind) +
                      (Date.parse(a.evidence_from) < now - 7 * 86400000
                        ? ' · retained portion only'
                        : ''),
                  },
                  e.currentTarget,
                );
            }}
          >
            {tab === 'resources' ? 'View resources' : 'View logs'}
          </button>
        ))}
      </div>
    </article>
  );
}
export default function AlertsPanel({
  project,
  refresh,
  now,
  onInvestigate,
  onChanged,
}: {
  project: string;
  refresh: number;
  now: number;
  onInvestigate: OpenInvestigation;
  onChanged: () => void;
}) {
  const [data, setData] = useState<Inbox | null>(null),
    [rule, setRule] = useState<Rule | null>(null),
    [error, setError] = useState<string | null>(null),
    [ruleError, setRuleError] = useState<string | null>(null),
    [loading, setLoading] = useState(true),
    [includeDismissed, setIncludeDismissed] = useState(false),
    [version, setVersion] = useState(0),
    [editing, setEditing] = useState(false),
    [busy, setBusy] = useState(false);
  const pending = useRef(false);
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    const timer = setTimeout(() => controller.abort(), 8000);
    const q = scopeQuery(project);
    q.set('include_dismissed', String(includeDismissed));
    setLoading(true);
    fetchJSON<Inbox>('/api/alerts?' + q, controller.signal)
      .then((value) => {
        if (active) {
          setData(value);
          setError(null);
        }
      })
      .catch((e) => {
        if (active) setError(e instanceof Error ? e.message : 'Alerts unavailable');
      })
      .finally(() => {
        clearTimeout(timer);
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      clearTimeout(timer);
      controller.abort();
    };
  }, [project, refresh, version, includeDismissed]);
  useEffect(() => {
    if (!project) {
      setRule(null);
      return;
    }
    const controller = new AbortController();
    let active = true;
    const timer = setTimeout(() => controller.abort(), 8000);
    fetchJSON<Rule>('/api/alert-rules?' + scopeQuery(project), controller.signal)
      .then((value) => {
        if (active) {
          setRule(value);
          setRuleError(null);
        }
      })
      .catch((e) => {
        if (active) setRuleError(e instanceof Error ? e.message : 'Rule unavailable');
      })
      .finally(() => clearTimeout(timer));
    return () => {
      active = false;
      clearTimeout(timer);
      controller.abort();
    };
  }, [project, version]);
  async function dismiss(id: string) {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    try {
      await writeJSON('/api/alerts/' + id + '/dismiss', 'POST', {});
      setVersion((v) => v + 1);
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Dismiss failed. Refresh before retrying.');
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  return (
    <section className="alerts-panel" aria-label="Local alerts">
      <div className="sessions-heading">
        <div>
          <div className="section-label">IN-APP NOTIFICATIONS</div>
          <h2>
            Alert inbox <span className="heading-count">{data?.needs_review ?? '—'}</span>
          </h2>
          <p>Recorded signals that need your attention.</p>
        </div>
        <div className="events-controls">
          <button
            className="refresh-button"
            aria-label="Refresh alerts"
            disabled={loading}
            onClick={() => setVersion((v) => v + 1)}
          >
            <RefreshCw size={14} className={loading ? 'spinning' : ''} />
          </button>
          <button
            className="refresh-button"
            disabled={!project || !rule || !!ruleError}
            onClick={() => setEditing(true)}
          >
            <Settings2 size={14} />
            Configure alerts
          </button>
        </div>
      </div>
      {error && (
        <div className="detail-notice" role="alert">
          {error} Previous inbox data may be stale.
        </div>
      )}
      {ruleError && (
        <div className="detail-notice" role="alert">
          {ruleError}
        </div>
      )}
      <div className="session-scope-note">
        <Bell size={16} />
        <span>
          {!project
            ? 'Choose a project above to enable or configure its alerts. Projects are off by default.'
            : rule?.enabled
              ? 'Alerts are enabled for this project. Repeated threshold/health observations stay in one episode until recovery.'
              : 'Alerts are off for this project. Configure alerts to opt in.'}
        </span>
      </div>
      <label className="alert-toggle">
        <input
          type="checkbox"
          checked={includeDismissed}
          onChange={(e) => setIncludeDismissed(e.target.checked)}
        />
        Include dismissed alerts
      </label>
      <p className="session-method">
        Dismissal acknowledges this occurrence; it does not resolve a condition. Alerts retain seven
        days of recent evidence. Thresholds require consecutive recorded evidence over 30 seconds;
        collection gaps are not filled in.
      </p>
      {data && !data.collector.healthy && (
        <div className="detail-notice" role="status">
          Resource collection needs attention. Alert evidence may be incomplete; missing
          observations are not a recovery.
        </div>
      )}
      <div className="alert-grid">
        {data?.alerts.map((a) => (
          <AlertCard
            key={a.id}
            alert={a}
            interval={data.sample_interval_seconds}
            now={now}
            onDismiss={dismiss}
            onInvestigate={onInvestigate}
            busy={busy}
          />
        ))}
      </div>
      {!data?.alerts.length && !error && (
        <div className="event-empty">
          <Bell size={28} />
          <h4>{loading ? 'Loading alerts…' : 'No alerts in this inbox'}</h4>
          <p>
            Only opted-in projects create alerts. An empty inbox does not prove that collection
            covered every event.
          </p>
        </div>
      )}
      {data?.has_more && (
        <p className="detail-notice">
          Showing the latest 100 matching alerts. Choose a project to narrow the inbox.
        </p>
      )}
      {editing && rule && (
        <RuleModal
          rule={rule}
          project={project}
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false);
            setVersion((v) => v + 1);
            onChanged();
          }}
        />
      )}
    </section>
  );
}

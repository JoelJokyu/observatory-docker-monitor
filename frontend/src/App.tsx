import { useCallback, useEffect, useRef, useState } from 'react';
import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';
import {
  Activity,
  Bell,
  Box,
  Boxes,
  Check,
  ChevronRight,
  CircleHelp,
  Clock3,
  Copy,
  Cpu,
  Database,
  HardDrive,
  Layers3,
  FlaskConical,
  MemoryStick,
  Moon,
  Network,
  PanelRightClose,
  RefreshCw,
  Search,
  Server,
  Sun,
  X,
} from 'lucide-react';
import type { Container, History, Inventory, Sample } from './types';
import { age, bytes, fetchJSON, inventoryFresh, isLive, numeric, status } from './format';
import { Chart } from './chart';
import LogsPanel from './LogsPanel';
import AboutModal from './AboutModal';
import EventsPanel from './EventsPanel';
import SessionsPanel from './SessionsPanel';
import AlertsPanel from './AlertsPanel';
import type { Investigation, OpenInvestigation } from './investigation';

const pct = (n: number | null | undefined) => (n == null ? '—' : numeric(n) + '%');
const perSecond = (n: number | null | undefined) => (n == null ? '—' : bytes(n) + '/s');
const windows = [
  { value: 900, label: '15 min' },
  { value: 3600, label: '1 hour' },
  { value: 21600, label: '6 hours' },
  { value: 86400, label: '24 hours' },
  { value: 604800, label: '7 days' },
];
const cpuSeries = [{ key: 'cpu_percent' as const, label: 'CPU', color: 'var(--chart-green)' }];
const memorySeries = [
  { key: 'memory_bytes' as const, label: 'Used', color: 'var(--chart-purple)' },
];
const pidSeries = [{ key: 'pids' as const, label: 'Tasks', color: 'var(--chart-amber)' }];
const networkSeries = [
  { key: 'network_rx_bytes_per_second' as const, label: 'Receive', color: 'var(--chart-blue)' },
  { key: 'network_tx_bytes_per_second' as const, label: 'Send', color: 'var(--chart-violet)' },
];
const blockSeries = [
  { key: 'block_read_bytes_per_second' as const, label: 'Read', color: 'var(--chart-amber)' },
  { key: 'block_write_bytes_per_second' as const, label: 'Write', color: 'var(--chart-green)' },
];
const numberFormat = (n: number | null | undefined) => numeric(n);
function ThemeButton() {
  const [theme, setTheme] = useState(() => document.documentElement.dataset.theme ?? 'dark');
  const toggle = () => {
    const next = theme === 'dark' ? 'light' : 'dark';
    setTheme(next);
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem('observatory-theme', next);
    } catch {
      /* Storage can be disabled. */
    }
  };
  return (
    <button
      className="icon-button"
      onClick={toggle}
      title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
      aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
    >
      {theme === 'dark' ? <Sun size={17} /> : <Moon size={17} />}
    </button>
  );
}
function Badge({ value }: { value: string }) {
  return (
    <span className={'badge ' + value.toLowerCase()}>
      <i />
      {value}
    </span>
  );
}
function IOPair({
  first,
  second,
  labels = ['↓', '↑'],
}: {
  first: number | null | undefined;
  second: number | null | undefined;
  labels?: string[];
}) {
  return (
    <div className="io-pair">
      <span>
        <em>{labels[0]}</em>
        {bytes(first)}
      </span>
      <span>
        <em>{labels[1]}</em>
        {bytes(second)}
      </span>
    </div>
  );
}
function Stat({
  icon,
  label,
  value,
  sub,
  className = '',
}: {
  icon: React.ReactNode;
  label: string;
  value: React.ReactNode;
  sub: React.ReactNode;
  className?: string;
}) {
  return (
    <article className={'stat-card ' + className}>
      <div className="stat-label">
        {label}
        {icon}
      </div>
      <strong>{value}</strong>
      <p>{sub}</p>
    </article>
  );
}
function CopyID({ id }: { id: string }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );
  return (
    <button
      className="copy-id"
      title="Copy container ID"
      aria-label="Copy container ID"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(id);
          setCopied(true);
          if (timer.current) clearTimeout(timer.current);
          timer.current = setTimeout(() => setCopied(false), 1800);
        } catch {
          setCopied(false);
        }
      }}
    >
      {id.slice(0, 12)}
      {copied ? <Check size={12} /> : <Copy size={12} />}
    </button>
  );
}
function DetailsPanel({
  container,
  data,
  error,
  now,
  onClose,
  width,
  onWidth,
  refresh,
  investigation,
  onUnpin,
  onInvestigate,
}: {
  container: Container;
  data: Inventory | null;
  error: string | null;
  now: number;
  onClose: () => void;
  width: number;
  onWidth: (n: number) => void;
  refresh: number;
  investigation: Investigation | null;
  onUnpin: () => void;
  onInvestigate: OpenInvestigation;
}) {
  const [tab, setTab] = useState<string>(investigation?.tab ?? 'resources'),
    [windowSeconds, setWindowSeconds] = useState(3600),
    [history, setHistory] = useState<History | null>(null),
    [loading, setLoading] = useState(true),
    [historyError, setHistoryError] = useState<string | null>(null);
  const panelRef = useRef<HTMLElement>(null);
  const [compact, setCompact] = useState(() => window.matchMedia('(max-width:1400px)').matches);
  useEffect(() => {
    const media = window.matchMedia('(max-width:1400px)');
    const update = () => setCompact(media.matches);
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);
  const closeButton = useRef<HTMLButtonElement>(null),
    drag = useRef<{ x: number; width: number } | null>(null);
  useEffect(() => {
    closeButton.current?.focus();
    const handler = (e: KeyboardEvent) => {
      if (document.querySelector('dialog[open]')) return;
      if (e.key === 'Escape') onClose();
      if (e.key === 'Tab' && window.matchMedia('(max-width:1400px)').matches) {
        const controls = Array.from(
          panelRef.current?.querySelectorAll<HTMLElement>('button,input,select,[tabindex="0"]') ??
            [],
        ).filter((el) => el.getClientRects().length > 0);
        const first = controls[0],
          last = controls[controls.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last?.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first?.focus();
        }
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onClose]);
  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    setLoading(true);
    setHistoryError(null);
    const to = new Date(investigation?.range.to ?? Date.now()),
      from = new Date(investigation?.range.from ?? to.getTime() - windowSeconds * 1000);
    const params = new URLSearchParams({ from: from.toISOString(), to: to.toISOString() });
    const timeout = setTimeout(() => controller.abort(), 8000);
    fetchJSON<History>(
      '/api/containers/' + encodeURIComponent(container.id) + '/history?' + params,
      controller.signal,
    )
      .then((value) => {
        if (active) setHistory(value);
      })
      .catch((reason) => {
        if (active) {
          setHistoryError(
            reason instanceof Error && reason.name !== 'AbortError'
              ? reason.message
              : 'History request timed out.',
          );
          setHistory(null);
        }
      })
      .finally(() => {
        if (active) setLoading(false);
        clearTimeout(timeout);
      });
    return () => {
      active = false;
      controller.abort();
      clearTimeout(timeout);
    };
  }, [container.id, windowSeconds, refresh, investigation]);
  const interval = data?.sample_interval_seconds ?? 10,
    fresh = !error && inventoryFresh(data, now),
    live = isLive(container, data, now, error),
    sample = live ? container.latest : null;
  const beginResize = (e: ReactPointerEvent<HTMLDivElement>) => {
    drag.current = { x: e.clientX, width };
    e.currentTarget.setPointerCapture(e.pointerId);
  };
  const moveResize = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (drag.current)
      onWidth(Math.max(430, Math.min(760, drag.current.width + drag.current.x - e.clientX)));
  };
  const chartProps = { history, interval, loading, error: historyError };
  return (
    <>
      <button className="panel-backdrop" aria-label="Close container details" onClick={onClose} />
      <aside
        ref={panelRef}
        role={compact ? 'dialog' : 'complementary'}
        aria-modal={compact ? true : undefined}
        className="detail-panel"
        aria-label={'Container details: ' + container.name}
      >
        <div
          className="panel-resizer"
          role="separator"
          aria-label="Resize container details"
          aria-orientation="vertical"
          aria-valuemin={430}
          aria-valuemax={760}
          aria-valuenow={width}
          tabIndex={0}
          onPointerDown={beginResize}
          onPointerMove={moveResize}
          onPointerUp={() => {
            drag.current = null;
          }}
          onPointerCancel={() => {
            drag.current = null;
          }}
          onKeyDown={(e) => {
            if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
              e.preventDefault();
              onWidth(Math.max(430, Math.min(760, width + (e.key === 'ArrowLeft' ? 20 : -20))));
            }
          }}
        />
        <div className="detail-top">
          <span>
            <Box size={14} /> CONTAINER DETAILS
          </span>
          <button
            ref={closeButton}
            className="icon-button"
            onClick={onClose}
            aria-label="Close container details"
            title="Close details (Escape)"
          >
            <PanelRightClose size={18} />
          </button>
        </div>
        <div className="detail-identity">
          <div className="detail-icon">
            <Box size={23} />
          </div>
          <div>
            <h2 title={container.name}>{container.name}</h2>
            <div className="identity-meta">
              <Badge value={status(container, fresh)} />
              <CopyID id={container.id} />
            </div>
          </div>
        </div>
        <div className="detail-tabs" role="tablist" aria-label="Container detail sections">
          {[
            { id: 'resources', label: 'Resources' },
            { id: 'io', label: 'Network & disk' },
            { id: 'logs', label: 'Logs' },
            { id: 'events', label: 'Events' },
            { id: 'details', label: 'Details' },
          ].map((t) => (
            <button
              key={t.id}
              role="tab"
              aria-selected={tab === t.id}
              aria-controls={'content-' + t.id}
              id={'tab-' + t.id}
              className={tab === t.id ? 'active' : ''}
              onClick={() => setTab(t.id)}
              onKeyDown={(e) => {
                if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
                  e.preventDefault();
                  const tabs = ['resources', 'io', 'logs', 'events', 'details'];
                  const index =
                    (tabs.indexOf(t.id) + (e.key === 'ArrowRight' ? 1 : tabs.length - 1)) %
                    tabs.length;
                  setTab(tabs[index]);
                  document.getElementById('tab-' + tabs[index])?.focus();
                }
              }}
            >
              {t.label}
            </button>
          ))}
        </div>
        <div
          className="detail-scroll"
          role="tabpanel"
          id={'content-' + tab}
          aria-labelledby={'tab-' + tab}
        >
          {investigation && (
            <div className="investigation-context">
              <strong>{investigation.label}</strong>
              <span>
                {new Date(investigation.range.from).toLocaleString()} –{' '}
                {new Date(investigation.range.to).toLocaleString()}
              </span>
              <p>
                This window stays fixed across Resources and Logs. Matching times help
                investigation; they do not establish a cause.
              </p>
              <button className="refresh-button" onClick={onUnpin}>
                Return to recent history
              </button>
            </div>
          )}
          {!live && tab !== 'logs' && tab !== 'events' && (
            <div className="detail-notice">
              <Clock3 size={14} />
              {!fresh
                ? 'Collection is unavailable. Showing recorded history.'
                : container.recorded_only
                  ? 'Recorded identity; current Docker presence is unknown.'
                  : !container.present
                    ? 'This container was removed. Its history is preserved.'
                    : container.state !== 'running'
                      ? 'This container is not running. Its history is preserved.'
                      : 'Waiting for fresh measurements.'}
            </div>
          )}
          {tab === 'events' ? (
            <EventsPanel
              containerID={container.id}
              refresh={refresh}
              onInvestigate={onInvestigate}
            />
          ) : tab === 'logs' ? (
            <LogsPanel
              container={container}
              windowSeconds={windowSeconds}
              onWindow={setWindowSeconds}
              windows={windows}
              resourceRange={history}
              fixedRange={investigation?.range}
              now={now}
            />
          ) : tab === 'details' ? (
            <>
              <div className="section-label">CONTAINER IDENTITY</div>
              <dl className="metadata">
                <dt>Name</dt>
                <dd>{container.name}</dd>
                <dt>Container ID</dt>
                <dd className="mono wrap">{container.id}</dd>
                <dt>Project / service</dt>
                <dd>
                  {container.project || 'Standalone'}
                  {container.service ? ' / ' + container.service : ''}
                </dd>
                <dt>Image</dt>
                <dd className="mono wrap">{container.image}</dd>
                <dt>Last observed state</dt>
                <dd>{container.present ? container.state : 'Removed'}</dd>
                <dt>Inventory seen</dt>
                <dd>
                  {container.recorded_only
                    ? 'Not available in inventory'
                    : new Date(container.seen_at).toLocaleString()}
                </dd>
                <dt>Last sample</dt>
                <dd>
                  {container.latest
                    ? new Date(container.latest.at).toLocaleString()
                    : 'Not recorded'}
                </dd>
                <dt>Memory limit</dt>
                <dd>
                  {bytes(sample?.memory_limit_bytes)}
                  <small>Docker-reported ceiling; may be inherited from the Engine.</small>
                </dd>
              </dl>
              <div className="explainer">
                <Database size={17} />
                <div>
                  <h4>History follows the container ID</h4>
                  <p>
                    Recreating a container starts a separate history. Stopped and removed containers
                    keep their recorded measurements within the seven-day retention window.
                  </p>
                </div>
              </div>
            </>
          ) : (
            <>
              {!investigation &&
                (tab === 'resources' ? (
                  <div className="detail-stats">
                    <div>
                      <span>CPU usage</span>
                      <strong>{pct(sample?.cpu_percent)}</strong>
                      <small>100% = one core</small>
                    </div>
                    <div>
                      <span>Memory</span>
                      <strong>{bytes(sample?.memory_bytes)}</strong>
                      <small>Inactive cache excluded</small>
                    </div>
                    <div>
                      <span>PIDs</span>
                      <strong>{numeric(sample?.pids, 0)}</strong>
                      <small>Processes & threads</small>
                    </div>
                  </div>
                ) : (
                  <div className="detail-io-stats">
                    <div>
                      <Network size={15} />
                      <span>NET I/O</span>
                      <IOPair first={sample?.network_rx_bytes} second={sample?.network_tx_bytes} />
                    </div>
                    <div>
                      <HardDrive size={15} />
                      <span>BLOCK I/O</span>
                      <IOPair
                        first={sample?.block_read_bytes}
                        second={sample?.block_write_bytes}
                        labels={['R', 'W']}
                      />
                    </div>
                    <p>
                      Cumulative bytes in the current container run. Charts below show transfer
                      rates.
                    </p>
                  </div>
                ))}
              <div className="history-toolbar">
                <span>
                  <Activity size={14} /> History
                </span>
                <label className="sr-only" htmlFor="history-window">
                  History time window
                </label>
                <select
                  id="history-window"
                  disabled={!!investigation}
                  value={windowSeconds}
                  onChange={(e) => setWindowSeconds(Number(e.target.value))}
                >
                  {windows.map((w) => (
                    <option key={w.value} value={w.value}>
                      {investigation ? 'Fixed investigation window' : 'Last ' + w.label}
                    </option>
                  ))}
                </select>
              </div>
              {historyError && (
                <p className="history-error" role="status">
                  {historyError}
                </p>
              )}
              {tab === 'resources' ? (
                <>
                  <Chart
                    title="CPU utilization"
                    subtitle="Percentage of one CPU core"
                    series={cpuSeries}
                    format={pct}
                    {...chartProps}
                  />
                  <Chart
                    title="Memory usage"
                    subtitle="Working set · inactive file cache excluded"
                    series={memorySeries}
                    format={bytes}
                    {...chartProps}
                  />
                  <Chart
                    title="PIDs"
                    subtitle="Processes and threads in the container"
                    series={pidSeries}
                    format={numberFormat}
                    {...chartProps}
                  />
                </>
              ) : (
                <>
                  <Chart
                    title="Network traffic"
                    subtitle="Bytes per second across container interfaces"
                    series={networkSeries}
                    format={perSecond}
                    {...chartProps}
                  />
                  <Chart
                    title="Block I/O"
                    subtitle="Bytes per second read from and written to devices"
                    series={blockSeries}
                    format={perSecond}
                    {...chartProps}
                  />
                  <div className="explainer">
                    <CircleHelp size={17} />
                    <div>
                      <h4>No reading is different from zero</h4>
                      <p>
                        Unsupported counters and the first rate calculation show a dash. Counter
                        resets start a new baseline. Block I/O is device activity, not disk space.
                      </p>
                    </div>
                  </div>
                </>
              )}
              <p className="history-caption">
                <i />
                {history?.samples.reduce((n, b) => n + b.count, 0).toLocaleString() ?? '—'}{' '}
                observations · Seven-day retention
                <br />
                <span>Charts show bucket averages. Missing periods remain gaps.</span>
              </p>
            </>
          )}
        </div>
        <div className="detail-footer">
          <span className={live ? 'live-dot' : 'muted-dot'} />
          {live ? 'Receiving measurements' : 'No current measurements'}
          <span>{age(container.latest?.at, now)}</span>
        </div>
      </aside>
    </>
  );
}

export default function App() {
  const [data, setData] = useState<Inventory | null>(null),
    [error, setError] = useState<string | null>(null),
    [busy, setBusy] = useState(false),
    [now, setNow] = useState(Date.now()),
    [filter, setFilter] = useState('all'),
    [query, setQuery] = useState(''),
    [selected, setSelected] = useState<string | null>(null),
    [refreshVersion, setRefreshVersion] = useState(0),
    [help, setHelp] = useState(false),
    [view, setView] = useState('containers'),
    [project, setProject] = useState('');
  const [investigation, setInvestigation] = useState<Investigation | null>(null);
  const [investigationVersion, setInvestigationVersion] = useState(0);
  const investigate: OpenInvestigation = (value, trigger) => {
    selectedTrigger.current = trigger;
    setInvestigation(value);
    setInvestigationVersion((v) => v + 1);
    setSelected(value.container.id);
  };
  const [alertCount, setAlertCount] = useState(0),
    [alertError, setAlertError] = useState(false),
    [alertVersion, setAlertVersion] = useState(0),
    [alertRefresh, setAlertRefresh] = useState(0);
  useEffect(() => {
    let active = true;
    let request: AbortController | null = null;
    async function load() {
      if (request) return;
      const controller = new AbortController();
      request = controller;
      const timeout = setTimeout(() => controller.abort(), 8000);
      try {
        const result = await fetchJSON<{ needs_review: number }>('/api/alerts', controller.signal);
        if (active) {
          setAlertCount(result.needs_review);
          setAlertError(false);
        }
      } catch {
        if (active) setAlertError(true);
      } finally {
        clearTimeout(timeout);
        request = null;
        if (active) setAlertRefresh((v) => v + 1);
      }
    }
    void load();
    const poll = setInterval(() => void load(), 10000);
    return () => {
      active = false;
      request?.abort();
      clearInterval(poll);
    };
  }, [alertVersion]);
  const [panelWidth, setPanelWidth] = useState(() => {
    try {
      return Math.max(
        430,
        Math.min(760, Number(localStorage.getItem('observatory-panel-width')) || 520),
      );
    } catch {
      return 520;
    }
  });
  const selectedTrigger = useRef<HTMLElement | null>(null),
    request = useRef<AbortController | null>(null),
    active = useRef(true),
    firstLoad = useRef(true);
  const close = useCallback(() => {
    setSelected(null);
    requestAnimationFrame(() => {
      if (selectedTrigger.current?.isConnected) selectedTrigger.current.focus();
      else document.querySelector<HTMLElement>('.rail-item.selected')?.focus();
    });
  }, []);
  const refresh = useCallback(async () => {
    if (request.current) return;
    const controller = new AbortController();
    request.current = controller;
    setBusy(true);
    const timer = setTimeout(() => controller.abort(), 8000);
    try {
      const value = await fetchJSON<Inventory>('/api/containers', controller.signal);
      if (!active.current || request.current !== controller) return;
      setData(value);
      setError(null);
      setNow(Date.now());
      setRefreshVersion((v) => v + 1);
      if (firstLoad.current && value.containers.length) {
        firstLoad.current = false;
        if (window.matchMedia('(min-width:1401px)').matches)
          setSelected(
            value.containers.find((c) => c.present && c.state === 'running')?.id ??
              value.containers[0].id,
          );
      }
    } catch (reason) {
      if (active.current && request.current === controller)
        setError(
          reason instanceof Error && reason.name !== 'AbortError'
            ? reason.message
            : 'The platform did not respond in time.',
        );
    } finally {
      clearTimeout(timer);
      if (request.current === controller) {
        request.current = null;
        if (active.current) setBusy(false);
      }
    }
  }, []);
  useEffect(() => {
    active.current = true;
    void refresh();
    const poll = setInterval(() => void refresh(), 10000),
      clock = setInterval(() => setNow(Date.now()), 1000);
    return () => {
      active.current = false;
      request.current?.abort();
      request.current = null;
      clearInterval(poll);
      clearInterval(clock);
    };
  }, [refresh]);
  useEffect(() => {
    try {
      localStorage.setItem('observatory-panel-width', String(panelWidth));
    } catch {
      /* Storage can be disabled. */
    }
  }, [panelWidth]);
  const inventory = data?.containers ?? [];
  const projects = [
    ...new Set([...(data?.projects ?? []), ...inventory.map((c) => c.project).filter(Boolean)]),
  ].sort();
  const all = inventory.filter(
      (c) => !project || (project === 'standalone' ? !c.project : c.project === project.slice(8)),
    ),
    fresh = !error && inventoryFresh(data, now),
    healthy = fresh && !!data?.collector.healthy;
  const live = all.filter((c) => isLive(c, data, now, error));
  const present = all.filter((c) => c.present),
    running = present.filter((c) => c.state === 'running');
  const cpu = live.reduce((sum, c) => sum + (c.latest?.cpu_percent ?? 0), 0),
    memory = live.reduce((sum, c) => sum + (c.latest?.memory_bytes ?? 0), 0);
  const cpuCount = live.filter((c) => c.latest?.cpu_percent != null).length;
  const cpuKnown = cpuCount > 0,
    memoryKnown = live.length > 0;
  const sorted = all
    .filter(
      (c) =>
        (filter === 'all' ||
          (filter === 'running' && c.present && c.state === 'running') ||
          (filter === 'stopped' && (!c.present || c.state !== 'running'))) &&
        [c.name, c.image, c.id].some((v) => v.toLowerCase().includes(query.toLowerCase())),
    )
    .sort(
      (a, b) =>
        Number(b.present && b.state === 'running') - Number(a.present && a.state === 'running') ||
        Number(b.present) - Number(a.present) ||
        a.name.localeCompare(b.name),
    );
  const selectedContainer =
    inventory.find((c) => c.id === selected) ??
    (investigation?.container.id === selected ? investigation.container : null);
  const shellStyle = { '--panel-width': panelWidth + 'px' } as CSSProperties;
  return (
    <div className="app" style={shellStyle}>
      <nav className="rail" aria-label="Primary navigation">
        <a href="/" className="brand-mark" title="Observatory">
          <Activity size={23} />
          <span className="sr-only">Observatory home</span>
        </a>
        <span className="rail-rule" />
        <button
          className={'rail-item ' + (view === 'containers' ? 'selected' : '')}
          title="Containers"
          aria-label="Containers"
          onClick={() => {
            setView('containers');
            setFilter('all');
            setQuery('');
          }}
        >
          <Boxes size={20} />
        </button>
        <button
          className={'rail-item ' + (view === 'events' ? 'selected' : '')}
          title="Event timeline"
          aria-label="Event timeline"
          onClick={() => {
            setView('events');
            setSelected(null);
          }}
        >
          <Clock3 size={20} />
        </button>
        <button
          className={'rail-item ' + (view === 'sessions' ? 'selected' : '')}
          title="Test sessions"
          aria-label="Test sessions"
          onClick={() => {
            setView('sessions');
            setSelected(null);
          }}
        >
          <FlaskConical size={20} />
        </button>
        <button
          className={'rail-item ' + (view === 'alerts' ? 'selected' : '')}
          title="Local alerts · count across all projects"
          aria-label={
            'Local alerts' + (alertError ? ' · unavailable' : ' · ' + alertCount + ' need review')
          }
          onClick={() => {
            setView('alerts');
            setSelected(null);
          }}
        >
          <Bell size={20} />
          {(alertCount > 0 || alertError) && (
            <span className="alert-badge">
              {alertError ? '!' : alertCount > 99 ? '99+' : alertCount}
            </span>
          )}
        </button>
        <div className="rail-bottom">
          <button
            className={'rail-item ' + (help ? 'selected' : '')}
            title="About this workspace"
            aria-label="About this workspace"
            onClick={() => setHelp((v) => !v)}
          >
            <CircleHelp size={19} />
          </button>
          <span
            className={'connection-light ' + (healthy ? 'connected' : '')}
            title={healthy ? 'Collector connected' : 'Collector unavailable'}
          />
        </div>
      </nav>
      <div className="workspace">
        <header className="topbar">
          <div className="breadcrumb">
            <span>Observatory</span>
            <ChevronRight size={13} />
            <strong>
              {view === 'alerts'
                ? 'Alerts'
                : view === 'sessions'
                  ? 'Sessions'
                  : view === 'events'
                    ? 'Events'
                    : 'Containers'}
            </strong>
          </div>
          <div className="top-actions">
            <span className="environment">
              <span />
              Local environment
            </span>
            <span className="top-divider" />
            <ThemeButton />
          </div>
        </header>
        <div className={'workspace-body ' + (selectedContainer ? 'with-panel' : '')}>
          <main>
            <div className="page-heading">
              <div>
                <div className="eyebrow">INFRASTRUCTURE / DOCKER</div>
                <h1>
                  {view === 'alerts' ? (
                    'Local alerts'
                  ) : view === 'sessions' ? (
                    'Test sessions'
                  ) : view === 'events' ? (
                    'Event timeline'
                  ) : (
                    <>
                      Containers<span className="heading-count">{present.length}</span>
                    </>
                  )}
                </h1>
                <p>
                  {view === 'alerts'
                    ? 'Review signals. Investigate the evidence.'
                    : view === 'sessions'
                      ? 'Run a test. Keep its context. Understand the result.'
                      : view === 'events'
                        ? 'Follow the lifecycle of your local applications.'
                        : 'A clear view of your containers and their resource use.'}
                </p>
              </div>
              <button className="refresh-button" onClick={() => void refresh()} disabled={busy}>
                <RefreshCw size={14} className={busy ? 'spinning' : ''} />
                <span>{busy ? 'Refreshing' : 'Refresh'}</span>
              </button>
            </div>
            <div className={'collection-status ' + (!healthy ? 'degraded' : '')} role="status">
              <span className={healthy ? 'live-dot' : 'muted-dot'} />
              <strong>
                {healthy
                  ? 'Collection active'
                  : data
                    ? 'Collection needs attention'
                    : 'Connecting to your environment'}
              </strong>
              <span className="status-description">
                {error ??
                  (!healthy
                    ? data?.collector.message
                    : 'Samples every ' + (data?.sample_interval_seconds ?? 10) + ' seconds') ??
                  'Waiting for a fresh Docker inventory'}
              </span>
              <span className="status-engine">
                <Server size={12} />1 Docker Engine
              </span>
            </div>
            {view !== 'alerts' && (alertCount > 0 || alertError) && (
              <button
                className="alert-review-banner"
                onClick={() => {
                  setProject('');
                  setView('alerts');
                  setSelected(null);
                }}
              >
                <Bell size={15} />
                {alertError
                  ? 'Alert inbox unavailable · open to retry'
                  : alertCount + ' alerts need review across all projects'}
              </button>
            )}
            <div className="project-toolbar">
              <label htmlFor="project-scope">
                <Layers3 size={15} />
                Project
              </label>
              <select
                id="project-scope"
                value={project}
                onChange={(e) => {
                  setProject(e.target.value);
                  setSelected(null);
                }}
              >
                <option value="">All projects</option>
                {projects.map((p) => (
                  <option key={p} value={'project:' + p}>
                    {p}
                  </option>
                ))}
                <option value="standalone">Standalone containers</option>
              </select>
              <span>
                {project ? 'Showing this project only' : 'One workspace, all your applications'}
              </span>
            </div>
            {view === 'containers' && (
              <section className="stats-grid" aria-label="Environment summary">
                <Stat
                  label="Running containers"
                  icon={<Box size={15} />}
                  value={
                    fresh ? (
                      <>
                        {running.length}
                        <small>/ {present.length}</small>
                      </>
                    ) : (
                      '—'
                    )
                  }
                  sub={
                    fresh
                      ? present.length - running.length + ' stopped or inactive'
                      : 'Waiting for fresh inventory'
                  }
                />
                <Stat
                  label="Total CPU"
                  icon={<Cpu size={15} />}
                  value={cpuKnown ? pct(cpu) : '—'}
                  sub={
                    cpuKnown
                      ? 'Across ' + cpuCount + ' reporting containers'
                      : 'Waiting for valid CPU samples'
                  }
                  className="cpu-stat"
                />
                <Stat
                  label="Memory in use"
                  icon={<MemoryStick size={15} />}
                  value={memoryKnown ? bytes(memory) : '—'}
                  sub="Across reporting containers"
                />
                <Stat
                  label="Fresh samples"
                  icon={<Activity size={15} />}
                  value={
                    fresh ? (
                      <>
                        {live.length}
                        <small>/ {running.length}</small>
                      </>
                    ) : (
                      '—'
                    )
                  }
                  sub={
                    healthy ? (
                      <>
                        <span className="tiny-dot" />
                        Collector connected
                      </>
                    ) : (
                      'Collection unavailable'
                    )
                  }
                  className="fresh-stat"
                />
              </section>
            )}
            {view === 'alerts' ? (
              <AlertsPanel
                key={project}
                project={project}
                refresh={alertRefresh}
                now={now}
                onInvestigate={investigate}
                onChanged={() => setAlertVersion((v) => v + 1)}
              />
            ) : view === 'sessions' ? (
              <SessionsPanel
                key={project}
                project={project}
                refresh={refreshVersion}
                now={now}
                canStart={fresh && present.length > 0}
                onInvestigate={investigate}
              />
            ) : view === 'events' ? (
              <EventsPanel
                project={project}
                refresh={refreshVersion}
                onInvestigate={investigate}
                inspectableIDs={new Set(inventory.map((c) => c.id))}
                onInspect={(id, trigger) => {
                  selectedTrigger.current = trigger;
                  setInvestigation(null);
                  setSelected(id);
                }}
              />
            ) : (
              <section className="container-section" aria-label="Container inventory">
                <div className="list-toolbar">
                  <div className="filter-tabs" aria-label="Filter containers">
                    {[
                      { id: 'all', label: 'All containers', count: all.length },
                      { id: 'running', label: 'Running', count: running.length },
                      { id: 'stopped', label: 'Inactive', count: all.length - running.length },
                    ].map((f) => (
                      <button
                        key={f.id}
                        className={filter === f.id ? 'active' : ''}
                        aria-pressed={filter === f.id}
                        onClick={() => setFilter(f.id)}
                      >
                        {f.label}
                        <span>{f.count}</span>
                      </button>
                    ))}
                  </div>
                  <div className="search-box">
                    <Search size={14} />
                    <input
                      aria-label="Search containers"
                      placeholder="Search containers…"
                      value={query}
                      onChange={(e) => setQuery(e.target.value)}
                    />
                    {query && (
                      <button aria-label="Clear search" onClick={() => setQuery('')}>
                        <X size={12} />
                      </button>
                    )}
                  </div>
                </div>
                <div className="table-scroll">
                  <table>
                    <thead>
                      <tr>
                        <th>Container</th>
                        <th>Status</th>
                        <th>CPU</th>
                        <th>Memory</th>
                        <th title="Cumulative received / sent bytes">
                          Net I/O <span>↓ / ↑</span>
                        </th>
                        <th title="Cumulative block read / write bytes">
                          Block I/O <span>R / W</span>
                        </th>
                        <th>PIDs</th>
                        <th>
                          <span className="sr-only">Details</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {sorted.map((c) => {
                        const current = isLive(c, data, now, error),
                          s: Sample | null = current ? c.latest : null;
                        return (
                          <tr
                            key={c.id}
                            className={
                              (selected === c.id ? 'selected-row ' : '') +
                              (!c.present ? 'removed-row' : '')
                            }
                            onClick={(e) => {
                              selectedTrigger.current = e.currentTarget.querySelector('button');
                              setInvestigation(null);
                              setSelected(c.id);
                            }}
                          >
                            <td>
                              <div className="container-cell">
                                <div className={'container-glyph ' + (current ? 'active' : '')}>
                                  <Box size={16} />
                                </div>
                                <div>
                                  <button
                                    className="container-link"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      selectedTrigger.current = e.currentTarget;
                                      setInvestigation(null);
                                      setSelected(c.id);
                                    }}
                                    aria-label={'Inspect ' + c.name}
                                  >
                                    {c.name}
                                  </button>
                                  <span className="container-image" title={c.image}>
                                    {c.image}
                                  </span>
                                  <span className="container-id">
                                    {c.id.slice(0, 12)} � {c.project || 'Standalone'}
                                    {c.service ? ' / ' + c.service : ''}
                                  </span>
                                </div>
                              </div>
                            </td>
                            <td>
                              <Badge value={status(c, fresh)} />
                              {!current && c.latest && (
                                <span className="stale-label">{age(c.latest.at, now)} · stale</span>
                              )}
                            </td>
                            <td>
                              <span className={s?.cpu_percent != null ? 'metric-value' : ''}>
                                {pct(s?.cpu_percent)}
                              </span>
                              <div className="meter">
                                <i style={{ width: Math.min(100, s?.cpu_percent ?? 0) + '%' }} />
                              </div>
                            </td>
                            <td className="mono">{bytes(s?.memory_bytes)}</td>
                            <td>
                              <IOPair first={s?.network_rx_bytes} second={s?.network_tx_bytes} />
                            </td>
                            <td>
                              <IOPair
                                first={s?.block_read_bytes}
                                second={s?.block_write_bytes}
                                labels={['R', 'W']}
                              />
                            </td>
                            <td className="mono">{numeric(s?.pids, 0)}</td>
                            <td>
                              <ChevronRight size={14} className="row-arrow" />
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
                {!sorted.length && (
                  <div className="empty-state">
                    <Boxes size={29} />
                    <h3>
                      {query
                        ? 'No matching containers'
                        : error
                          ? 'Unable to load containers'
                          : busy
                            ? 'Discovering your containers…'
                            : 'No containers here yet'}
                    </h3>
                    <p>
                      {query
                        ? 'Try a name, image, or container ID.'
                        : error
                          ? 'Your last-known data is stale. Use Refresh to retry.'
                          : 'Start a container on this Docker Engine to see its resource usage.'}
                    </p>
                  </div>
                )}
                <div className="list-footer">
                  <span>
                    {sorted.length} {sorted.length === 1 ? 'container' : 'containers'}
                    {query ? ' matching your search' : ''}
                  </span>
                  <span>
                    <Clock3 size={12} />
                    7-day history<span className="footer-separator">·</span>Auto-refresh on
                  </span>
                </div>
              </section>
            )}
            <div className="workspace-note">
              <Layers3 size={14} />
              <span>Running is a container state, not an application health check.</span>
            </div>
            <footer className="workspace-footer">
              <span>
                <Activity size={13} />
                OBSERVATORY
              </span>
              <span>Self-hosted. In your hands.</span>
            </footer>
          </main>
          {selectedContainer && (
            <DetailsPanel
              key={selectedContainer.id + investigationVersion}
              container={selectedContainer}
              data={data}
              error={error}
              now={now}
              onClose={close}
              width={panelWidth}
              onWidth={setPanelWidth}
              refresh={refreshVersion}
              investigation={investigation}
              onUnpin={() => setInvestigation(null)}
              onInvestigate={investigate}
            />
          )}
        </div>
      </div>
      {help && <AboutModal onClose={() => setHelp(false)} />}
    </div>
  );
}

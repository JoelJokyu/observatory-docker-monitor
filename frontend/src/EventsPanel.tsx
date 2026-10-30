import { useEffect, useState } from 'react';
import { Activity, AlertTriangle, ArrowUpRight, Clock3, RefreshCw } from 'lucide-react';
import { fetchJSON } from './format';
import { eventRange, recordedContainer, type OpenInvestigation } from './investigation';

export interface LifecycleEvent {
  id: string;
  container_id: string;
  name: string;
  project: string;
  service: string;
  action: string;
  at: string;
  exit_code?: string;
  signal?: string;
}
interface Timeline {
  events: LifecycleEvent[];
  from: string;
  to: string;
  has_more: boolean;
  collector: { connected: boolean; started_at: string; interrupted: boolean; message: string };
  coverage_notice: string;
}
export function eventLabel(action: string): string {
  return (
    (
      {
        create: 'Created',
        start: 'Started',
        stop: 'Stopped',
        restart: 'Restarted',
        die: 'Exited',
        destroy: 'Removed',
        kill: 'Signal sent',
        oom: 'Out of memory',
        pause: 'Paused',
        unpause: 'Resumed',
        rename: 'Renamed',
        'health_status: healthy': 'Healthy',
        'health_status: unhealthy': 'Unhealthy',
        'health_status: starting': 'Health check starting',
      } as Record<string, string>
    )[action] ?? action
  );
}
export function EventRows({
  events,
  onInspect,
  inspectableIDs,
  onInvestigate,
}: {
  events: LifecycleEvent[];
  onInspect?: (id: string, trigger: HTMLButtonElement) => void;
  inspectableIDs?: Set<string>;
  onInvestigate?: OpenInvestigation;
}) {
  return (
    <ol className="event-list">
      {events.map((e) => {
        const warning =
          e.action === 'oom' ||
          e.action === 'health_status: unhealthy' ||
          (e.action === 'die' && e.exit_code != null && e.exit_code !== '0');
        return (
          <li key={e.id} className={warning ? 'event-warning' : ''}>
            <span className="event-marker">
              {warning ? <AlertTriangle size={15} /> : <Activity size={15} />}
            </span>
            <div className="event-body">
              <div className="event-title">
                <strong>{eventLabel(e.action)}</strong>
                <time dateTime={e.at} title={new Date(e.at).toLocaleString()}>
                  {new Date(e.at).toLocaleTimeString([], {
                    hour12: false,
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                  })}
                </time>
              </div>
              <div className="event-target">
                <span>{e.name || e.container_id.slice(0, 12)}</span>
                {onInspect && (!inspectableIDs || inspectableIDs.has(e.container_id)) && (
                  <button
                    className="event-inspect"
                    aria-label={
                      'Inspect event container ' + (e.name || e.container_id.slice(0, 12))
                    }
                    onClick={(click) => onInspect(e.container_id, click.currentTarget)}
                  >
                    <ArrowUpRight size={14} />
                  </button>
                )}
              </div>
              {onInvestigate && (
                <div className="investigation-actions">
                  {(['resources', 'logs'] as const).map((tab) => (
                    <button
                      key={tab}
                      className="refresh-button"
                      disabled={!eventRange(e.at)}
                      onClick={(click) => {
                        const range = eventRange(e.at);
                        if (range)
                          onInvestigate(
                            {
                              container: recordedContainer(
                                e.container_id,
                                e.name,
                                e.project,
                                e.service,
                              ),
                              range,
                              tab,
                              label: 'Event context · up to 2 minutes either side',
                            },
                            click.currentTarget,
                          );
                      }}
                    >
                      {tab === 'logs' ? 'View logs' : 'View resources'}
                    </button>
                  ))}
                </div>
              )}
              <div className="event-meta">
                <span>
                  {e.project || 'Standalone'}
                  {e.service ? ' / ' + e.service : ''}
                </span>
                <code>{e.container_id.slice(0, 12)}</code>
                <span>{new Date(e.at).toLocaleDateString()}</span>
                {e.exit_code != null && <span>Exit code {e.exit_code}</span>}
                {e.signal && <span>Signal {e.signal}</span>}
              </div>
            </div>
          </li>
        );
      })}
    </ol>
  );
}
export default function EventsPanel({
  project = '',
  containerID = '',
  refresh = 0,
  onInspect,
  inspectableIDs,
  onInvestigate,
}: {
  project?: string;
  containerID?: string;
  refresh?: number;
  onInspect?: (id: string, trigger: HTMLButtonElement) => void;
  inspectableIDs?: Set<string>;
  onInvestigate?: OpenInvestigation;
}) {
  const [seconds, setSeconds] = useState(3600),
    [version, setVersion] = useState(0),
    [data, setData] = useState<Timeline | null>(null),
    [error, setError] = useState<string | null>(null),
    [busy, setBusy] = useState(true);
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    async function load() {
      setBusy(true);
      const to = new Date();
      const q = new URLSearchParams({
        from: new Date(to.getTime() - seconds * 1000).toISOString(),
        to: to.toISOString(),
      });
      if (project === 'standalone') q.set('standalone', 'true');
      else if (project.startsWith('project:')) q.set('project', project.slice(8));
      if (containerID) q.set('container_id', containerID);
      const timeout = setTimeout(() => controller.abort(), 8000);
      try {
        const value = await fetchJSON<Timeline>('/api/events?' + q, controller.signal);
        if (active) {
          setData(value);
          setError(null);
        }
      } catch (e) {
        if (active)
          setError(
            e instanceof Error && e.name !== 'AbortError'
              ? e.message
              : 'Event request timed out. Refresh to retry.',
          );
      } finally {
        clearTimeout(timeout);
        if (active) setBusy(false);
      }
    }
    void load();
    return () => {
      active = false;
      controller.abort();
    };
  }, [project, containerID, seconds, version, refresh]);
  // Never show the previous project's/container's rows while a scope change loads.
  useEffect(() => {
    setData(null);
    setError(null);
  }, [project, containerID, seconds]);
  return (
    <section className="events-panel" aria-label="Container event timeline">
      <div className="events-toolbar">
        <div>
          <div className="section-label">EVENT TIMELINE</div>
          <h3>What happened</h3>
        </div>
        <div className="events-controls">
          <select
            aria-label="Event time window"
            value={seconds}
            onChange={(e) => setSeconds(Number(e.target.value))}
          >
            {[
              { v: 900, l: '15 min' },
              { v: 3600, l: '1 hour' },
              { v: 86400, l: '24 hours' },
              { v: 604800, l: '7 days' },
            ].map((w) => (
              <option key={w.v} value={w.v}>
                {w.l}
              </option>
            ))}
          </select>
          <button
            className="refresh-button"
            aria-label="Refresh events"
            disabled={busy}
            onClick={() => setVersion((v) => v + 1)}
          >
            <RefreshCw size={14} className={busy ? 'spinning' : ''} />
          </button>
        </div>
      </div>
      {error ? (
        <div className="detail-notice" role="alert">
          {error} Previously loaded events may be stale.
        </div>
      ) : (
        data && (
          <div
            className={'event-connection ' + (!data.collector.connected ? 'event-offline' : '')}
            role="status"
          >
            <span className={data.collector.connected ? 'live-dot' : 'muted-dot'} />
            {data.collector.message}
            {data.collector.interrupted && ' · Collection was interrupted; gaps are possible.'}
          </div>
        )
      )}
      {data && (
        <p className="event-coverage">
          <Clock3 size={13} />
          <span>{data.coverage_notice}</span>
        </p>
      )}
      {busy && !data ? (
        <p className="event-empty">Loading recorded events…</p>
      ) : data?.events.length ? (
        <EventRows
          events={data.events}
          onInspect={onInspect}
          inspectableIDs={inspectableIDs}
          onInvestigate={onInvestigate}
        />
      ) : (
        !error && (
          <div className="event-empty">
            <Activity size={26} />
            <h4>No recorded events in this window</h4>
            <p>
              Start, stop, or restart a test container to see its lifecycle here. An empty timeline
              does not prove that nothing happened.
            </p>
          </div>
        )
      )}
      {data && (
        <div className="events-footer">
          <span>{data.events.length} recorded events · newest first</span>
          {data.has_more && (
            <strong>Showing latest 200. Choose a shorter window to narrow results.</strong>
          )}
          <span>
            Window: {new Date(data.from).toLocaleString()} – {new Date(data.to).toLocaleString()}
          </span>
        </div>
      )}
    </section>
  );
}

import { useCallback, useEffect, useRef, useState } from 'react';
import { Clock3, FileText, RefreshCw } from 'lucide-react';
import type { Container } from './types';
import type { LogEntry, LogRange, LogSnapshot } from './logs';
import { copyLogText, filterLogEntries, fetchSnapshot, snapshotRange } from './logs';
import { age, bytes } from './format';

export function LogRows({ entries }: { entries: LogEntry[] }) {
  return (
    <>
      {entries.map((entry, index) => (
        <tr key={index}>
          <td className="log-line-number">{index + 1}</td>
          <td className="log-time">
            <time
              title={entry.timestamp ?? 'Docker timestamp unavailable'}
              dateTime={entry.timestamp ?? undefined}
            >
              {entry.timestamp
                ? new Date(entry.timestamp).toLocaleTimeString([], {
                    hour12: false,
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                    fractionalSecondDigits: 3,
                  })
                : '—'}
            </time>
          </td>
          <td className="log-stream">
            <span className={'stream-label ' + entry.stream}>{entry.stream}</span>
          </td>
          <td className="log-message">
            <pre>{entry.message}</pre>
            {entry.partial && (
              <span className="partial-line">Partial line · output limit reached</span>
            )}
          </td>
        </tr>
      ))}
    </>
  );
}
export default function LogsPanel({
  container,
  windowSeconds,
  onWindow,
  windows,
  resourceRange,
  now,
  fixedRange,
}: {
  container: Container;
  windowSeconds: number;
  onWindow: (value: number) => void;
  windows: { value: number; label: string }[];
  resourceRange: LogRange | null;
  now: number;
  fixedRange?: LogRange;
}) {
  const logAvailable = container.present || !!container.recorded_only;
  const [snapshot, setSnapshot] = useState<LogSnapshot | null>(null),
    [range, setRange] = useState<LogRange | null>(null),
    [loading, setLoading] = useState(false),
    [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState(''),
    [stream, setStream] = useState('all'),
    [copyStatus, setCopyStatus] = useState('');
  const filtered = filterLogEntries(snapshot?.entries ?? [], query, stream);
  const controller = useRef<AbortController | null>(null),
    latestRange = useRef(resourceRange);
  latestRange.current = resourceRange;
  const load = useCallback(async () => {
    controller.current?.abort();
    const request = new AbortController();
    controller.current = request;
    if (!logAvailable) {
      setSnapshot(null);
      setError(null);
      setLoading(false);
      return;
    }
    const selected = fixedRange ?? snapshotRange(windowSeconds, latestRange.current);
    setCopyStatus('');
    setRange(selected);
    setLoading(true);
    setError(null);
    setSnapshot(null);
    const timeout = setTimeout(() => request.abort(), 10000);
    try {
      const data = await fetchSnapshot(container.id, selected, request.signal);
      if (controller.current === request && !request.signal.aborted) setSnapshot(data);
    } catch (reason) {
      if (controller.current === request)
        setError(
          reason instanceof Error && reason.name !== 'AbortError'
            ? reason.message
            : 'The log snapshot timed out. Try Refresh or a shorter window.',
        );
    } finally {
      clearTimeout(timeout);
      if (controller.current === request) {
        setLoading(false);
        controller.current = null;
      }
    }
  }, [container.id, container.present, container.recorded_only, windowSeconds, fixedRange]);
  useEffect(() => {
    void load();
    return () => {
      controller.current?.abort();
      controller.current = null;
    };
  }, [load]);
  return (
    <section className="logs-panel" aria-label="Container log snapshot">
      <div className="logs-intro">
        <FileText size={17} />
        <div>
          <h3>Container logs</h3>
          <p>A snapshot from Docker. Refresh when you want a new reading.</p>
        </div>
      </div>
      <div className="logs-toolbar">
        <label className="sr-only" htmlFor="logs-window">
          Log time window
        </label>
        <select
          id="logs-window"
          disabled={!!fixedRange}
          value={windowSeconds}
          onChange={(e) => onWindow(Number(e.target.value))}
        >
          {windows.map((w) => (
            <option key={w.value} value={w.value}>
              {fixedRange ? 'Fixed investigation window' : 'Last ' + w.label}
            </option>
          ))}
        </select>
        <button
          className="refresh-button"
          disabled={loading || !logAvailable}
          onClick={() => void load()}
        >
          <RefreshCw size={13} className={loading ? 'spinning' : ''} />
          {loading ? 'Loading…' : 'Refresh logs'}
        </button>
      </div>
      {range && logAvailable && (
        <p className="logs-range">
          <Clock3 size={12} />
          <span>
            {new Date(range.from).toLocaleString()}
            <br />
            to {new Date(range.to).toLocaleString()}
          </span>
        </p>
      )}
      <div className="logs-disclosure">
        Docker-retained output only · latest up to 500 lines · 512 KiB text cap. Availability may
        differ from resource history. stderr is a stream, not a severity.
      </div>
      {snapshot && (
        <div className="log-filter-tools">
          <input
            aria-label="Search loaded log lines"
            placeholder="Search loaded messages…"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setCopyStatus('');
            }}
          />
          <select
            aria-label="Filter loaded log stream"
            value={stream}
            onChange={(e) => {
              setStream(e.target.value);
              setCopyStatus('');
            }}
          >
            <option value="all">All streams</option>
            <option value="stdout">stdout</option>
            <option value="stderr">stderr</option>
            <option value="combined">Combined (TTY)</option>
          </select>
          <button
            className="refresh-button"
            disabled={!filtered.length}
            onClick={async () => {
              try {
                await navigator.clipboard.writeText(copyLogText(filtered));
                setCopyStatus('Copied matching lines');
              } catch {
                setCopyStatus('Copy unavailable. Select and copy the text manually.');
              }
            }}
          >
            Copy matching lines
          </button>
          <p role="status">
            {filtered.length} of {snapshot.entries.length} loaded lines · Search covers this
            snapshot only. {copyStatus}
          </p>
        </div>
      )}
      {snapshot?.tty && (
        <p className="logs-notice">
          TTY output is combined. Docker cannot separate stdout and stderr for this container.
        </p>
      )}
      {snapshot?.truncated && (
        <p className="logs-notice" role="status">
          Output was truncated (
          {snapshot.truncation_reason?.replaceAll('_', ' ') ?? 'response limit'}). A partial
          snapshot may omit newer lines. Narrow the time window to inspect more detail.
        </p>
      )}
      {snapshot?.limit_reached && !snapshot.truncated && (
        <p className="logs-notice" role="status">
          The 500-line tail limit was reached. Earlier logs may exist in this window.
        </p>
      )}
      {!logAvailable ? (
        <div className="log-empty">
          <FileText size={25} />
          <h4>Container removed</h4>
          <p>
            Docker no longer has this container. Its resource history is preserved, but Observatory
            does not archive logs.
          </p>
        </div>
      ) : error ? (
        <div className="log-empty error" role="alert">
          <FileText size={25} />
          <h4>Logs unavailable</h4>
          <p>{error}</p>
        </div>
      ) : loading ? (
        <div className="log-empty" role="status">
          <RefreshCw size={23} className="spinning" />
          <p>Reading a bounded snapshot from Docker…</p>
        </div>
      ) : snapshot && !snapshot.entries.length ? (
        <div className="log-empty">
          <FileText size={25} />
          <h4>No output returned</h4>
          <p>
            Docker returned no log lines for this request. Output may have rotated, or the container
            may not have written any in this window.
          </p>
        </div>
      ) : snapshot ? (
        <div
          className="logs-output"
          tabIndex={0}
          aria-label="Log output; timestamps use local time"
        >
          <table className="log-table">
            <caption className="sr-only">
              Docker log snapshot. Stream labels do not indicate severity.
            </caption>
            <thead>
              <tr>
                <th>Line</th>
                <th>Time</th>
                <th>Stream</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              <LogRows entries={filtered} />
            </tbody>
          </table>
        </div>
      ) : null}
      {snapshot && snapshot.entries.length > 0 && !filtered.length && (
        <p className="logs-notice" role="status">
          No loaded lines match these filters. Change the search or stream; this does not search
          Docker’s full log history.
        </p>
      )}
      <div className="logs-snapshot-status" role="status">
        <span>
          {snapshot
            ? snapshot.entries.length + ' lines · ' + bytes(snapshot.text_bytes)
            : 'Snapshot mode'}
        </span>
        <span>{snapshot ? 'Fetched ' + age(snapshot.fetched_at, now) : 'No live following'}</span>
      </div>
    </section>
  );
}

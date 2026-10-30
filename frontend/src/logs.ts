import type { History } from './types';
export interface LogEntry {
  timestamp: string | null;
  stream: 'stdout' | 'stderr' | 'combined';
  message: string;
  partial: boolean;
}
export interface LogSnapshot {
  container_id: string;
  from: string;
  to: string;
  fetched_at: string;
  entries: LogEntry[];
  limit: number;
  limit_reached: boolean;
  truncated: boolean;
  truncation_reason?: string;
  tty: boolean;
  text_bytes: number;
  max_text_bytes: number;
  source: 'docker';
  logging_driver: string;
}
export type LogRange = Pick<History, 'from' | 'to'>;
export function snapshotRange(
  seconds: number,
  resourceRange: LogRange | null,
  now = Date.now(),
): LogRange {
  if (resourceRange) {
    const from = Date.parse(resourceRange.from),
      to = Date.parse(resourceRange.to);
    if (
      Number.isFinite(from) &&
      Number.isFinite(to) &&
      Math.abs(to - from - seconds * 1000) < 1000 &&
      from >= now - 7 * 86400000 - 60000 &&
      to <= now + 60000
    ) {
      return resourceRange;
    }
  }
  return { from: new Date(now - seconds * 1000).toISOString(), to: new Date(now).toISOString() };
}
export async function fetchSnapshot(
  id: string,
  range: LogRange,
  signal: AbortSignal,
): Promise<LogSnapshot> {
  const query = new URLSearchParams({ ...range, limit: '500' });
  const response = await fetch('/api/containers/' + encodeURIComponent(id) + '/logs?' + query, {
    signal,
    cache: 'no-store',
  });
  let body;
  try {
    body = await response.json();
  } catch {
    throw new Error(
      'The logs endpoint is unavailable. Check that Observatory has been rebuilt with log snapshots.',
    );
  }
  if (!response.ok) {
    throw new Error(body.error ?? 'Docker log access is temporarily unavailable.');
  }
  return body as LogSnapshot;
}

export function filterLogEntries(entries: LogEntry[], query: string, stream: string): LogEntry[] {
  const needle = query.toLocaleLowerCase();
  return entries.filter(
    (e) =>
      (stream === 'all' || e.stream === stream) && e.message.toLocaleLowerCase().includes(needle),
  );
}
export function copyLogText(entries: LogEntry[]): string {
  return entries
    .map(
      (e) =>
        `${e.timestamp ?? 'unknown time'} [${e.stream}] ${e.message}${e.partial ? ' [partial line]' : ''}`,
    )
    .join('\n');
}

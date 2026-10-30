import type { Container, Inventory } from './types';
export const numeric = (n: number | null | undefined, digits = 1) =>
  n == null || !Number.isFinite(n)
    ? '—'
    : n.toLocaleString(undefined, { maximumFractionDigits: digits });
export function bytes(n: number | null | undefined): string {
  if (n == null || !Number.isFinite(n)) return '—';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let value = Math.max(0, n),
    i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return numeric(value, i === 0 ? 0 : 1) + ' ' + units[i];
}
export const age = (at: string | null | undefined, now = Date.now()) => {
  if (!at) return 'No samples';
  const s = Math.max(0, Math.floor((now - Date.parse(at)) / 1000));
  return s < 60
    ? s + 's ago'
    : s < 3600
      ? Math.floor(s / 60) + 'm ago'
      : s < 86400
        ? Math.floor(s / 3600) + 'h ago'
        : Math.floor(s / 86400) + 'd ago';
};
export const inventoryFresh = (data: Inventory | null, now: number) =>
  !!data?.collector.last_inventory &&
  now - Date.parse(data.collector.last_inventory) <=
    Math.max(3 * data.sample_interval_seconds, 15) * 1000;
export const sampleFresh = (c: Container, interval: number, now: number) =>
  !!c.latest && now - Date.parse(c.latest.at) <= interval * 3000;
export const isLive = (c: Container, data: Inventory | null, now: number, error: string | null) =>
  !error &&
  inventoryFresh(data, now) &&
  c.present &&
  c.state === 'running' &&
  sampleFresh(c, data?.sample_interval_seconds ?? 10, now);
export function status(c: Container, fresh: boolean): string {
  if (c.recorded_only) return 'Recorded';
  return !fresh
    ? 'Unknown'
    : !c.present
      ? 'Removed'
      : c.state === 'running'
        ? 'Running'
        : c.state === 'exited'
          ? 'Exited'
          : c.state[0]?.toUpperCase() + c.state.slice(1);
}
export async function fetchJSON<T>(url: string, signal: AbortSignal): Promise<T> {
  const response = await fetch(url, { signal, cache: 'no-store' });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || 'The platform is temporarily unavailable.');
  return data as T;
}

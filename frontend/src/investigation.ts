import type { Container } from './types';
import type { LogRange } from './logs';
export interface Investigation {
  container: Container;
  range: LogRange;
  tab: 'resources' | 'logs';
  label: string;
}
export type OpenInvestigation = (value: Investigation, trigger: HTMLButtonElement) => void;
export function eventRange(at: string, now = Date.now()): LogRange | null {
  const time = Date.parse(at);
  if (!Number.isFinite(time) || time < now - 7 * 86400000 || time > now) return null;
  return {
    from: new Date(Math.max(time - 120000, now - 7 * 86400000)).toISOString(),
    to: new Date(Math.min(time + 120000, now)).toISOString(),
  };
}
export function recordedContainer(
  id: string,
  name: string,
  project: string,
  service: string,
): Container {
  return {
    id,
    name: name || id.slice(0, 12),
    project,
    service,
    recorded_only: true,
    image: 'Recorded identity',
    state: 'unknown',
    present: false,
    seen_at: new Date(0).toISOString(),
    latest: null,
  };
}

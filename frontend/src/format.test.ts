import { describe, expect, it } from 'vitest';
import { bytes, isLive } from './format';
import { paths } from './chart';
import type { Bucket, Container, Inventory } from './types';
describe('measurement presentation', () => {
  it('distinguishes unknown from zero', () => {
    expect(bytes(null)).toBe('—');
    expect(bytes(0)).toBe('0 B');
    expect(bytes(1048576)).toBe('1 MiB');
  });
  it('does not show a stale reading as live', () => {
    const now = Date.now(),
      c = {
        present: true,
        state: 'running',
        latest: { at: new Date(now - 40000).toISOString() },
      } as Container;
    const data = {
      sample_interval_seconds: 10,
      collector: { last_inventory: new Date(now).toISOString() },
    } as Inventory;
    expect(isLive(c, data, now, null)).toBe(false);
    c.latest!.at = new Date(now).toISOString();
    expect(isLive(c, data, now, null)).toBe(true);
    expect(isLive(c, data, now, 'offline')).toBe(false);
  });
  it('breaks chart paths across missing data and collection gaps', () => {
    const point = (seconds: number, cpu: number | null) =>
      ({
        at: new Date(seconds * 1000).toISOString(),
        first: new Date(seconds * 1000).toISOString(),
        last: new Date(seconds * 1000).toISOString(),
        cpu_percent: cpu,
      }) as Bucket;
    const data = [point(0, 1), point(10, 2), point(20, null), point(30, 3), point(100, 4)];
    const result = paths(
      data,
      'cpu_percent',
      10,
      10,
      (at) => Date.parse(at) / 1000,
      (v) => v,
    );
    expect(result).toEqual(['M0.00,1.00 L10.00,2.00', 'M30.00,3.00', 'M100.00,4.00']);
  });
});

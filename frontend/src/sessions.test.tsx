import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { duration, SessionResults, type SessionSummary } from './SessionsPanel';
const summary: SessionSummary = {
  session: {
    id: 'id',
    name: 'test',
    project: 'chat',
    standalone: false,
    started_at: '2026-10-26T12:00:00Z',
    finished_at: '2026-10-26T12:01:10Z',
    sample_interval_seconds: 10,
  },
  as_of: '2026-10-26T12:02:00Z',
  available_from: '2026-10-26T12:00:00Z',
  to: '2026-10-26T12:01:10Z',
  history_expired: false,
  container_count: 1,
  samples: 2,
  event_count: 1,
  nonzero_exits: 1,
  containers: [
    {
      container_id: 'a'.repeat(64),
      name: '<script>unsafe</script>',
      service: 'api',
      first_observed_at: '2026-10-26T12:00:00Z',
      samples: 2,
      first_sample: '2026-10-26T12:00:10Z',
      last_sample: '2026-10-26T12:00:20Z',
      cpu_observations: 1,
      cpu_mean: 0,
      cpu_peak: 0,
      memory_peak_bytes: 0,
    },
  ],
  containers_limited: false,
  actions: [{ action: 'die', count: 1 }],
};
describe('session evidence', () => {
  it('formats elapsed wall time across a restart-sized gap', () => {
    expect(duration('2026-10-26T12:00:00Z', '2026-10-26T12:01:10Z')).toBe('1m 10s');
    expect(duration('2026-10-25T12:00:00Z', '2026-10-26T13:10:00Z')).toBe('1d 1h');
  });
  it('escapes names, preserves explicit zero and explains sample-based statistics', () => {
    const html = renderToStaticMarkup(
      <SessionResults summary={summary} notice="Recorded observations only" />,
    );
    expect(html).toContain('&lt;script&gt;');
    expect(html).not.toContain('<script>');
    expect(html).toContain('0%');
    expect(html).toContain('0 B');
    expect(html).toContain('without time weighting');
    expect(html).toContain('1 nonzero');
    expect(html).toContain('Exited');
  });
  it('discloses expired history and leaves unknown values unknown', () => {
    const value = {
      ...summary,
      history_expired: true,
      containers: [
        { ...summary.containers[0], cpu_mean: null, cpu_peak: null, memory_peak_bytes: null },
      ],
    };
    const html = renderToStaticMarkup(
      <SessionResults summary={value} notice="Coverage can have gaps" />,
    );
    expect(html).toContain('Those measurements and events are unavailable');
    expect(html).toContain('—');
    expect(html).not.toContain('0 B');
    expect(html).toContain('Coverage can have gaps');
  });
});

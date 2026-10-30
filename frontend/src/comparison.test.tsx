import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { ComparisonResults, comparisonRows, sameScope } from './SessionComparison';
import type { Detail, SessionMember } from './SessionsPanel';
const member: SessionMember = {
  container_id: 'a'.repeat(64),
  name: '<script>api</script>',
  service: 'api',
  first_observed_at: '2026-10-30T12:00:00Z',
  samples: 2,
  first_sample: '2026-10-30T12:00:10Z',
  last_sample: '2026-10-30T12:00:20Z',
  cpu_observations: 1,
  cpu_mean: 0,
  cpu_peak: 0,
  memory_peak_bytes: 0,
};
function detail(id: string): Detail {
  return {
    summary: {
      session: {
        id,
        name: id,
        project: 'chat',
        standalone: false,
        started_at: '2026-10-30T12:00:00Z',
        finished_at: '2026-10-30T12:01:00Z',
        sample_interval_seconds: 10,
      },
      as_of: '2026-10-30T12:02:00Z',
      available_from: '2026-10-30T12:00:00Z',
      to: '2026-10-30T12:01:00Z',
      history_expired: false,
      container_count: 1,
      samples: 2,
      event_count: 0,
      nonzero_exits: 0,
      containers: [{ ...member }],
      containers_limited: false,
      actions: [],
    },
    coverage_notice: 'Recorded observations only; gaps possible.',
    collector: { healthy: true, message: '', last_inventory: null },
  };
}
describe('session comparison evidence', () => {
  it('matches only full IDs, keeping same-name replacements separate', () => {
    const a = detail('A').summary,
      b = detail('B').summary;
    b.containers = [{ ...member, container_id: 'b'.repeat(64) }];
    const rows = comparisonRows(a, b);
    expect(rows).toHaveLength(2);
    expect(rows[0].b).toBeUndefined();
    expect(rows[1].a).toBeUndefined();
    b.containers = [{ ...member }];
    expect(comparisonRows(a, b)).toHaveLength(1);
  });
  it('rejects active, identical and cross-scope comparisons', () => {
    const a = detail('A'),
      b = detail('B');
    expect(sameScope(a.summary.session, b.summary.session)).toBe(true);
    for (const invalid of [
      { ...b.summary.session, finished_at: null },
      { ...b.summary.session, id: 'A' },
      { ...b.summary.session, project: 'other' },
      { ...b.summary.session, standalone: true },
    ]) {
      const html = renderToStaticMarkup(
        <ComparisonResults
          left={a}
          right={{ ...b, summary: { ...b.summary, session: invalid } }}
        />,
      );
      expect(html).toContain('role="alert"');
      expect(html).not.toContain('Resources by container');
    }
  });
  it('preserves zero vs unknown, escapes names and discloses definitions', () => {
    const a = detail('A'),
      b = detail('B');
    b.summary.containers = [{ ...member, cpu_mean: null, cpu_peak: null, memory_peak_bytes: null }];
    const html = renderToStaticMarkup(<ComparisonResults left={a} right={b} />);
    expect(html).toContain('0%');
    expect(html).toContain('0 B');
    expect(html).toContain('—');
    expect(html).toContain('&lt;script&gt;');
    expect(html).not.toContain('<script>');
    expect(html).toContain('without time weighting');
    expect(html).toContain('not a performance score');
    expect(html).toContain('gaps possible');
  });
  it('discloses incomplete member lists, retention loss and unequal raw counts', () => {
    const a = detail('A'),
      b = detail('B');
    b.summary.history_expired = true;
    b.summary.containers_limited = true;
    b.summary.containers = [];
    b.summary.actions = [{ action: 'die', count: 3 }];
    const html = renderToStaticMarkup(<ComparisonResults left={a} right={b} />);
    expect(html).toContain('Retained portion only');
    expect(html).toContain('Absent from returned members');
    expect(html).toContain('limited to 500');
    expect(html).toContain('without duration normalization');
    expect(html).toContain('>3</td>');
    expect(html).toContain('>0</td>');
  });
  it('keeps each run’s own investigation actions and disables empty ranges', () => {
    const a = detail('A'),
      b = detail('B');
    b.summary.available_from = b.summary.to;
    const html = renderToStaticMarkup(
      <ComparisonResults left={a} right={b} onInvestigate={() => {}} />,
    );
    expect(html.match(/View resources/g)).toHaveLength(2);
    expect(html.match(/disabled=""/g)).toHaveLength(2);
  });
});

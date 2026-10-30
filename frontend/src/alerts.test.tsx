import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { AlertCard, alertWindow, type LocalAlert } from './AlertsPanel';
const alert: LocalAlert = {
  id: 'a',
  project: 'chat',
  container_id: 'c'.repeat(64),
  name: '<script>plain</script>',
  service: 'api',
  kind: 'nonzero_exit',
  evidence_from: '2026-10-28T12:00:00Z',
  at: '2026-10-28T12:00:00Z',
  last_observed_at: '2026-10-28T12:00:00Z',
  value: null,
  threshold: null,
  detail: 'Intentional stops can also produce nonzero exits.',
  resolved_at: null,
  resolution: '',
  dismissed_at: null,
};
describe('alert evidence', () => {
  it('renders event evidence factually and safely', () => {
    const html = renderToStaticMarkup(
      <AlertCard
        alert={alert}
        interval={10}
        now={Date.parse('2026-10-28T12:01:00Z')}
        onDismiss={() => {}}
        onInvestigate={() => {}}
        busy={false}
      />,
    );
    expect(html).toContain('Nonzero exit');
    expect(html).toContain('Recorded event');
    expect(html).toContain('&lt;script&gt;');
    expect(html).not.toContain('<script>');
    expect(html).toContain('Intentional stops');
    expect(html).toContain('View logs');
  });
  it('keeps stale evidence separate from recovery and dismissal', () => {
    const html = renderToStaticMarkup(
      <AlertCard
        alert={{ ...alert, kind: 'cpu', value: 90, threshold: 0.01, dismissed_at: alert.at }}
        interval={10}
        now={Date.parse('2026-10-28T12:02:00Z')}
        onDismiss={() => {}}
        onInvestigate={() => {}}
        busy={false}
      />,
    );
    expect(html).toContain('Evidence is stale or missing');
    expect(html).toContain('Dismissed');
    expect(html).toContain('0.01');
    expect(html).not.toContain('>Dismiss</button>');
    expect(html).not.toContain('>Recovered');
  });
  it('bounds investigation context and distinguishes rule changes', () => {
    expect(alertWindow(alert, Date.parse('2026-10-28T12:01:00Z'))).toEqual({
      from: '2026-10-28T11:58:00.000Z',
      to: '2026-10-28T12:01:00.000Z',
    });
    const html = renderToStaticMarkup(
      <AlertCard
        alert={{ ...alert, kind: 'memory', resolved_at: alert.at, resolution: 'rule_changed' }}
        interval={10}
        now={Date.parse(alert.at)}
        onDismiss={() => {}}
        onInvestigate={() => {}}
        busy={false}
      />,
    );
    expect(html).toContain('Rule changed');
    expect(html).not.toContain('>Recovered');
  });
});

import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { eventLabel, EventRows, type LifecycleEvent } from './EventsPanel';
describe('event presentation', () => {
  it('uses factual labels without assuming a restart or cause', () => {
    expect(eventLabel('die')).toBe('Exited');
    expect(eventLabel('kill')).toBe('Signal sent');
    expect(eventLabel('oom')).toBe('Out of memory');
  });
  it('escapes names and preserves identity, project and exit evidence', () => {
    const e: LifecycleEvent = {
      id: 'event',
      container_id: 'a'.repeat(64),
      name: '<script>alert(1)</script>',
      project: 'chat',
      service: 'api',
      action: 'die',
      at: '2026-10-26T12:30:40Z',
      exit_code: '137',
    };
    const html = renderToStaticMarkup(<EventRows events={[e]} />);
    expect(html).toContain('&lt;script&gt;');
    expect(html).not.toContain('<script>');
    expect(html).toContain('Exit code 137');
    expect(html).toContain('chat / api');
    expect(html).toContain('aaaaaaaaaaaa');
    expect(html).toContain('event-warning');
  });
  it('only offers inventory navigation for known container IDs', () => {
    const e: LifecycleEvent = {
      id: 'e',
      container_id: 'missing',
      name: 'gone',
      project: '',
      service: '',
      action: 'destroy',
      at: '2026-10-26T12:30:40Z',
    };
    const html = renderToStaticMarkup(
      <EventRows events={[e]} onInspect={() => {}} inspectableIDs={new Set()} />,
    );
    expect(html).not.toContain('<button');
    expect(html).toContain('Standalone');
    expect(html).toContain('Removed');
  });
});

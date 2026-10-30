import { describe, expect, it } from 'vitest';
import { eventRange } from './investigation';
import { copyLogText, filterLogEntries, type LogEntry } from './logs';
describe('investigation boundaries and snapshot tools', () => {
  const now = Date.parse('2026-10-28T12:00:00Z');
  it('bounds event context to now and retention, rejecting unavailable events', () => {
    expect(eventRange('2026-10-28T11:59:30Z', now)).toEqual({
      from: '2026-10-28T11:57:30.000Z',
      to: '2026-10-28T12:00:00.000Z',
    });
    expect(eventRange('2020-01-01', now)).toBeNull();
    expect(eventRange('bad', now)).toBeNull();
    expect(eventRange('2026-10-21T12:00:30Z', now)?.from).toBe('2026-10-21T12:00:00.000Z');
  });
  const lines: LogEntry[] = [
    { timestamp: null, stream: 'stdout', message: 'Hello\nworld', partial: false },
    {
      timestamp: '2026-10-28T11:59:30Z',
      stream: 'stderr',
      message: 'HELLO <script>',
      partial: true,
    },
    { timestamp: null, stream: 'combined', message: 'tty', partial: false },
  ];
  it('combines literal case-insensitive multiline search and stream filtering without changing the snapshot', () => {
    expect(filterLogEntries(lines, 'hello', 'stderr')).toEqual([lines[1]]);
    expect(filterLogEntries(lines, 'lo\nwo', 'all')).toEqual([lines[0]]);
    expect(filterLogEntries(lines, '[', 'all')).toEqual([]);
    expect(filterLogEntries(lines, '', 'combined')).toEqual([lines[2]]);
    expect(lines).toHaveLength(3);
  });
  it('copies only matching entries with provenance, partial markers and multiline text', () => {
    expect(copyLogText(filterLogEntries(lines, 'hello', 'stderr'))).toBe(
      '2026-10-28T11:59:30Z [stderr] HELLO <script> [partial line]',
    );
    expect(copyLogText([lines[0]])).toContain('Hello\nworld');
  });
});

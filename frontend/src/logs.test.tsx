import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { LogRows } from './LogsPanel';
import { snapshotRange } from './logs';

describe('log snapshots', () => {
  it('uses the exact resource window when its duration matches', () => {
    const now = Date.parse('2026-10-24T12:00:00Z');
    const range = { from: '2026-10-24T11:00:00Z', to: '2026-10-24T12:00:00Z' };
    expect(snapshotRange(3600, range, now)).toEqual(range);
    expect(snapshotRange(900, range, now)).toEqual({
      from: '2026-10-24T11:45:00.000Z',
      to: '2026-10-24T12:00:00.000Z',
    });
  });
  it('rejects stale or malformed resource window snapshots', () => {
    const now = Date.now();
    const value = snapshotRange(3600, { from: 'invalid', to: 'invalid' }, now);
    expect(Date.parse(value.to)).toBe(now);
    expect(Date.parse(value.to) - Date.parse(value.from)).toBe(3600000);
  });
  it('renders untrusted logs as text and does not infer error severity from stderr', () => {
    const html = renderToStaticMarkup(
      <table>
        <tbody>
          <LogRows
            entries={[
              {
                timestamp: null,
                stream: 'stderr',
                message: '<script>alert("x")</script>\nsecond line',
                partial: false,
              },
            ]}
          />
        </tbody>
      </table>,
    );
    expect(html).toContain('&lt;script&gt;');
    expect(html).not.toContain('<script>');
    expect(html).toContain('stderr');
    expect(html).not.toContain('severity');
    expect(html).toContain('second line');
  });
  it('keeps combined TTY provenance and partial output explicit', () => {
    const html = renderToStaticMarkup(
      <table>
        <tbody>
          <LogRows
            entries={[
              { timestamp: null, stream: 'combined', message: 'partial text', partial: true },
            ]}
          />
        </tbody>
      </table>,
    );
    expect(html).toContain('combined');
    expect(html).toContain('Partial line');
    expect(html).toContain('Docker timestamp unavailable');
  });
});

it('shows a full clock time, not only fractional seconds', () => {
  const html = renderToStaticMarkup(
    <table>
      <tbody>
        <LogRows
          entries={[
            {
              timestamp: '2026-10-24T12:34:56.123Z',
              stream: 'stdout',
              message: 'timestamp test',
              partial: false,
            },
          ]}
        />
      </tbody>
    </table>,
  );
  expect(html).toMatch(/\d{2}:\d{2}:\d{2}/);
  expect(html).toContain('123');
});

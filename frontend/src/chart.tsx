import { useEffect, useId, useRef, useState } from 'react';
import type { Bucket, History, Metric } from './types';
import { numeric } from './format';
export interface Series {
  key: Metric;
  label: string;
  color: string;
}
interface Props {
  title: string;
  subtitle: string;
  history: History | null;
  series: Series[];
  interval: number;
  format: (n: number | null | undefined) => string;
  loading: boolean;
  error: string | null;
}
const H = 180,
  L = 68,
  T = 12,
  B = 151;
export function paths(
  samples: Bucket[],
  key: Metric,
  bucket: number,
  interval: number,
  x: (t: string) => number,
  y: (v: number) => number,
): string[] {
  const segments: string[] = [];
  let path = '',
    previous: Bucket | null = null;
  for (const p of samples) {
    const v = p[key];
    if (v == null) {
      if (path) segments.push(path);
      path = '';
      previous = null;
      continue;
    }
    const gap =
      previous &&
      (Date.parse(p.at) - Date.parse(previous.at) > bucket * 1500 ||
        Date.parse(p.first) - Date.parse(previous.last) > interval * 3000);
    if (gap && path) {
      segments.push(path);
      path = '';
    }
    path += (path ? ' L' : 'M') + x(p.at).toFixed(2) + ',' + y(v).toFixed(2);
    previous = p;
  }
  if (path) segments.push(path);
  return segments;
}
export function Chart({
  title,
  subtitle,
  history,
  series,
  interval,
  format,
  loading,
  error,
}: Props) {
  const rawId = useId(),
    id = rawId.replace(/[^a-zA-Z0-9_-]/g, '');
  const [hover, setHover] = useState<number | null>(null);
  const plotRef = useRef<HTMLDivElement>(null);
  const [W, setWidth] = useState(480);
  const R = W - 12;
  useEffect(() => {
    const observer = new ResizeObserver((entries) =>
      setWidth(Math.max(260, entries[0].contentRect.width)),
    );
    if (plotRef.current) observer.observe(plotRef.current);
    return () => observer.disconnect();
  }, []);
  const samples = history?.samples ?? [],
    from = Date.parse(history?.from ?? '') || Date.now() - 3600000,
    to = Date.parse(history?.to ?? '') || Date.now();
  const values = samples.flatMap((p) =>
    series.map((s) => p[s.key]).filter((v): v is number => v != null),
  );
  const max = Math.max(1, ...values) * 1.15;
  const x = (at: string) => L + ((Date.parse(at) - from) / (to - from)) * (R - L);
  const y = (v: number) => B - (v / max) * (B - T);
  const time = (ms: number) =>
    new Date(ms).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  const bucket = hover == null ? null : samples[hover];
  const last = [...samples].reverse().find((p) => series.some((s) => p[s.key] != null));
  const summary = last
    ? series.map((s) => s.label + ' ' + format(last[s.key])).join(' · ')
    : 'No recorded values';
  return (
    <section className="chart-card">
      <div className="chart-heading">
        <div>
          <h3>{title}</h3>
          <p>{subtitle}</p>
        </div>
        <div className="legend">
          {series.map((s) => (
            <span key={s.key}>
              <i style={{ backgroundColor: s.color }} />
              {s.label}
            </span>
          ))}
        </div>
      </div>
      <div className="plot" ref={plotRef} onMouseLeave={() => setHover(null)}>
        <svg
          viewBox={`0 0 ${W} ${H}`}
          role="img"
          aria-label={`${title}. ${summary}`}
          onMouseMove={(e) => {
            if (!samples.length) return;
            const rect = e.currentTarget.getBoundingClientRect();
            const px = ((e.clientX - rect.left) / rect.width) * W;
            const at = from + ((px - L) / (R - L)) * (to - from);
            let nearest = 0;
            for (let i = 1; i < samples.length; i++)
              if (
                Math.abs(Date.parse(samples[i].at) - at) <
                Math.abs(Date.parse(samples[nearest].at) - at)
              )
                nearest = i;
            setHover(nearest);
          }}
        >
          <title>{title}</title>
          <desc>{summary}. Missing observations are not filled with zeros.</desc>
          <defs>
            <clipPath id={'clip-' + id}>
              <rect x={L} y={T} width={R - L} height={B - T + 2} />
            </clipPath>
            {series.map((s, i) => (
              <linearGradient id={id + '-' + i} key={s.key} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={s.color} stopOpacity="0.14" />
                <stop offset="100%" stopColor={s.color} stopOpacity="0.01" />
              </linearGradient>
            ))}
          </defs>
          {[0, 0.5, 1].map((f) => (
            <g key={f}>
              <line x1={L} x2={R} y1={y(max * f)} y2={y(max * f)} className="chart-grid" />
              <text x={L - 10} y={y(max * f) + 3} textAnchor="end" className="axis-label">
                {format(max * f)}
              </text>
            </g>
          ))}
          <g clipPath={`url(#clip-${id})`}>
            {series.map((s, i) => (
              <g key={s.key}>
                {paths(samples, s.key, history?.bucket_seconds ?? 10, interval, x, y).map(
                  (path, j) => {
                    const matches = [...path.matchAll(/[ML]([\d.-]+),([\d.-]+)/g)],
                      first = matches[0],
                      final = matches[matches.length - 1];
                    return (
                      <g key={j}>
                        {first && final && (
                          <path
                            d={path + ` L${final[1]},${B} L${first[1]},${B} Z`}
                            fill={`url(#${id}-${i})`}
                          />
                        )}
                        <path
                          d={path}
                          fill="none"
                          stroke={s.color}
                          strokeWidth="1.7"
                          vectorEffect="non-scaling-stroke"
                        />
                        {matches.length === 1 && first && (
                          <circle
                            cx={Number(first[1])}
                            cy={Number(first[2])}
                            r="2.3"
                            fill={s.color}
                          />
                        )}
                      </g>
                    );
                  },
                )}
              </g>
            ))}
          </g>
          <text x={L} y={H - 5} className="axis-label">
            {time(from)}
          </text>
          <text x={(L + R) / 2} y={H - 5} textAnchor="middle" className="axis-label">
            {time((from + to) / 2)}
          </text>
          <text x={R} y={H - 5} textAnchor="end" className="axis-label">
            {time(to)}
          </text>
          {bucket && (
            <g>
              <line x1={x(bucket.at)} x2={x(bucket.at)} y1={T} y2={B} className="cursor-line" />
              {series.map((s) =>
                bucket[s.key] == null ? null : (
                  <circle
                    key={s.key}
                    cx={x(bucket.at)}
                    cy={y(bucket[s.key]!)}
                    r="3"
                    fill={s.color}
                  />
                ),
              )}
            </g>
          )}
        </svg>
        {(!values.length || error) && (
          <div className="chart-empty">
            {error
              ? 'History unavailable'
              : loading
                ? 'Loading measurements…'
                : 'No recorded values in this window'}
          </div>
        )}
        {bucket && (
          <div className="chart-tooltip">
            <strong>{new Date(bucket.at).toLocaleString()}</strong>
            {series.map((s) => (
              <span key={s.key}>
                {s.label}
                <b>{format(bucket[s.key])}</b>
              </span>
            ))}
            <small>{numeric(bucket.count, 0)} observations</small>
          </div>
        )}
      </div>
      <div className="chart-foot">
        <span>{error ? 'Retry using Refresh' : loading ? 'Updating…' : summary}</span>
        <span>Mean · {history?.bucket_seconds ?? '—'}s buckets</span>
      </div>
    </section>
  );
}

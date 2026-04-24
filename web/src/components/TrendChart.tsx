import { fmtDayMonth } from '../constants';
import { FerberHighlight } from './FerberHighlight';

interface Series<T> {
  getValue: (p: T) => number;
  // Omit to render the series without a moving-average overlay.
  getAvg?: (p: T) => number | null;
  color: string;
  label?: string;
}

interface Props<T> {
  points: T[];
  getDate: (p: T) => string;
  series: Series<T>[];
  formatValue: (v: number) => string;
  title: string;
  highlightFerber?: boolean;
  isFerber?: (p: T) => boolean;
}

const W = 320;
const H = 140;
const PAD = { top: 24, right: 8, bottom: 20, left: 40 };
const CHART_W = W - PAD.left - PAD.right;
const CHART_H = H - PAD.top - PAD.bottom;

export function TrendChart<T>({ points, getDate, series, formatValue, title, highlightFerber, isFerber }: Props<T>) {
  if (points.length === 0) return null;

  // Compute global min/max across all series.
  let maxVal = 1;
  let minVal = 0;
  for (const s of series) {
    const vals = points.map(s.getValue);
    maxVal = Math.max(maxVal, ...vals);
    minVal = Math.min(minVal, ...vals);
  }
  const range = maxVal - minVal || 1;

  function x(i: number): number {
    if (points.length === 1) return PAD.left + CHART_W / 2;
    return PAD.left + (i / (points.length - 1)) * CHART_W;
  }

  function y(v: number): number {
    return PAD.top + CHART_H - ((v - minVal) / range) * CHART_H;
  }

  function buildPath(values: number[]): string {
    return values
      .map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(v).toFixed(1)}`)
      .join(' ');
  }

  function buildAvgPath(s: Series<T>): string {
    if (!s.getAvg) return '';
    const getAvg = s.getAvg;
    return points
      .map((p, i) => ({ i, v: getAvg(p) }))
      .filter((d): d is { i: number; v: number } => d.v !== null)
      .map((d, j) => `${j === 0 ? 'M' : 'L'}${x(d.i).toFixed(1)},${y(d.v).toFixed(1)}`)
      .join(' ');
  }

  // Date labels.
  const dateLabels: { x: number; label: string }[] = [];
  if (points.length <= 7) {
    points.forEach((p, i) => {
      dateLabels.push({ x: x(i), label: fmtDayMonth(new Date(getDate(p))) });
    });
  } else {
    for (const i of [0, Math.floor(points.length / 2), points.length - 1]) {
      dateLabels.push({ x: x(i), label: fmtDayMonth(new Date(getDate(points[i]))) });
    }
  }

  const hasLegend = series.length > 1 && series.some(s => s.label);
  const ferberCheck = isFerber ?? ((_p: T) => false);

  return (
    <div class="trend-chart">
      <div class="trend-title">{title}</div>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" style={{ maxWidth: `${W}px` }}>
        <text x={PAD.left - 4} y={PAD.top + 4} fill="#999" font-size="10" text-anchor="end">
          {formatValue(maxVal)}
        </text>
        <text x={PAD.left - 4} y={PAD.top + CHART_H + 4} fill="#999" font-size="10" text-anchor="end">
          {formatValue(minVal)}
        </text>
        {highlightFerber && (
          <FerberHighlight
            count={points.length}
            isFerber={i => ferberCheck(points[i])}
            x={x}
            left={PAD.left}
            top={PAD.top}
            width={CHART_W}
            height={CHART_H}
          />
        )}

        <line x1={PAD.left} y1={PAD.top + CHART_H} x2={PAD.left + CHART_W} y2={PAD.top + CHART_H} stroke="#222" />

        {series.map((s, si) => {
          const values = points.map(s.getValue);
          const avgPath = buildAvgPath(s);
          return (
            <g key={si}>
              {avgPath && (
                <path d={avgPath} fill="none" stroke={s.color} stroke-width="1.5" stroke-dasharray="4,3" opacity="0.5" />
              )}
              <path d={buildPath(values)} fill="none" stroke={s.color} stroke-width="2" />
              {values.map((v, i) => (
                <circle key={i} cx={x(i)} cy={y(v)} r="3" fill={s.color} />
              ))}
            </g>
          );
        })}

        {dateLabels.map((dl, i) => (
          <text key={i} x={dl.x} y={H - 2} fill="#999" font-size="9" text-anchor="middle">
            {dl.label}
          </text>
        ))}
      </svg>
      {hasLegend && (
        <div class="chart-legend">
          {series.filter(s => s.label).map((s, i) => (
            <div key={i} class="legend-item">
              <div class="legend-dot" style={{ background: s.color }} />
              {s.label}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

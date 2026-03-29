import { TrendPoint } from '../api';

interface Props {
  trends: TrendPoint[];
  /** Function to extract raw value (nanoseconds for durations, count for integers) */
  getValue: (p: TrendPoint) => number;
  /** Function to extract moving average (null if not enough data) */
  getAvg: (p: TrendPoint) => number | null;
  /** Format a value for the Y axis label */
  formatValue: (v: number) => string;
  title: string;
  color: string;
  avgColor?: string;
}

const W = 320;
const H = 140;
const PAD = { top: 24, right: 8, bottom: 20, left: 40 };
const CHART_W = W - PAD.left - PAD.right;
const CHART_H = H - PAD.top - PAD.bottom;

export function TrendChart({ trends, getValue, getAvg, formatValue, title, color, avgColor = '#666' }: Props) {
  if (trends.length === 0) return null;

  const values = trends.map(getValue);
  const maxVal = Math.max(...values, 1);
  const minVal = Math.min(...values, 0);
  const range = maxVal - minVal || 1;

  function x(i: number): number {
    if (trends.length === 1) return PAD.left + CHART_W / 2;
    return PAD.left + (i / (trends.length - 1)) * CHART_W;
  }

  function y(v: number): number {
    return PAD.top + CHART_H - ((v - minVal) / range) * CHART_H;
  }

  // Raw value line
  const rawPath = values
    .map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(v).toFixed(1)}`)
    .join(' ');

  // Moving average line
  const avgPoints = trends
    .map((p, i) => ({ i, v: getAvg(p) }))
    .filter((d): d is { i: number; v: number } => d.v !== null);
  const avgPath = avgPoints
    .map((d, j) => `${j === 0 ? 'M' : 'L'}${x(d.i).toFixed(1)},${y(d.v).toFixed(1)}`)
    .join(' ');

  // Date labels
  const dateLabels: { x: number; label: string }[] = [];
  if (trends.length <= 7) {
    trends.forEach((p, i) => {
      const d = new Date(p.date);
      dateLabels.push({ x: x(i), label: `${d.getMonth() + 1}/${d.getDate()}` });
    });
  } else {
    // Show first, middle, last
    for (const i of [0, Math.floor(trends.length / 2), trends.length - 1]) {
      const d = new Date(trends[i].date);
      dateLabels.push({ x: x(i), label: `${d.getMonth() + 1}/${d.getDate()}` });
    }
  }

  return (
    <div class="trend-chart">
      <div class="trend-title">{title}</div>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" style={{ maxWidth: `${W}px` }}>
        {/* Y axis labels */}
        <text x={PAD.left - 4} y={PAD.top + 4} fill="#666" font-size="10" text-anchor="end">
          {formatValue(maxVal)}
        </text>
        <text x={PAD.left - 4} y={PAD.top + CHART_H + 4} fill="#666" font-size="10" text-anchor="end">
          {formatValue(minVal)}
        </text>

        {/* Grid line */}
        <line x1={PAD.left} y1={PAD.top + CHART_H} x2={PAD.left + CHART_W} y2={PAD.top + CHART_H} stroke="#222" />

        {/* Moving average line (drawn first, behind) */}
        {avgPath && (
          <path d={avgPath} fill="none" stroke={avgColor} stroke-width="2" stroke-dasharray="4,3" />
        )}

        {/* Raw value line */}
        <path d={rawPath} fill="none" stroke={color} stroke-width="2" />

        {/* Data points */}
        {values.map((v, i) => (
          <circle key={i} cx={x(i)} cy={y(v)} r="3" fill={color} />
        ))}

        {/* Date labels */}
        {dateLabels.map((dl, i) => (
          <text key={i} x={dl.x} y={H - 2} fill="#666" font-size="9" text-anchor="middle">
            {dl.label}
          </text>
        ))}
      </svg>
    </div>
  );
}

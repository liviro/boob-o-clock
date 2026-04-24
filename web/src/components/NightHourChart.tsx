import { NIGHT_EPOCH_H, fmtDayMonth } from '../constants';
import { FerberHighlight } from './FerberHighlight';
import { useMeasuredWidth } from '../hooks/useMeasuredWidth';

export type HourDot = {
  hour: number;  // hours since NIGHT_EPOCH_H
};

interface Props<T> {
  points: T[];
  getDate: (p: T) => string;
  // Returns dots for a given point. Each dot has an hour-offset from NIGHT_EPOCH_H.
  getDots: (p: T) => HourDot[];
  // Optional: moving-average line overlay (dashed). Return null for points
  // without sufficient history. Same hour-offset space as getDots.
  getAvgHour?: (p: T) => number | null;
  color: string;
  title: string;
  highlightFerber?: boolean;
  isFerber?: (p: T) => boolean;
}

const H = 160;
const PAD = { top: 24, right: 8, bottom: 20, left: 44 };
const CHART_H = H - PAD.top - PAD.bottom;
const MIN_RANGE_H = 2;

export function NightHourChart<T>({
  points, getDate, getDots, getAvgHour, color, title, highlightFerber, isFerber,
}: Props<T>) {
  // See TrendChart: dynamic W so viewBox user units stay at 1:1 with CSS pixels.
  const [svgRef, W] = useMeasuredWidth<SVGSVGElement>(320);
  const CHART_W = W - PAD.left - PAD.right;

  // Use points in the order they arrive — callers pass chronological
  // (oldest-first) so i=0 plots at the left edge (matches TrendChart).
  const dots: { ni: number; hour: number }[] = [];
  for (let i = 0; i < points.length; i++) {
    for (const d of getDots(points[i])) {
      dots.push({ ni: i, hour: d.hour });
    }
  }
  if (dots.length === 0) return null;

  const avgHours: (number | null)[] = getAvgHour
    ? points.map(p => getAvgHour(p))
    : [];

  const allHours = dots.map(d => d.hour).concat(
    avgHours.filter((v): v is number => v != null),
  );
  let minH = Math.floor(Math.min(...allHours));
  let maxH = Math.ceil(Math.max(...allHours));
  if (maxH - minH < MIN_RANGE_H) {
    const mid = (minH + maxH) / 2;
    minH = mid - MIN_RANGE_H / 2;
    maxH = mid + MIN_RANGE_H / 2;
  }
  minH = Math.max(0, minH - 0.5);
  maxH = maxH + 0.5;
  const rangeH = maxH - minH;

  const n = points.length;
  const x = (ni: number) => n === 1 ? PAD.left + CHART_W / 2 : PAD.left + (ni / (n - 1)) * CHART_W;
  const y = (h: number) => PAD.top + ((h - minH) / rangeH) * CHART_H;

  // Build the moving-average polyline path. Gaps (null avg values) produce
  // separate sub-paths via SVG's "M" command.
  let avgPath = '';
  if (avgHours.length > 0) {
    const segments: string[] = [];
    let inPath = false;
    for (let i = 0; i < avgHours.length; i++) {
      const v = avgHours[i];
      if (v == null) {
        inPath = false;
        continue;
      }
      segments.push(`${inPath ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`);
      inPath = true;
    }
    avgPath = segments.join(' ');
  }

  const dateLabels: { x: number; label: string }[] = [];
  if (n <= 7) {
    points.forEach((p, i) => {
      dateLabels.push({ x: x(i), label: fmtDayMonth(new Date(getDate(p))) });
    });
  } else {
    for (const i of [0, Math.floor(n / 2), n - 1]) {
      dateLabels.push({ x: x(i), label: fmtDayMonth(new Date(getDate(points[i]))) });
    }
  }

  function fmtEpochHour(h: number): string {
    let clock = Math.round(h + NIGHT_EPOCH_H);
    if (clock >= 24) clock -= 24;
    if (clock < 0) clock += 24;
    return String(clock).padStart(2, '0');
  }

  // Step adapts to range to keep the axis around 6 labels.
  const yStepH = rangeH <= 2 ? 1 : rangeH <= 6 ? 1 : rangeH <= 12 ? 2 : 4;
  const yLabels: { y: number; label: string }[] = [];
  for (let h = Math.ceil(minH); h <= Math.floor(maxH); h += yStepH) {
    yLabels.push({ y: y(h), label: fmtEpochHour(h) });
  }

  const ferberCheck = isFerber ?? ((_p: T) => false);

  return (
    <div class="trend-chart">
      <div class="trend-title">{title}</div>
      <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} width="100%">
        {yLabels.map((yl, i) => (
          <text key={i} x={PAD.left - 4} y={yl.y + 3} fill="#999" font-size="9" text-anchor="end">
            {yl.label}
          </text>
        ))}

        {yLabels.map((yl, i) => (
          <line key={`g${i}`} x1={PAD.left} y1={yl.y} x2={PAD.left + CHART_W} y2={yl.y} stroke="#222" />
        ))}

        {highlightFerber && (
          <FerberHighlight
            count={n}
            isFerber={i => ferberCheck(points[i])}
            x={x}
            left={PAD.left}
            top={PAD.top}
            width={CHART_W}
            height={CHART_H}
          />
        )}

        <line x1={PAD.left} y1={PAD.top + CHART_H} x2={PAD.left + CHART_W} y2={PAD.top + CHART_H} stroke="#222" />

        {avgPath && (
          <path d={avgPath} fill="none" stroke={color} stroke-width="1.5" stroke-dasharray="4,3" opacity="0.5" />
        )}

        {dots.map((d, i) => (
          <circle key={i} cx={x(d.ni)} cy={y(d.hour)} r="4" fill={color} opacity="0.85" />
        ))}

        {dateLabels.map((dl, i) => (
          <text key={i} x={dl.x} y={H - 2} fill="#999" font-size="9" text-anchor="middle">
            {dl.label}
          </text>
        ))}
      </svg>
    </div>
  );
}

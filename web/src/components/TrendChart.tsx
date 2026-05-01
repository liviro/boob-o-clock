import { fmtDayMonth } from '../constants';
import { NightModeHighlight } from './NightModeHighlight';
import { useMeasuredWidth } from '../hooks/useMeasuredWidth';
import { buildGappedPath } from './svgPath';

interface Series<T> {
  // null = skip the point (no dot, breaks the line, excluded from min/max).
  // 0 is a real plotted value — use null when the underlying stat is absent.
  getValue: (p: T) => number | null;
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
  highlightChair?: boolean;
  isChair?: (p: T) => boolean;
}

const H = 140;
const PAD = { top: 24, right: 8, bottom: 20, left: 40 };
const CHART_H = H - PAD.top - PAD.bottom;

export function TrendChart<T>({ points, getDate, series, formatValue, title, highlightFerber, isFerber, highlightChair, isChair }: Props<T>) {
  // W tracks the rendered SVG width so the viewBox matches actual CSS pixels.
  // Without this, text and circle-radius in user units get scaled by the
  // viewBox transform when the SVG stretches (e.g. landscape).
  const [svgRef, W] = useMeasuredWidth<SVGSVGElement>(320);
  const CHART_W = W - PAD.left - PAD.right;

  if (points.length === 0) return null;

  const seriesValues = series.map(s => points.map(s.getValue));
  const seriesAvgs = series.map(s => s.getAvg ? points.map(s.getAvg) : null);

  let maxVal = 1;
  let minVal = 0;
  for (const values of seriesValues) {
    for (const v of values) {
      if (v == null) continue;
      if (v > maxVal) maxVal = v;
      if (v < minVal) minVal = v;
    }
  }
  const range = maxVal - minVal || 1;

  function x(i: number): number {
    if (points.length === 1) return PAD.left + CHART_W / 2;
    return PAD.left + (i / (points.length - 1)) * CHART_W;
  }

  function y(v: number): number {
    return PAD.top + CHART_H - ((v - minVal) / range) * CHART_H;
  }

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
  const chairCheck = isChair ?? ((_p: T) => false);

  return (
    <div class="trend-chart">
      <div class="trend-title">{title}</div>
      <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} width="100%">
        <text x={PAD.left - 4} y={PAD.top + 4} fill="#999" font-size="10" text-anchor="end">
          {formatValue(maxVal)}
        </text>
        <text x={PAD.left - 4} y={PAD.top + CHART_H + 4} fill="#999" font-size="10" text-anchor="end">
          {formatValue(minVal)}
        </text>
        {highlightFerber && (
          <NightModeHighlight
            count={points.length}
            isMode={i => ferberCheck(points[i])}
            fill="#1a3a1a"
            x={x}
            left={PAD.left}
            top={PAD.top}
            width={CHART_W}
            height={CHART_H}
          />
        )}
        {highlightChair && (
          <NightModeHighlight
            count={points.length}
            isMode={i => chairCheck(points[i])}
            fill="#3a1a2a"
            x={x}
            left={PAD.left}
            top={PAD.top}
            width={CHART_W}
            height={CHART_H}
          />
        )}

        <line x1={PAD.left} y1={PAD.top + CHART_H} x2={PAD.left + CHART_W} y2={PAD.top + CHART_H} stroke="#222" />

        {series.map((s, si) => {
          const values = seriesValues[si];
          const avgs = seriesAvgs[si];
          const avgPath = avgs ? buildGappedPath(avgs, x, y) : '';
          return (
            <g key={si}>
              {avgPath && (
                <path d={avgPath} fill="none" stroke={s.color} stroke-width="1.5" stroke-dasharray="4,3" opacity="0.5" />
              )}
              <path d={buildGappedPath(values, x, y)} fill="none" stroke={s.color} stroke-width="2" />
              {values.map((v, i) => v == null ? null : (
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

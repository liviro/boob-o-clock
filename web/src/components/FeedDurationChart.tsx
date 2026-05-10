import { useId } from 'preact/hooks';
import { fmtDayMonth, fmtEpochHour } from '../constants';
import { NightModeHighlight } from './NightModeHighlight';
import { useMeasuredWidth } from '../hooks/useMeasuredWidth';

export type FeedSpan = {
  startHour: number;  // hours since NIGHT_EPOCH_H
  endHour: number;    // hours since NIGHT_EPOCH_H, > startHour
};

interface Props<T> {
  points: T[];
  getDate: (p: T) => string;
  // Returns intra-sleep feed spans for a given point. Each span has start
  // and end as hour-offsets from NIGHT_EPOCH_H (matches NightHourChart's
  // HourDot.hour). Empty for nights with no qualifying feeds.
  getSpans: (p: T) => FeedSpan[];
  color: string;
  title: string;
  highlightFerber?: boolean;
  isFerber?: (p: T) => boolean;
  highlightChair?: boolean;
  isChair?: (p: T) => boolean;
}

const H = 230;
const PAD = { top: 24, right: 8, bottom: 20, left: 44 };
const CHART_H = H - PAD.top - PAD.bottom;  // 186
const MIN_RANGE_H = 2;

export function FeedDurationChart<T>({
  points, getDate, getSpans, color, title,
  highlightFerber, isFerber, highlightChair, isChair,
}: Props<T>) {
  // Dynamic W so viewBox user units stay 1:1 with CSS pixels — see TrendChart.
  const [svgRef, W] = useMeasuredWidth<SVGSVGElement>(320);
  const CHART_W = W - PAD.left - PAD.right;
  const clipId = useId();

  const spans: { ni: number; startHour: number; endHour: number }[] = [];
  for (let i = 0; i < points.length; i++) {
    for (const s of getSpans(points[i])) {
      spans.push({ ni: i, ...s });
    }
  }
  if (spans.length === 0) return null;

  // Y-axis range: include both ends of every span so the longest feed fits.
  const allHours = spans.flatMap(s => [s.startHour, s.endHour]);
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
  // Center each column in its CHART_W/n lane so edge slivers sit flush
  // inside the chart area without clipping.
  const x = (ni: number) => PAD.left + ((ni + 0.5) / n) * CHART_W;
  const y = (h: number) => PAD.top + ((h - minH) / rangeH) * CHART_H;

  const sliverWidth = CHART_W / n;

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

  const yStepH = rangeH <= 2 ? 1 : rangeH <= 6 ? 1 : rangeH <= 12 ? 2 : 4;
  const yLabels: { y: number; label: string }[] = [];
  for (let h = Math.ceil(minH); h <= Math.floor(maxH); h += yStepH) {
    yLabels.push({ y: y(h), label: fmtEpochHour(h) });
  }

  const ferberCheck = isFerber ?? ((_p: T) => false);
  const chairCheck = isChair ?? ((_p: T) => false);

  return (
    <div class="trend-chart">
      <div class="trend-title">{title}</div>
      <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} width="100%">
        <defs>
          <clipPath id={clipId}>
            <rect x={PAD.left} y={PAD.top} width={CHART_W} height={CHART_H} />
          </clipPath>
        </defs>

        {yLabels.map((yl, i) => (
          <text key={i} x={PAD.left - 4} y={yl.y + 3} fill="#999" font-size="9" text-anchor="end">
            {yl.label}
          </text>
        ))}

        {yLabels.map((yl, i) => (
          <line key={`g${i}`} x1={PAD.left} y1={yl.y} x2={PAD.left + CHART_W} y2={yl.y} stroke="#222" />
        ))}

        {highlightFerber && (
          <NightModeHighlight
            count={n}
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
            count={n}
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

        <g clip-path={`url(#${clipId})`}>
          {spans.map((s, i) => {
            const yTop = y(s.startHour);
            const yBottom = y(s.endHour);
            return (
              <rect
                key={i}
                x={x(s.ni) - sliverWidth / 2}
                y={yTop}
                width={sliverWidth}
                height={yBottom - yTop}
                fill={color}
                opacity="0.85"
              />
            );
          })}
        </g>

        {dateLabels.map((dl, i) => (
          <text key={i} x={dl.x} y={H - 2} fill="#999" font-size="9" text-anchor="middle">
            {dl.label}
          </text>
        ))}
      </svg>
    </div>
  );
}

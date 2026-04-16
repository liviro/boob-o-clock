import { NightSummary } from '../api';
import { toNightHour, fmtHour, fmtDayMonth } from '../constants';

interface Props {
  nights: NightSummary[];
}

const W = 320;
const H = 160;
const PAD = { top: 24, right: 8, bottom: 20, left: 44 };
const CHART_W = W - PAD.left - PAD.right;
const CHART_H = H - PAD.top - PAD.bottom;
const MIN_RANGE_H = 2; // minimum 2-hour Y axis range

export function FeedScatterChart({ nights }: Props) {
  // Collect all feed points: { nightIndex, nightHour }
  const allNights = [...nights].reverse();
  const points: { ni: number; nh: number }[] = [];

  for (let i = 0; i < allNights.length; i++) {
    const ft = allNights[i].stats.feedTimes;
    if (!ft) continue;
    for (const ts of ft) {
      points.push({ ni: i, nh: toNightHour(ts) });
    }
  }

  if (points.length === 0) return null;

  // Dynamic Y range from data
  const allHours = points.map(p => p.nh);
  let minH = Math.floor(Math.min(...allHours));
  let maxH = Math.ceil(Math.max(...allHours));
  // Enforce minimum range
  if (maxH - minH < MIN_RANGE_H) {
    const mid = (minH + maxH) / 2;
    minH = mid - MIN_RANGE_H / 2;
    maxH = mid + MIN_RANGE_H / 2;
  }
  // Add a little padding
  minH = Math.max(0, minH - 0.5);
  maxH = maxH + 0.5;
  const rangeH = maxH - minH;

  const n = allNights.length;

  function x(ni: number): number {
    if (n === 1) return PAD.left + CHART_W / 2;
    return PAD.left + (ni / (n - 1)) * CHART_W;
  }

  // Y is inverted: early night (low nightHour) at top, late night at bottom
  function y(nh: number): number {
    return PAD.top + ((nh - minH) / rangeH) * CHART_H;
  }

  // Date labels
  const dateLabels: { x: number; label: string }[] = [];
  if (n <= 7) {
    allNights.forEach((night, i) => {
      dateLabels.push({ x: x(i), label: fmtDayMonth(new Date(night.startedAt)) });
    });
  } else {
    for (const i of [0, Math.floor(n / 2), n - 1]) {
      dateLabels.push({ x: x(i), label: fmtDayMonth(new Date(allNights[i].startedAt)) });
    }
  }

  // Y axis labels: show ~3-4 evenly spaced clock times
  const yStepH = rangeH <= 4 ? 1 : 2;
  const yLabels: { y: number; label: string }[] = [];
  for (let h = Math.ceil(minH); h <= Math.floor(maxH); h += yStepH) {
    yLabels.push({ y: y(h), label: fmtHour(h) });
  }

  return (
    <div class="trend-chart">
      <div class="trend-title">Feed Times</div>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" style={{ maxWidth: `${W}px` }}>
        {/* Y axis labels */}
        {yLabels.map((yl, i) => (
          <text key={i} x={PAD.left - 4} y={yl.y + 3} fill="#999" font-size="9" text-anchor="end">
            {yl.label}
          </text>
        ))}

        {/* Horizontal grid lines */}
        {yLabels.map((yl, i) => (
          <line key={`g${i}`} x1={PAD.left} y1={yl.y} x2={PAD.left + CHART_W} y2={yl.y} stroke="#222" />
        ))}

        {/* Bottom axis */}
        <line x1={PAD.left} y1={PAD.top + CHART_H} x2={PAD.left + CHART_W} y2={PAD.top + CHART_H} stroke="#222" />

        {/* Feed dots */}
        {points.map((p, i) => (
          <circle key={i} cx={x(p.ni)} cy={y(p.nh)} r="4" fill="#c0b040" opacity="0.85" />
        ))}

        {/* Date labels */}
        {dateLabels.map((dl, i) => (
          <text key={i} x={dl.x} y={H - 2} fill="#999" font-size="9" text-anchor="middle">
            {dl.label}
          </text>
        ))}
      </svg>
    </div>
  );
}

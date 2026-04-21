import { NightSummary } from '../api';
import { fmtHour, fmtDayMonth } from '../constants';
import { FerberHighlight } from './FerberHighlight';

interface Props {
  nights: NightSummary[];
  // Maps a night to zero or more night-hours (hours since NIGHT_EPOCH_H).
  // Callers pre-compute via toNightHour() on the raw timestamp.
  getHours: (n: NightSummary) => number[];
  color: string;
  title: string;
  highlightFerber?: boolean;
}

const W = 320;
const H = 160;
const PAD = { top: 24, right: 8, bottom: 20, left: 44 };
const CHART_W = W - PAD.left - PAD.right;
const CHART_H = H - PAD.top - PAD.bottom;
const MIN_RANGE_H = 2;

export function NightHourChart({ nights, getHours, color, title, highlightFerber }: Props) {
  const allNights = [...nights].reverse();
  const points: { ni: number; nh: number }[] = [];
  for (let i = 0; i < allNights.length; i++) {
    for (const nh of getHours(allNights[i])) {
      points.push({ ni: i, nh });
    }
  }
  if (points.length === 0) return null;

  const allHours = points.map(p => p.nh);
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

  const n = allNights.length;
  const x = (ni: number) => n === 1 ? PAD.left + CHART_W / 2 : PAD.left + (ni / (n - 1)) * CHART_W;
  const y = (nh: number) => PAD.top + ((nh - minH) / rangeH) * CHART_H;

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

  const yStepH = rangeH <= 4 ? 1 : 2;
  const yLabels: { y: number; label: string }[] = [];
  for (let h = Math.ceil(minH); h <= Math.floor(maxH); h += yStepH) {
    yLabels.push({ y: y(h), label: fmtHour(h) });
  }

  return (
    <div class="trend-chart">
      <div class="trend-title">{title}</div>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" style={{ maxWidth: `${W}px` }}>
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
            isFerber={i => !!allNights[i].ferberEnabled}
            x={x}
            left={PAD.left}
            top={PAD.top}
            width={CHART_W}
            height={CHART_H}
          />
        )}

        <line x1={PAD.left} y1={PAD.top + CHART_H} x2={PAD.left + CHART_W} y2={PAD.top + CHART_H} stroke="#222" />

        {points.map((p, i) => (
          <circle key={i} cx={x(p.ni)} cy={y(p.nh)} r="4" fill={color} opacity="0.85" />
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

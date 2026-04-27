import { useMeasuredWidth } from '../hooks/useMeasuredWidth';

interface Props<T> {
  points: T[];                    // assumed in chronological order (oldest first)
  getX: (p: T) => number;
  getY: (p: T) => number;
  formatX: (v: number) => string;
  formatY: (v: number) => string;
  title: string;
  color: string;
}

const H = 180;
const PAD = { top: 24, right: 12, bottom: 24, left: 48 };
const CHART_H = H - PAD.top - PAD.bottom;

// ScatterChart plots each point at (getX(p), getY(p)) with both axes pinned
// at zero. Opacity gradient encodes recency: oldest faintest, newest fullest.
export function ScatterChart<T>({ points, getX, getY, formatX, formatY, title, color }: Props<T>) {
  const [svgRef, W] = useMeasuredWidth<SVGSVGElement>(320);
  const CHART_W = W - PAD.left - PAD.right;

  if (points.length === 0) return null;

  const xs = points.map(getX);
  const ys = points.map(getY);
  const xMax = Math.max(1, ...xs);
  const yMax = Math.max(1, ...ys);

  const px = (v: number) => PAD.left + (v / xMax) * CHART_W;
  const py = (v: number) => PAD.top + CHART_H - (v / yMax) * CHART_H;

  return (
    <div class="trend-chart">
      <div class="trend-title">{title}</div>
      <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} width="100%">
        <text x={PAD.left - 4} y={PAD.top + 4} fill="#999" font-size="10" text-anchor="end">{formatY(yMax)}</text>
        <text x={PAD.left - 4} y={PAD.top + CHART_H + 4} fill="#999" font-size="10" text-anchor="end">0</text>

        <line x1={PAD.left} y1={PAD.top + CHART_H} x2={PAD.left + CHART_W} y2={PAD.top + CHART_H} stroke="#222" />
        <line x1={PAD.left} y1={PAD.top} x2={PAD.left} y2={PAD.top + CHART_H} stroke="#222" />

        <text x={PAD.left} y={H - 6} fill="#999" font-size="10" text-anchor="start">0</text>
        <text x={PAD.left + CHART_W} y={H - 6} fill="#999" font-size="10" text-anchor="end">{formatX(xMax)}</text>

        {points.map((_, i) => {
          const opacity = points.length === 1 ? 1 : 0.3 + 0.7 * (i / (points.length - 1));
          return <circle key={i} cx={px(xs[i])} cy={py(ys[i])} r="3.5" fill={color} opacity={opacity} />;
        })}
      </svg>
    </div>
  );
}

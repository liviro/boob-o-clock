interface Props {
  count: number;
  isMode: (i: number) => boolean;
  fill: string;
  x: (i: number) => number;
  left: number;
  top: number;
  width: number;
  height: number;
}

// NightModeHighlight paints translucent vertical stripes behind chart points
// whose night was tagged with a particular sleep-training mode (Ferber, Chair).
// Charts call it once per mode with mode-specific predicate + color.
export function NightModeHighlight({ count, isMode, fill, x, left, top, width, height }: Props) {
  const stripeW = count > 1 ? width / (count - 1) : width * 0.15;
  const right = left + width;
  const rects = [];
  for (let i = 0; i < count; i++) {
    if (!isMode(i)) continue;
    const cx = x(i);
    const l = Math.max(left, cx - stripeW / 2);
    const r = Math.min(right, cx + stripeW / 2);
    rects.push(
      <rect key={i} x={l} y={top} width={r - l} height={height} fill={fill} opacity="0.8" />
    );
  }
  return <>{rects}</>;
}

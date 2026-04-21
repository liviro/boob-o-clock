interface Props {
  count: number;
  isFerber: (i: number) => boolean;
  x: (i: number) => number;
  left: number;
  top: number;
  width: number;
  height: number;
}

export function FerberHighlight({ count, isFerber, x, left, top, width, height }: Props) {
  const stripeW = count > 1 ? width / (count - 1) : width * 0.15;
  const right = left + width;
  const rects = [];
  for (let i = 0; i < count; i++) {
    if (!isFerber(i)) continue;
    const cx = x(i);
    const l = Math.max(left, cx - stripeW / 2);
    const r = Math.min(right, cx + stripeW / 2);
    rects.push(
      <rect key={i} x={l} y={top} width={r - l} height={height} fill="#1a3a1a" opacity="0.8" />
    );
  }
  return <>{rects}</>;
}

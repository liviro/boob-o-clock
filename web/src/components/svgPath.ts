// null entries break the path: M starts each non-null run, L extends it.
export function buildGappedPath(
  values: (number | null)[],
  x: (i: number) => number,
  y: (v: number) => number,
): string {
  const segments: string[] = [];
  let inPath = false;
  values.forEach((v, i) => {
    if (v == null) {
      inPath = false;
      return;
    }
    segments.push(`${inPath ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`);
    inPath = true;
  });
  return segments.join(' ');
}

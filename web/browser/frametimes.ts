// The WebGL exercise's arithmetic (webgl-walk.spec.ts, agni issue 1330). It summarizes frame times
// and lines up two renderers' screen frames. The browser driving lives in the spec, so these can be
// unit tested (src/frametimes.test.ts). The file has no imports.

export interface FrameSummary {
  frames: number;
  p50: number;
  p95: number;
  max: number;
}

// summarize reads a run of requestAnimationFrame deltas in milliseconds. A percentile is the
// nearest rank, so p95 of twenty frames is the nineteenth slowest. An empty run summarizes to
// zeros, with frames 0 saying so.
export function summarize(deltas: readonly number[]): FrameSummary {
  if (deltas.length === 0) return { frames: 0, p50: 0, p95: 0, max: 0 };
  const sorted = [...deltas].sort((a, b) => a - b);
  const rank = (p: number) => sorted[Math.min(sorted.length - 1, Math.max(0, Math.ceil(p * sorted.length) - 1))];
  return { frames: sorted.length, p50: rank(0.5), p95: rank(0.95), max: sorted[sorted.length - 1] };
}

export type Point = [number, number];

// A Frame maps a client point in one renderer to the client point showing the same place in the
// other, as x' = kx*x + cx and y' = ky*y + cy. The renderers fit a sheet to the view on their own,
// so the same place can sit a few pixels apart in each, at a slightly different scale. Neither one
// rotates or mirrors a sheet on the canvas.
export interface Frame {
  kx: number;
  cx: number;
  ky: number;
  cy: number;
  // residual is the median distance, in pixels, between a mapped anchor and its twin, over the
  // anchors the fit kept.
  residual: number;
  anchors: number;
}

// fitFrame fits a Frame to pairs of the same label's anchor in each renderer. It fits by least
// squares, keeps the half of the pairs nearest the fit, and fits again. A label whose renderers
// justify it differently then pulls the second fit nowhere. It needs three pairs, and returns null
// with fewer.
export function fitFrame(pairs: readonly [Point, Point][]): Frame | null {
  if (pairs.length < 3) return null;
  const first = fitOnce(pairs);
  const kept = [...pairs]
    .map((p) => ({ p, d: distance(first, p) }))
    .sort((a, b) => a.d - b.d)
    .slice(0, Math.max(3, Math.ceil(pairs.length / 2)))
    .map((x) => x.p);
  const f = fitOnce(kept);
  const ds = kept.map((p) => distance(f, p)).sort((a, b) => a - b);
  return { ...f, residual: ds[ds.length >> 1], anchors: kept.length };
}

// mapPoint carries a client point through a Frame.
export function mapPoint(f: Pick<Frame, "kx" | "cx" | "ky" | "cy">, [x, y]: Point): Point {
  return [f.kx * x + f.cx, f.ky * y + f.cy];
}

function fitOnce(pairs: readonly [Point, Point][]): Omit<Frame, "residual" | "anchors"> {
  const axis = (i: 0 | 1): [number, number] => {
    const n = pairs.length;
    let sa = 0, sb = 0, saa = 0, sab = 0;
    for (const [a, b] of pairs) {
      sa += a[i];
      sb += b[i];
      saa += a[i] * a[i];
      sab += a[i] * b[i];
    }
    const den = n * saa - sa * sa;
    // Every anchor on one line along this axis leaves its scale unknown, so take it as 1.
    const k = den === 0 ? 1 : (n * sab - sa * sb) / den;
    return [k, (sb - k * sa) / n];
  };
  const [kx, cx] = axis(0);
  const [ky, cy] = axis(1);
  return { kx, cx, ky, cy };
}

function distance(f: Omit<Frame, "residual" | "anchors">, [a, b]: [Point, Point]): number {
  const [x, y] = mapPoint(f, a);
  return Math.hypot(x - b[0], y - b[1]);
}

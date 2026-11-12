// A glide moves a view to a revealed subject over a short time instead of in one frame (agni issue
// 1546). A reader who clicks a part in a panel sees the drawing travel to it, so they keep track of
// where on the sheet it is. Both renderers glide the same way: SvgView's revealOverlay and the WebGL
// canvas's reveal.
//
// The zoom changes on a log scale, so a twentyfold zoom reads as steady rather than a lurch at the
// end, and the centre moves on a straight line in the drawing's own coordinates.

import type { CameraView } from "./camera.js";

// GLIDE_MS is how long a glide takes.
export const GLIDE_MS = 300;

// Clock is what a glide reads time and asks for frames from, so a test can drive it by hand.
export interface Clock {
  now(): number;
  frame(cb: () => void): number;
  cancel(id: number): void;
}

const browserClock: Clock = {
  now: () => performance.now(),
  frame: (cb) => requestAnimationFrame(cb),
  cancel: (id) => cancelAnimationFrame(id),
};

// motionAllowed says whether this page may animate a view. A reader whose system asks for reduced
// motion gets the jump, and so does a page with no matchMedia (jsdom) or no animation frames.
export function motionAllowed(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function" || typeof requestAnimationFrame !== "function") return false;
  if (typeof document !== "undefined" && document.visibilityState === "hidden") return false;
  return !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

// easeInOut is a cubic ease, slow at both ends.
export function easeInOut(t: number): number {
  const x = Math.min(1, Math.max(0, t));
  return x < 0.5 ? 4 * x * x * x : 1 - Math.pow(-2 * x + 2, 3) / 2;
}

// geo interpolates two positive numbers on a log scale.
function geo(a: number, b: number, t: number): number {
  return a > 0 && b > 0 ? a * Math.pow(b / a, t) : a + (b - a) * t;
}

// lerpCamera is the WebGL camera between two views at t in [0,1].
export function lerpCamera(a: CameraView, b: CameraView, t: number): CameraView {
  return {
    centerX: a.centerX + (b.centerX - a.centerX) * t,
    centerY: a.centerY + (b.centerY - a.centerY) * t,
    worldPerPixel: geo(a.worldPerPixel, b.worldPerPixel, t),
  };
}

// SvgViewState is SvgView's pan and zoom, a translate in host pixels and a scale.
export interface SvgViewState {
  tx: number;
  ty: number;
  scale: number;
}

// lerpSvgView is an SVG view between two at t in [0,1], for a host w by h pixels. It moves the
// drawing point at the host's centre in a straight line, and the scale on a log scale.
export function lerpSvgView(a: SvgViewState, b: SvgViewState, w: number, h: number, t: number): SvgViewState {
  const centre = (v: SvgViewState) => ({ x: (w / 2 - v.tx) / v.scale, y: (h / 2 - v.ty) / v.scale });
  const ca = centre(a);
  const cb = centre(b);
  const scale = geo(a.scale, b.scale, t);
  const x = ca.x + (cb.x - ca.x) * t;
  const y = ca.y + (cb.y - ca.y) * t;
  return { tx: w / 2 - x * scale, ty: h / 2 - y * scale, scale };
}

// Glide runs one view animation at a time. A new run replaces the one in flight, and stop leaves the
// view where it is, for a reader who takes the wheel mid-glide.
export class Glide {
  private id: number | null = null;

  constructor(
    private readonly clock: Clock = browserClock,
    private readonly allowed: () => boolean = motionAllowed,
  ) {}

  get active(): boolean {
    return this.id !== null;
  }

  // run moves from `from` to `to` through lerp, calling apply each frame and once with `to` at the
  // end. Without motion it applies `to` at once.
  run<T>(from: T, to: T, lerp: (a: T, b: T, t: number) => T, apply: (v: T) => void): void {
    this.stop();
    if (!this.allowed()) {
      apply(to);
      return;
    }
    const start = this.clock.now();
    const step = (): void => {
      const t = (this.clock.now() - start) / GLIDE_MS;
      if (t >= 1) {
        this.id = null;
        apply(to);
        return;
      }
      apply(lerp(from, to, easeInOut(t)));
      this.id = this.clock.frame(step);
    };
    this.id = this.clock.frame(step);
  }

  stop(): void {
    if (this.id !== null) this.clock.cancel(this.id);
    this.id = null;
  }
}

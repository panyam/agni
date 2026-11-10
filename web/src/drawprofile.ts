// DrawProfile measures how a drawing keeps up with pan and zoom, for a page opened with ?timing
// (agni issue 1331). A team that sees a board lag sends back its log lines, and the lines say which
// renderer drew, on what GPU, how large the scene was, and where each frame's time went.
//
// A burst starts at the first pan or zoom and ends once no input has come for BURST_IDLE_MS. During a
// burst the profile times each animation frame that followed an input, so a frame interval covers
// everything the browser did to show that input. That includes our draw, the text layer's style and
// layout, and compositing. A frame with no input since the last one drew nothing new, and counting
// it would pull the median down to the display's refresh. A WebGL drawing also
// reports what its own draw and text update cost (rendered), so the rest of an interval is the
// browser's. At the end of each burst the profile logs one line, "agni draw <view>: <DrawReport as
// JSON>", covering every burst since the sheet was drawn.
//
// A DrawReport holds numbers, the renderer's name and the GPU's name, and nothing else. It has no
// field for a net name, a ref-des or a sheet name, so a report from a private board can be pasted
// into a public ticket.

// BURST_IDLE_MS is how long the drawing has to go without input before a burst ends.
// A lagging machine can take most of a second over one frame, so a shorter wait would cut one pan
// into several bursts.
export const BURST_IDLE_MS = 1000;

// MAX_SAMPLES bounds each sample list, so a long session holds a fixed amount.
const MAX_SAMPLES = 20000;

export interface Stat {
  n: number;
  p50: number;
  p95: number;
  max: number;
}

// DrawEnv is the machine. gpu and gpuVendor are the WebGL renderer and vendor strings, unmasked where
// the browser allows.
export interface DrawEnv {
  gpu: string;
  gpuVendor: string;
  dpr: number;
  cores: number;
  memoryGb: number;
  cssWidth: number;
  cssHeight: number;
}

// DrawScene counts what one sheet drawing holds. A WebGL drawing fills primitives, vertices, labels
// and images, and an SVG drawing fills elements. buildMs is the time to build the drawing from the
// fetched sheet, the GPU upload and text layer for WebGL and the DOM for SVG.
export interface DrawScene {
  primitives: number;
  vertices: number;
  labels: number;
  images: number;
  elements: number;
  buildMs: number;
}

export interface DrawReport {
  renderer: "webgl" | "svg";
  env: DrawEnv;
  scene: DrawScene;
  bursts: number;
  // frame is the interval between animation frames during a burst.
  frame: Stat;
  // draw, text and drawCalls are the WebGL drawing's own cost per frame it drew. Each is empty for SVG.
  draw: Stat;
  text: Stat;
  drawCalls: Stat;
}

// stat summarizes samples by nearest rank, as the WebGL walk's summarize does (browser/frametimes.ts).
export function stat(samples: readonly number[]): Stat {
  if (samples.length === 0) return { n: 0, p50: 0, p95: 0, max: 0 };
  const s = [...samples].sort((a, b) => a - b);
  const rank = (p: number) => s[Math.min(s.length - 1, Math.max(0, Math.ceil(p * s.length) - 1))];
  return { n: s.length, p50: round(rank(0.5)), p95: round(rank(0.95)), max: round(s[s.length - 1]) };
}

function round(v: number): number {
  return Math.round(v * 100) / 100;
}

function push(list: number[], v: number): void {
  if (list.length < MAX_SAMPLES) list.push(v);
}

// ProfileClock is the time and frame source, which a test replaces.
export interface ProfileClock {
  now(): number;
  frame(cb: (t: number) => void): void;
}

const pageClock: ProfileClock = {
  now: () => performance.now(),
  frame: (cb) => requestAnimationFrame(cb),
};

const EMPTY_SCENE: DrawScene = { primitives: 0, vertices: 0, labels: 0, images: 0, elements: 0, buildMs: 0 };

export class DrawProfile {
  private renderer: "webgl" | "svg" = "svg";
  private scene: DrawScene = EMPTY_SCENE;
  private bursts = 0;
  private frames: number[] = [];
  private draws: number[] = [];
  private texts: number[] = [];
  private calls: number[] = [];
  private running = false;
  private lastInput = 0;
  private lastFrame = -1;
  // inputs counts inputs, and seen is the count at the last frame, so a frame knows whether one came.
  private inputs = 0;
  private seen = 0;

  constructor(
    private readonly view: string,
    private readonly env: () => DrawEnv,
    private readonly log: (line: string) => void = (l) => console.info(l),
    private readonly clock: ProfileClock = pageClock,
  ) {}

  // sheet starts a new record for a sheet just drawn, and logs its scene, since the build cost is
  // part of a slow first open.
  sheet(renderer: "webgl" | "svg", scene: Partial<DrawScene>): void {
    this.renderer = renderer;
    this.scene = { ...EMPTY_SCENE, ...scene, buildMs: round(scene.buildMs ?? 0) };
    this.bursts = 0;
    this.frames = [];
    this.draws = [];
    this.texts = [];
    this.calls = [];
    this.log(`agni draw ${this.view}: ${JSON.stringify(this.report())}`);
  }

  // input marks a pan or zoom, and starts a burst when none is running.
  input(): void {
    this.lastInput = this.clock.now();
    this.inputs++;
    if (this.running) return;
    this.running = true;
    this.bursts++;
    this.lastFrame = -1;
    this.seen = this.inputs;
    this.clock.frame((t) => this.tick(t));
  }

  // rendered records one WebGL frame's own cost, in milliseconds, and its draw calls.
  rendered(drawMs: number, textMs: number, drawCalls: number): void {
    if (!this.running) return;
    push(this.draws, drawMs);
    push(this.texts, textMs);
    push(this.calls, drawCalls);
  }

  private tick(t: number): void {
    if (this.lastFrame >= 0 && this.inputs > this.seen) push(this.frames, t - this.lastFrame);
    this.lastFrame = t;
    this.seen = this.inputs;
    if (this.clock.now() - this.lastInput > BURST_IDLE_MS) {
      this.running = false;
      this.log(`agni draw ${this.view}: ${JSON.stringify(this.report())}`);
      return;
    }
    this.clock.frame((n) => this.tick(n));
  }

  report(): DrawReport {
    return {
      renderer: this.renderer,
      env: this.env(),
      scene: this.scene,
      bursts: this.bursts,
      frame: stat(this.frames),
      draw: stat(this.draws),
      text: stat(this.texts),
      drawCalls: stat(this.calls),
    };
  }
}

// pageEnv reads the machine from the browser. It asks a throwaway WebGL2 context for the GPU's name,
// so an SVG drawing reports it too, and reads it once per page.
let gpuCache: { gpu: string; gpuVendor: string } | undefined;
export function pageEnv(el: HTMLElement): DrawEnv {
  gpuCache ??= gpuName();
  const nav = globalThis.navigator as Navigator & { deviceMemory?: number };
  return {
    ...gpuCache,
    dpr: globalThis.devicePixelRatio || 1,
    cores: nav?.hardwareConcurrency ?? 0,
    memoryGb: nav?.deviceMemory ?? 0,
    cssWidth: el.clientWidth,
    cssHeight: el.clientHeight,
  };
}

function gpuName(): { gpu: string; gpuVendor: string } {
  try {
    const gl = document.createElement("canvas").getContext("webgl2");
    if (!gl) return { gpu: "no webgl2", gpuVendor: "" };
    const info = gl.getExtension("WEBGL_debug_renderer_info");
    const gpu = String(gl.getParameter(info ? info.UNMASKED_RENDERER_WEBGL : gl.RENDERER) ?? "");
    const gpuVendor = String(gl.getParameter(info ? info.UNMASKED_VENDOR_WEBGL : gl.VENDOR) ?? "");
    gl.getExtension("WEBGL_lose_context")?.loseContext();
    return { gpu, gpuVendor };
  } catch {
    return { gpu: "unknown", gpuVendor: "" };
  }
}

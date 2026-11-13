// WebGL2 renderer for a decoded PackedSheet. The vertex positions are int32 and
// sheet-relative, so the attribute is an INTEGER attribute (ivec2) fed with
// gl.vertexAttribIPointer + gl.INT. The vertex buffer is uploaded ONCE; only the mat3
// view uniform changes between frames, and the canvas draws a frame only when something changed. The
// primitives draw through an index buffer in a few calls per run of one group (glbatch.ts, agni issue
// 1484), and the renderer culls a chunk of a run when its bounds fall outside the view rect.

import { DecodedSheet, Primitive, KIND_LINE_STRIP, KIND_LINE_LOOP, KIND_POINTS, KIND_TRIANGLES, pickOnly } from "./packed.js";
import { Bounds } from "./camera.js";
import { buildBatches, wideCount, type Batches, type Chunk, type Stroke } from "./glbatch.js";

type Rgba = [number, number, number, number];

const VERT_SRC = `#version 300 es
in ivec2 a_pos;
uniform mat3 u_view;
void main() {
  vec3 p = u_view * vec3(float(a_pos.x), float(a_pos.y), 1.0);
  gl_Position = vec4(p.xy, 0.0, 1.0);
  gl_PointSize = 4.0;
}
`;

const FRAG_SRC = `#version 300 es
precision highp float;
uniform vec4 u_color;
out vec4 outColor;
void main() {
  outColor = u_color;
}
`;

const DEFAULT_COLOR: Rgba = [0.5, 0.5, 0.5, 1];
const WHITE: Rgba = [1, 1, 1, 1];

// HighlightDraw is one resolved highlight layer for the GPU, holding the primitives to redraw
// and the RGBA to draw them in (alpha already folded in). The canvas builds it from
// resolveHighlights or from a server PackedHighlight.
export interface HighlightDraw {
  color: Rgba;
  primitives: Set<number>;
}

// OverlayDraw is one bounding-shape highlight layer (WS9-017). It holds world-space triangles
// that the canvas tessellated from the entity frames, and it draws translucent above the base
// geometry. It is the one dynamic buffer that crosses to the GPU after load (C4). It re-uploads
// only when the highlight specs change, not per frame.
export interface OverlayDraw {
  color: Rgba;
  vertices: Int32Array; // triangle list, (x,y) world-coordinate pairs
}

// hexToRgba parses "#rgb"/"#rrggbb" into GL's 0..1 RGBA. The palette (group colors + page
// background) is chosen server-side from render.Style, so both renderers share it. An
// unparseable value falls back to `fallback`. alpha (default 1) fills the A channel, for
// translucent highlight tints.
export function hexToRgba(hex: string, fallback: Rgba, alpha = 1): Rgba {
  let h = hex.trim().replace(/^#/, "");
  if (h.length === 3) h = h[0] + h[0] + h[1] + h[1] + h[2] + h[2];
  if (!/^[0-9a-fA-F]{6}$/.test(h)) return fallback;
  return [
    parseInt(h.slice(0, 2), 16) / 255,
    parseInt(h.slice(2, 4), 16) / 255,
    parseInt(h.slice(4, 6), 16) / 255,
    alpha,
  ];
}

// enableBlending lets a translucent draw, a zone's wash or a highlight tint, blend over what lies
// under it, and keeps the canvas opaque. The browser composites a canvas as premultiplied alpha, so a
// canvas left below alpha 1 adds the page behind it, and an 18% wash came out white (agni issue
// 1334). The colour blends by the source's alpha, and the canvas's own alpha only ever adds up to 1.
export function enableBlending(gl: Pick<WebGL2RenderingContext, "enable" | "blendFuncSeparate" | "BLEND" | "SRC_ALPHA" | "ONE_MINUS_SRC_ALPHA" | "ONE">): void {
  gl.enable(gl.BLEND);
  gl.blendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
}

function drawMode(gl: WebGL2RenderingContext, kind: number): number {
  switch (kind) {
    case KIND_LINE_STRIP:
      return gl.LINE_STRIP;
    case KIND_LINE_LOOP:
      return gl.LINE_LOOP;
    case KIND_TRIANGLES:
      return gl.TRIANGLES;
    case KIND_POINTS:
    default:
      return gl.POINTS;
  }
}

// RendererOptions says when the renderer lays out its batches. defer draws the first frame one
// primitive at a time and lays them out after it, so building them costs the first draw nothing.
// Jetson's board takes about 30 ms to lay out, and its first draw is what the board-open mission
// times (agni issue 1499).
export interface RendererOptions {
  defer?: boolean;
}

function compile(gl: WebGL2RenderingContext, type: number, src: string): WebGLShader {
  const sh = gl.createShader(type);
  if (!sh) throw new Error("createShader failed");
  gl.shaderSource(sh, src);
  gl.compileShader(sh);
  if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) {
    const log = gl.getShaderInfoLog(sh);
    gl.deleteShader(sh);
    throw new Error(`shader compile failed: ${log}`);
  }
  return sh;
}

function link(gl: WebGL2RenderingContext, vs: WebGLShader, fs: WebGLShader): WebGLProgram {
  const prog = gl.createProgram();
  if (!prog) throw new Error("createProgram failed");
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.linkProgram(prog);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
    const log = gl.getProgramInfoLog(prog);
    gl.deleteProgram(prog);
    throw new Error(`program link failed: ${log}`);
  }
  return prog;
}

// Precompute the world-space bounds of one primitive record for culling.
function recordBounds(vertices: Int32Array, p: Primitive): Bounds {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  const start = p.firstVertex;
  const end = p.firstVertex + p.count;
  for (let v = start; v < end; v++) {
    const x = vertices[v * 2];
    const y = vertices[v * 2 + 1];
    if (x < minX) minX = x;
    if (y < minY) minY = y;
    if (x > maxX) maxX = x;
    if (y > maxY) maxY = y;
  }
  return { minX, minY, maxX, maxY };
}

// strokeQuads tessellates a line primitive into a triangle list `width` world units wide, as SVG
// strokes it. Each segment runs on by half the width at both ends, so two segments meeting at a
// corner close it as a square join does, and a loop gets its closing segment.
export function strokeQuads(vertices: Int32Array, p: Primitive, width: number): number[] {
  const tris: number[] = [];
  const h = width / 2;
  const segments = p.kind === KIND_LINE_LOOP && p.count > 2 ? p.count : p.count - 1;
  for (let j = 0; j < segments; j++) {
    const a = p.firstVertex + j;
    const b = p.firstVertex + ((j + 1) % p.count);
    const ax = vertices[a * 2], ay = vertices[a * 2 + 1];
    const bx = vertices[b * 2], by = vertices[b * 2 + 1];
    const len = Math.hypot(bx - ax, by - ay);
    if (len === 0) continue;
    const ux = ((bx - ax) / len) * h, uy = ((by - ay) / len) * h;
    const sx = ax - ux, sy = ay - uy, ex = bx + ux, ey = by + uy;
    tris.push(sx - uy, sy + ux, sx + uy, sy - ux, ex - uy, ey + ux);
    tris.push(sx + uy, sy - ux, ex + uy, ey - ux, ex - uy, ey + ux);
  }
  return tris.map(Math.round);
}

// strokesOf appends a triangle list for every line primitive with a width, and returns the vertex
// buffer to upload with each primitive's stroke, if it has one.
function strokesOf(sheet: DecodedSheet): { vertices: Int32Array; strokes: (Stroke | undefined)[] } {
  const strokes: (Stroke | undefined)[] = [];
  const extra: number[][] = [];
  let at = sheet.vertices.length / 2;
  sheet.primitives.forEach((p, i) => {
    if (!p.width || p.count < 2 || (p.kind !== KIND_LINE_STRIP && p.kind !== KIND_LINE_LOOP)) return;
    const tris = strokeQuads(sheet.vertices, p, p.width);
    if (tris.length === 0) return;
    strokes[i] = { first: at, count: tris.length / 2, width: p.width };
    at += tris.length / 2;
    extra.push(tris);
  });
  if (extra.length === 0) return { vertices: sheet.vertices, strokes };
  const vertices = new Int32Array(at * 2);
  vertices.set(sheet.vertices);
  let off = sheet.vertices.length;
  for (const t of extra) {
    vertices.set(t, off);
    off += t.length;
  }
  return { vertices, strokes };
}

function outside(b: Bounds, view: Bounds): boolean {
  return b.maxX < view.minX || b.minX > view.maxX || b.maxY < view.minY || b.minY > view.maxY;
}

export class Renderer {
  private program: WebGLProgram;
  private vao: WebGLVertexArrayObject;
  private buf: WebGLBuffer;
  private uView: WebGLUniformLocation;
  private uColor: WebGLUniformLocation;
  private primitives: Primitive[];
  private bounds: Bounds[];
  // strokes holds the triangles of each line its source draws at a width (agni issue 1486). A line
  // draws as them once the width spans a canvas pixel, and as a 1px GL line below that.
  private strokes: (Stroke | undefined)[];
  // Geometry colors by group and the clear color, resolved from the sheet's server-chosen
  // palette so WebGL matches the SVG backend.
  private colors: Rgba[];
  private background: Rgba;
  // Per-primitive highlight override color, flattened from the highlight layers (a later
  // layer wins an overlap). Empty means no highlight. Set by the canvas from the resolved
  // highlight specs.
  private highlightColors: Map<number, Rgba> = new Map();
  // lit holds the chunks with a highlighted primitive. Such a chunk draws around its highlighted
  // primitives, which draw one by one in their colour, in their place.
  private lit = new Set<number>();
  // batches lays the primitives out in the index buffer ibuf, by run and chunk, for the draw order.
  // Until it is built, a frame draws one primitive at a time.
  private batches: Batches | null = null;
  private ibuf: WebGLBuffer;
  private readonly defer: boolean;
  private scheduled = false;
  private disposed = false;
  // version counts the changes made to what this renderer draws since it was built: highlights,
  // overlays, draw order and hidden groups. A view draws a frame only when it or the camera moved
  // (drawgate.ts, agni issue 1479), so every setter bumps it.
  version = 0;

  constructor(
    private readonly gl: WebGL2RenderingContext,
    sheet: DecodedSheet,
    opts: RendererOptions = {},
  ) {
    this.defer = !!opts.defer;
    const vs = compile(gl, gl.VERTEX_SHADER, VERT_SRC);
    const fs = compile(gl, gl.FRAGMENT_SHADER, FRAG_SRC);
    this.program = link(gl, vs, fs);
    gl.deleteShader(vs);
    gl.deleteShader(fs);

    const uView = gl.getUniformLocation(this.program, "u_view");
    const uColor = gl.getUniformLocation(this.program, "u_color");
    if (!uView || !uColor) throw new Error("failed to resolve uniforms");
    this.uView = uView;
    this.uColor = uColor;

    // Upload the static int32 vertex buffer ONCE.
    const buf = gl.createBuffer();
    const vao = gl.createVertexArray();
    if (!buf || !vao) throw new Error("failed to create buffer/vao");
    this.vao = vao;
    this.buf = buf;
    gl.bindVertexArray(vao);
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    const { vertices, strokes } = strokesOf(sheet);
    this.strokes = strokes;
    gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.STATIC_DRAW);
    const aPos = gl.getAttribLocation(this.program, "a_pos");
    gl.enableVertexAttribArray(aPos);
    // Integer attribute, ivec2 of int32, tightly packed (stride 0 = 8 bytes).
    gl.vertexAttribIPointer(aPos, 2, gl.INT, 0, 0);

    this.primitives = sheet.primitives;
    this.bounds = sheet.primitives.map((p) => recordBounds(sheet.vertices, p));
    // The index buffer belongs to the VAO, so it binds while the VAO is bound.
    const ibuf = gl.createBuffer();
    if (!ibuf) throw new Error("failed to create the index buffer");
    this.ibuf = ibuf;
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibuf);
    gl.bindVertexArray(null);
    gl.bindBuffer(gl.ARRAY_BUFFER, null);

    enableBlending(gl);

    // A group with an alpha draws translucent, as a board's zones wash over it (agni issue 1334).
    this.colors = sheet.groupColors.map((c, g) => hexToRgba(c, DEFAULT_COLOR, sheet.groupAlphas?.[g] || 1));
    this.background = hexToRgba(sheet.backgroundColor, WHITE);
    if (!this.defer) this.prepare();
  }

  // prepare lays out the batches for the current draw order and uploads their index buffer. The
  // pixels a frame draws don't change, so nothing needs drawing again.
  prepare(): void {
    if (this.disposed) return;
    const gl = this.gl;
    this.batches = buildBatches(this.primitives, this.bounds, this.strokes, this.order);
    gl.bindVertexArray(this.vao);
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, this.ibuf);
    gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, this.batches.indices, gl.STATIC_DRAW);
    gl.bindVertexArray(null);
    this.markLit();
  }

  // setHighlights replaces the highlight layers. Each group's primitives redraw in its RGBA,
  // a later group winning where they overlap. An empty array clears every highlight. Called
  // by the canvas whenever the highlight specs (or the sheet) change.
  setHighlights(groups: HighlightDraw[]): void {
    this.version++;
    this.highlightColors = new Map();
    for (const g of groups) {
      for (const i of g.primitives) this.highlightColors.set(i, g.color);
    }
    this.markLit();
  }

  // markLit finds the chunks holding a highlighted primitive.
  private markLit(): void {
    this.lit = new Set();
    if (!this.batches) return;
    for (const i of this.highlightColors.keys()) {
      const c = this.batches.chunkOf[i];
      if (c !== undefined && c >= 0) this.lit.add(c);
    }
  }

  // Bounding-shape overlay state is one dynamically (re)uploaded buffer holding every
  // overlay group's triangles back to back, with per-group offsets so each draws in its
  // own color. Lazily created on the first overlay.
  private overlayBuf: WebGLBuffer | null = null;
  private overlayVao: WebGLVertexArrayObject | null = null;
  private overlayGroups: { color: Rgba; first: number; count: number }[] = [];

  // setOverlays replaces the bounding-shape overlay layers (WS9-017). The concatenated
  // triangle lists upload once per call, since the overlay changes with the highlight specs
  // and not with the camera. An empty array clears the overlay.
  setOverlays(overlays: OverlayDraw[]): void {
    this.version++;
    this.overlayGroups = [];
    const total = overlays.reduce((n, o) => n + o.vertices.length, 0);
    if (total === 0) return;
    const gl = this.gl;
    if (!this.overlayBuf || !this.overlayVao) {
      const buf = gl.createBuffer();
      const vao = gl.createVertexArray();
      if (!buf || !vao) throw new Error("failed to create overlay buffer/vao");
      this.overlayBuf = buf;
      this.overlayVao = vao;
      gl.bindVertexArray(vao);
      gl.bindBuffer(gl.ARRAY_BUFFER, buf);
      const aPos = gl.getAttribLocation(this.program, "a_pos");
      gl.enableVertexAttribArray(aPos);
      gl.vertexAttribIPointer(aPos, 2, gl.INT, 0, 0);
      gl.bindVertexArray(null);
    }
    const all = new Int32Array(total);
    let at = 0;
    for (const o of overlays) {
      this.overlayGroups.push({ color: o.color, first: at / 2, count: o.vertices.length / 2 });
      all.set(o.vertices, at);
      at += o.vertices.length;
    }
    gl.bindBuffer(gl.ARRAY_BUFFER, this.overlayBuf);
    gl.bufferData(gl.ARRAY_BUFFER, all, gl.DYNAMIC_DRAW);
    gl.bindBuffer(gl.ARRAY_BUFFER, null);
  }

  // hiddenGroups is board layer visibility (WS7-035), skipped at draw time so hiding the back
  // side needs no re-upload. Highlighted primitives still draw, so a finding on a hidden layer
  // is still visible.
  private hiddenGroups = new Set<number>();

  // order is the draw order, a permutation of the primitive indices, or null for pack order. A board
  // seen from below draws its strata nearest last (boardstrata.ts, strataDrawOrder).
  private order: Int32Array | null = null;

  // setOrder replaces the draw order. null draws in pack order.
  setOrder(order: Int32Array | null): void {
    this.version++;
    this.order = order;
    if (this.batches) this.prepare();
  }

  // setHiddenGroups replaces the hidden set.
  setHiddenGroups(groups: Iterable<number>): void {
    this.version++;
    this.hiddenGroups = new Set(groups);
  }

  // dispose deletes the vertex buffer, the overlay's buffer, both VAOs and the program. The canvas
  // calls it when a new sheet replaces this one on the same context. A context keeps every object
  // until it is deleted or lost, so each sheet switch used to leave the old sheet on the GPU (agni
  // issue 1331).
  dispose(): void {
    const gl = this.gl;
    this.disposed = true;
    gl.deleteBuffer(this.buf);
    gl.deleteBuffer(this.ibuf);
    gl.deleteVertexArray(this.vao);
    if (this.overlayBuf) gl.deleteBuffer(this.overlayBuf);
    if (this.overlayVao) gl.deleteVertexArray(this.overlayVao);
    gl.deleteProgram(this.program);
    this.overlayBuf = null;
    this.overlayVao = null;
    this.overlayGroups = [];
  }

  // drawEach draws every primitive with a call of its own, in the draw order, as a frame did before
  // the batches (agni issue 1484). The first frame of a deferred renderer draws this way.
  private drawEach(perPx: number, viewRect: Bounds): number {
    const gl = this.gl;
    let calls = 0;
    let lastGroup = -1;
    const order = this.order;
    for (let n = 0; n < this.primitives.length; n++) {
      const i = order ? order[n] : n;
      const p = this.primitives[i];
      if (p.count === 0 || pickOnly(p.group)) continue;
      const hl = this.highlightColors.get(i);
      if (!hl && this.hiddenGroups.has(p.group)) continue;
      if (!hl && outside(this.bounds[i], viewRect)) continue;
      if (hl) {
        gl.uniform4f(this.uColor, hl[0], hl[1], hl[2], hl[3]);
        lastGroup = -1;
      } else if (p.group !== lastGroup) {
        const c = this.colors[p.group] ?? DEFAULT_COLOR;
        gl.uniform4f(this.uColor, c[0], c[1], c[2], c[3]);
        lastGroup = p.group;
      }
      const st = this.strokes[i];
      if (st && st.width >= perPx) gl.drawArrays(gl.TRIANGLES, st.first, st.count);
      else gl.drawArrays(drawMode(gl, p.kind), p.firstVertex, p.count);
      calls++;
    }
    return calls;
  }

  // drawAround draws one region of a chunk that holds a highlighted primitive. The primitives sit back
  // to back in the index buffer, so each stretch between two highlighted ones draws as one call in the
  // base colour, and each highlighted one draws alone in its own. away draws the highlighted ones only.
  private drawAround(
    prims: number[],
    at: Int32Array,
    count: Int32Array,
    mode: number,
    base: Rgba,
    away: boolean,
    use: (c: Rgba) => void,
    draw: (mode: number, start: number, end: number) => void,
  ): void {
    let from = -1;
    let to = -1;
    const flush = () => {
      if (from >= 0 && !away) {
        use(base);
        draw(mode, from, to);
      }
      from = -1;
    };
    for (const i of prims) {
      const hl = this.highlightColors.get(i);
      if (!hl) {
        if (from < 0) from = at[i];
        to = at[i] + count[i];
        continue;
      }
      flush();
      use(hl);
      draw(mode, at[i], at[i] + count[i]);
    }
    flush();
  }

  // Draw one frame and return how many draw calls it made. viewMatrix is the mat3 (column-major)
  // from Camera; viewRect is the world-space rectangle it maps to, used for culling. perPx is the
  // world size of one canvas pixel, and a line with a width at least that draws at its width.
  render(viewMatrix: Float32Array, viewRect: Bounds, perPx = Infinity): number {
    let calls = 0;
    const gl = this.gl;
    gl.clearColor(this.background[0], this.background[1], this.background[2], this.background[3]);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.useProgram(this.program);
    gl.bindVertexArray(this.vao);
    gl.uniformMatrix3fv(this.uView, false, viewMatrix);

    let color: Rgba | null = null;
    const use = (c: Rgba) => {
      if (c === color) return;
      gl.uniform4f(this.uColor, c[0], c[1], c[2], c[3]);
      color = c;
    };
    const draw = (mode: number, start: number, end: number) => {
      if (end <= start) return;
      gl.drawElements(mode, end - start, gl.UNSIGNED_INT, start * 4);
      calls++;
    };
    if (!this.batches) {
      calls += this.drawEach(perPx, viewRect);
      if (this.defer && !this.scheduled) {
        this.scheduled = true;
        setTimeout(() => this.prepare(), 0);
      }
    }
    for (const run of this.batches?.runs ?? []) {
      const base = this.colors[run.group] ?? DEFAULT_COLOR;
      const hidden = this.hiddenGroups.has(run.group);
      for (const c of run.chunks) {
        const lit = this.lit.has(c.id);
        // A highlighted primitive always draws, neither hidden nor culled, so selecting an
        // off-screen finding still tints its element.
        const away = hidden || outside(c.bounds, viewRect);
        if (away && !lit) continue;
        const k = wideCount(c, perPx);
        if (!lit) {
          use(base);
          draw(gl.TRIANGLES, c.strokes.start, k > 0 ? c.strokeEnds[k - 1] : c.strokes.start);
          draw(gl.LINES, k < c.lineList.length ? this.batches!.at[c.lineList[k]] : c.lines.end, c.lines.end);
          draw(gl.TRIANGLES, c.triangles.start, c.triangles.end);
          draw(gl.POINTS, c.points.start, c.points.end);
          continue;
        }
        const b = this.batches!;
        this.drawAround(c.lineList.slice(0, k), b.strokeAt, b.strokeCount, gl.TRIANGLES, base, away, use, draw);
        this.drawAround(c.lineList.slice(k), b.at, b.count, gl.LINES, base, away, use, draw);
        this.drawAround(c.triList, b.at, b.count, gl.TRIANGLES, base, away, use, draw);
        this.drawAround(c.pointList, b.at, b.count, gl.POINTS, base, away, use, draw);
      }
    }
    gl.bindVertexArray(null);

    // The bounding-shape overlay draws last, translucent so the framed entity is still readable
    // through it. Like highlighted primitives it is never culled, so a framed off-screen
    // entity's shape appears the moment it pans in.
    if (this.overlayVao && this.overlayGroups.length > 0) {
      gl.bindVertexArray(this.overlayVao);
      for (const g of this.overlayGroups) {
        gl.uniform4f(this.uColor, g.color[0], g.color[1], g.color[2], g.color[3]);
        gl.drawArrays(gl.TRIANGLES, g.first, g.count);
        calls++;
      }
      gl.bindVertexArray(null);
    }
    return calls;
  }
}

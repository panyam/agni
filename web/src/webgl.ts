// WebGL2 renderer for a decoded PackedSheet. The vertex positions are int32 and
// sheet-relative, so the attribute is an INTEGER attribute (ivec2) fed with
// gl.vertexAttribIPointer + gl.INT. The vertex buffer is uploaded ONCE; only the mat3
// view uniform changes per frame. Each primitive record becomes one gl.drawArrays call,
// with the draw mode and color chosen per record. Records whose precomputed world-space
// bounds fall entirely outside the current view rect are culled.

import {
  KIND_TRIANGLES, DecodedSheet, Primitive, KIND_LINE_STRIP, KIND_LINE_LOOP, KIND_POINTS } from "./packed.js";
import { Bounds } from "./camera.js";

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

// HighlightDraw is one resolved highlight layer for the GPU: the primitives to redraw and
// the RGBA to draw them in (alpha already folded in). Built by the canvas from
// resolveHighlights (or from a server PackedHighlight — same shape either way).
export interface HighlightDraw {
  color: Rgba;
  primitives: Set<number>;
}

// OverlayDraw is one bounding-shape highlight layer (WS9-017): computed world-space
// triangles (already tessellated by the canvas from the entity frames) drawn translucent
// above the base geometry. This is the one dynamic buffer that crosses to the GPU after
// load (C4) — it re-uploads only when the highlight specs change, not per frame.
export interface OverlayDraw {
  color: Rgba;
  vertices: Int32Array; // triangle list, (x,y) world-coordinate pairs
}

// hexToRgba parses "#rgb"/"#rrggbb" into GL's 0..1 RGBA. The palette (group colors + page
// background) is chosen server-side from render.Style, so both renderers share it; an
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

function drawMode(gl: WebGL2RenderingContext, kind: number): number {
  switch (kind) {
    case KIND_LINE_STRIP:
      return gl.LINE_STRIP;
    case KIND_LINE_LOOP:
      return gl.LINE_LOOP;
    case KIND_POINTS:
      return gl.POINTS;
    case KIND_TRIANGLES:
      return gl.TRIANGLES;
    default:
      return gl.POINTS;
  }
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

function outside(b: Bounds, view: Bounds): boolean {
  return b.maxX < view.minX || b.minX > view.maxX || b.maxY < view.minY || b.minY > view.maxY;
}

export class Renderer {
  private program: WebGLProgram;
  private vao: WebGLVertexArrayObject;
  private uView: WebGLUniformLocation;
  private uColor: WebGLUniformLocation;
  private primitives: Primitive[];
  private bounds: Bounds[];
  // Geometry colors by group and the clear color, resolved from the sheet's server-chosen
  // palette so WebGL matches the SVG backend.
  private colors: Rgba[];
  private background: Rgba;
  // Per-primitive highlight override color, flattened from the highlight layers (a later
  // layer wins an overlap). Empty means no highlight. Set by the canvas from the resolved
  // highlight specs.
  private highlightColors: Map<number, Rgba> = new Map();

  constructor(
    private readonly gl: WebGL2RenderingContext,
    sheet: DecodedSheet,
  ) {
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
    gl.bindVertexArray(vao);
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, sheet.vertices, gl.STATIC_DRAW);
    const aPos = gl.getAttribLocation(this.program, "a_pos");
    gl.enableVertexAttribArray(aPos);
    // Integer attribute: ivec2 of int32, tightly packed (stride 0 = 8 bytes).
    gl.vertexAttribIPointer(aPos, 2, gl.INT, 0, 0);
    gl.bindVertexArray(null);
    gl.bindBuffer(gl.ARRAY_BUFFER, null);

    // Highlight tints can be translucent; opaque geometry (alpha 1) is unaffected by blending.
    gl.enable(gl.BLEND);
    gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

    this.primitives = sheet.primitives;
    this.bounds = sheet.primitives.map((p) => recordBounds(sheet.vertices, p));
    this.colors = sheet.groupColors.map((c) => hexToRgba(c, DEFAULT_COLOR));
    this.background = hexToRgba(sheet.backgroundColor, WHITE);
  }

  // setHighlights replaces the highlight layers: each group's primitives redraw in its RGBA,
  // a later group winning where they overlap. An empty array clears every highlight. Called
  // by the canvas whenever the highlight specs (or the sheet) change.
  setHighlights(groups: HighlightDraw[]): void {
    this.highlightColors = new Map();
    for (const g of groups) {
      for (const i of g.primitives) this.highlightColors.set(i, g.color);
    }
  }

  // Bounding-shape overlay state: one dynamically-(re)uploaded buffer holding every
  // overlay group's triangles back to back, with per-group offsets so each draws in its
  // own color. Lazily created on the first overlay.
  private overlayBuf: WebGLBuffer | null = null;
  private overlayVao: WebGLVertexArrayObject | null = null;
  private overlayGroups: { color: Rgba; first: number; count: number }[] = [];

  // setOverlays replaces the bounding-shape overlay layers (WS9-017). The concatenated
  // triangle lists upload once per call — the overlay changes with the highlight specs,
  // not with the camera, so no per-frame upload happens. An empty array clears the overlay.
  setOverlays(overlays: OverlayDraw[]): void {
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

  // hiddenGroups skips whole primitive groups at draw time — board layer visibility
  // (WS7-035): hiding the back side means skipping its groups, no re-upload. Highlighted
  // primitives still draw (a finding on a hidden layer stays visible).
  private hiddenGroups = new Set<number>();

  // setHiddenGroups replaces the hidden set.
  setHiddenGroups(groups: Iterable<number>): void {
    this.hiddenGroups = new Set(groups);
  }

  // Draw one frame. viewMatrix is the mat3 (column-major) from Camera; viewRect is the
  // world-space rectangle it maps to, used for culling.
  render(viewMatrix: Float32Array, viewRect: Bounds): void {
    const gl = this.gl;
    gl.clearColor(this.background[0], this.background[1], this.background[2], this.background[3]);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.useProgram(this.program);
    gl.bindVertexArray(this.vao);
    gl.uniformMatrix3fv(this.uView, false, viewMatrix);

    let lastGroup = -1;
    for (let i = 0; i < this.primitives.length; i++) {
      const p = this.primitives[i];
      if (p.count === 0) continue;
      const hl = this.highlightColors.get(i);
      // A highlighted primitive always draws (never culled), so selecting an off-screen finding
      // still tints its element.
      if (!hl && this.hiddenGroups.has(p.group)) continue;
      if (!hl && outside(this.bounds[i], viewRect)) continue;
      if (hl) {
        gl.uniform4f(this.uColor, hl[0], hl[1], hl[2], hl[3]);
        lastGroup = -1; // force the next non-highlighted primitive to re-set its group color
      } else if (p.group !== lastGroup) {
        const c = this.colors[p.group] ?? DEFAULT_COLOR;
        gl.uniform4f(this.uColor, c[0], c[1], c[2], c[3]);
        lastGroup = p.group;
      }
      gl.drawArrays(drawMode(gl, p.kind), p.firstVertex, p.count);
    }
    gl.bindVertexArray(null);

    // The bounding-shape overlay draws last, above everything: translucent by design, so
    // the entity it frames stays readable through it. Never culled — like highlighted
    // primitives, a framed off-screen entity's shape must appear the moment it pans in.
    if (this.overlayVao && this.overlayGroups.length > 0) {
      gl.bindVertexArray(this.overlayVao);
      for (const g of this.overlayGroups) {
        gl.uniform4f(this.uColor, g.color[0], g.color[1], g.color[2], g.color[3]);
        gl.drawArrays(gl.TRIANGLES, g.first, g.count);
      }
      gl.bindVertexArray(null);
    }
  }
}

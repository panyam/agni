// The parts of a board's stack view that the packed board does not hold (agni issue 1593): a barrel
// for each plated hole, running between the layers it joins, and a translucent slab for each
// dielectric and solder mask, as one face at its top. Depths are millimetres below the board's top
// face, and x and y sit in the packed sheet's rebased frame. The shaders turn a depth into z, so the
// explode control changes two uniforms and uploads nothing.
//
// Every barrel is one shared cylinder, drawn once per hole from a row of five floats: the hole's
// centre, its radius and the depths it runs between. Jetson has some 3,400 plated holes, and a
// cylinder of their own for each took 1.9 MB, 18% of what the board's own buffers hold.

import type { BoardStackView } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { spanDepths, type StackLayout } from "./stacklayout.js";

type Rgba = [number, number, number, number];

// BARREL_SIDES is how many flat sides a barrel draws with.
export const BARREL_SIDES = 8;

// HOLE_FLOATS is the length of one hole's row: x, y, radius, the top depth and the bottom depth.
export const HOLE_FLOATS = 5;

// SLAB_ALPHA is how opaque a slab draws, by its kind. A board has a dozen dielectrics, so each tints
// lightly, or the layers under them vanish.
export const SLAB_ALPHA: Readonly<Record<string, number>> = { dielectric: 0.08, mask: 0.1 };

export interface SlabFace {
  first: number;
  count: number;
  depth: number;
  color: string;
  alpha: number;
}

// StackGeometry is everything stackgl draws. holes holds a HOLE_FLOATS row per hole and nets its net,
// one per hole. slabs holds (x, y, depth) triples, and faces says which run of them each slab is.
export interface StackGeometry {
  holes: Float32Array;
  nets: string[];
  slabs: Float32Array;
  faces: SlabFace[];
}

// stackGeometry builds the barrels and slabs of a board's stack. originX and originY are the packed
// sheet's origin, which the drills' and the outline's board coordinates rebase to. A span whose layers
// the stack does not name draws no barrel.
export function stackGeometry(view: BoardStackView, layout: StackLayout, originX: number, originY: number): StackGeometry {
  const holes: number[] = [];
  const nets: string[] = [];
  for (const sp of view.drills) {
    const depths = spanDepths(layout, sp.layerFrom, sp.layerTo);
    if (!depths) continue;
    for (let i = 0; i < sp.diameter.length; i++) {
      holes.push(sp.xy[2 * i] - originX, sp.xy[2 * i + 1] - originY, sp.diameter[i] / 2, depths[0], depths[1]);
      nets.push(sp.net[i] ?? "");
    }
  }
  const slabs: number[] = [];
  const faces: SlabFace[] = [];
  const tris = view.outlineTris;
  if (tris.length >= 6) {
    for (const p of layout.planes) {
      const alpha = SLAB_ALPHA[p.kind];
      if (alpha === undefined || !p.color) continue;
      const first = slabs.length / 3;
      for (let i = 0; i + 1 < tris.length; i += 2) slabs.push(tris[i] - originX, tris[i + 1] - originY, p.top);
      faces.push({ first, count: slabs.length / 3 - first, depth: p.top, color: p.color, alpha });
    }
  }
  return { holes: Float32Array.from(holes), nets, slabs: Float32Array.from(slabs), faces };
}

// barrelSides is the shared cylinder, as triangles of (cos, sin, end) with end 0 at the top.
export function barrelSides(): Float32Array {
  const out: number[] = [];
  for (let k = 0; k < BARREL_SIDES; k++) {
    const a0 = (2 * Math.PI * k) / BARREL_SIDES;
    const a1 = (2 * Math.PI * (k + 1)) / BARREL_SIDES;
    const c0 = Math.cos(a0), s0 = Math.sin(a0), c1 = Math.cos(a1), s1 = Math.sin(a1);
    out.push(c0, s0, 0, c1, s1, 0, c1, s1, 1, c0, s0, 0, c1, s1, 1, c0, s0, 1);
  }
  return Float32Array.from(out);
}

const BARREL_SRC = `#version 300 es
in vec3 a_side;
in vec3 a_hole;
in vec2 a_span;
uniform mat4 u_view;
uniform float u_zTop;
uniform float u_zPerMm;
void main() {
  float depth = mix(a_span.x, a_span.y, a_side.z);
  vec2 at = a_hole.xy + a_side.xy * a_hole.z;
  gl_Position = u_view * vec4(at, u_zTop - depth * u_zPerMm, 1.0);
}
`;

const SLAB_SRC = `#version 300 es
in vec3 a_pos;
uniform mat4 u_view;
uniform float u_zTop;
uniform float u_zPerMm;
void main() {
  gl_Position = u_view * vec4(a_pos.x, a_pos.y, u_zTop - a_pos.z * u_zPerMm, 1.0);
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

function program(gl: WebGL2RenderingContext, vert: string): WebGLProgram {
  const shader = (type: number, src: string) => {
    const sh = gl.createShader(type);
    if (!sh) throw new Error("createShader failed");
    gl.shaderSource(sh, src);
    gl.compileShader(sh);
    if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) throw new Error(`stack shader compile failed: ${gl.getShaderInfoLog(sh)}`);
    return sh;
  };
  const vs = shader(gl.VERTEX_SHADER, vert);
  const fs = shader(gl.FRAGMENT_SHADER, FRAG_SRC);
  const prog = gl.createProgram();
  if (!prog) throw new Error("createProgram failed");
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.linkProgram(prog);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) throw new Error(`stack program link failed: ${gl.getProgramInfoLog(prog)}`);
  gl.deleteShader(vs);
  gl.deleteShader(fs);
  return prog;
}

// StackDrawOptions is what one frame of the barrels and slabs needs. zTop is the world z of the
// board's top face and zPerMm the world z one millimetre of depth spans. eyeZ is the z of the camera's
// eye direction, which orders the slabs from the farthest to the nearest. lit names the nets whose
// barrels draw in a highlight colour.
export interface StackDrawOptions {
  view: Float32Array;
  zTop: number;
  zPerMm: number;
  eyeZ: number;
  barrels: boolean;
  barrelColor: Rgba;
  lit: Map<string, Rgba>;
  parseColor: (hex: string, alpha: number) => Rgba;
}

interface Part {
  prog: WebGLProgram;
  vao: WebGLVertexArrayObject;
  view: WebGLUniformLocation | null;
  zTop: WebGLUniformLocation | null;
  zPerMm: WebGLUniformLocation | null;
  color: WebGLUniformLocation | null;
}

// StackExtras draws a StackGeometry on the board view's WebGL context. Its buffers upload once.
export class StackExtras {
  private readonly barrels: Part;
  private readonly slabs: Part;
  private readonly buffers: WebGLBuffer[] = [];
  private readonly sides: number;
  private readonly holeCount: number;
  // byNet lists each net's holes by index, for lighting a selected net's barrels.
  private readonly byNet = new Map<string, number[]>();
  private readonly holeLoc: number;
  private readonly spanLoc: number;
  private readonly holeBuf: WebGLBuffer;
  // bytes is what the barrels and slabs hold on the GPU.
  readonly bytes: number;

  constructor(
    private readonly gl: WebGL2RenderingContext,
    private readonly geometry: StackGeometry,
  ) {
    const part = (src: string): Part => {
      const prog = program(gl, src);
      const vao = gl.createVertexArray();
      if (!vao) throw new Error("failed to create a stack vertex array");
      return {
        prog,
        vao,
        view: gl.getUniformLocation(prog, "u_view"),
        zTop: gl.getUniformLocation(prog, "u_zTop"),
        zPerMm: gl.getUniformLocation(prog, "u_zPerMm"),
        color: gl.getUniformLocation(prog, "u_color"),
      };
    };
    const upload = (data: Float32Array): WebGLBuffer => {
      const buf = gl.createBuffer();
      if (!buf) throw new Error("failed to create a stack buffer");
      gl.bindBuffer(gl.ARRAY_BUFFER, buf);
      gl.bufferData(gl.ARRAY_BUFFER, data, gl.STATIC_DRAW);
      this.buffers.push(buf);
      return buf;
    };
    this.barrels = part(BARREL_SRC);
    this.slabs = part(SLAB_SRC);
    const sides = barrelSides();
    this.sides = sides.length / 3;
    this.holeCount = geometry.nets.length;

    gl.bindVertexArray(this.barrels.vao);
    upload(sides);
    const sideLoc = gl.getAttribLocation(this.barrels.prog, "a_side");
    gl.enableVertexAttribArray(sideLoc);
    gl.vertexAttribPointer(sideLoc, 3, gl.FLOAT, false, 0, 0);
    this.holeBuf = upload(geometry.holes);
    this.holeLoc = gl.getAttribLocation(this.barrels.prog, "a_hole");
    this.spanLoc = gl.getAttribLocation(this.barrels.prog, "a_span");
    gl.enableVertexAttribArray(this.holeLoc);
    gl.enableVertexAttribArray(this.spanLoc);
    this.pointHoles(0);
    gl.vertexAttribDivisor(this.holeLoc, 1);
    gl.vertexAttribDivisor(this.spanLoc, 1);

    gl.bindVertexArray(this.slabs.vao);
    upload(geometry.slabs);
    const posLoc = gl.getAttribLocation(this.slabs.prog, "a_pos");
    gl.enableVertexAttribArray(posLoc);
    gl.vertexAttribPointer(posLoc, 3, gl.FLOAT, false, 0, 0);
    gl.bindVertexArray(null);
    gl.bindBuffer(gl.ARRAY_BUFFER, null);

    this.bytes = sides.byteLength + geometry.holes.byteLength + geometry.slabs.byteLength;
    geometry.nets.forEach((net, i) => {
      if (!net) return;
      const list = this.byNet.get(net) ?? [];
      list.push(i);
      this.byNet.set(net, list);
    });
  }

  // pointHoles points the per-hole attributes at the row of hole first, so an instanced draw starts
  // there. WebGL2 has no base instance to do it.
  private pointHoles(first: number): void {
    const gl = this.gl;
    const stride = HOLE_FLOATS * 4;
    gl.bindBuffer(gl.ARRAY_BUFFER, this.holeBuf);
    gl.vertexAttribPointer(this.holeLoc, 3, gl.FLOAT, false, stride, first * stride);
    gl.vertexAttribPointer(this.spanLoc, 2, gl.FLOAT, false, stride, first * stride + 12);
  }

  private use(p: Part, o: StackDrawOptions): void {
    const gl = this.gl;
    gl.useProgram(p.prog);
    gl.bindVertexArray(p.vao);
    gl.uniformMatrix4fv(p.view, false, o.view);
    gl.uniform1f(p.zTop, o.zTop);
    gl.uniform1f(p.zPerMm, o.zPerMm);
  }

  // draw draws the barrels, then the slabs from the farthest to the nearest without writing depth, so
  // every slab tints what lies behind it. It returns the draw calls it made.
  draw(o: StackDrawOptions): number {
    const gl = this.gl;
    let calls = 0;
    if (this.holeCount > 0) {
      this.use(this.barrels, o);
      if (o.barrels) {
        const c = o.barrelColor;
        gl.uniform4f(this.barrels.color, c[0], c[1], c[2], c[3]);
        gl.drawArraysInstanced(gl.TRIANGLES, 0, this.sides, this.holeCount);
        calls++;
      }
      // A lit net's barrels draw even when the through layer is hidden, as a lit track does.
      for (const [net, c] of o.lit) {
        const list = this.byNet.get(net);
        if (!list) continue;
        gl.uniform4f(this.barrels.color, c[0], c[1], c[2], c[3]);
        for (const i of list) {
          this.pointHoles(i);
          gl.drawArraysInstanced(gl.TRIANGLES, 0, this.sides, 1);
          calls++;
        }
      }
      this.pointHoles(0);
    }
    if (this.geometry.faces.length > 0) {
      this.use(this.slabs, o);
      // Seen from above, a deeper face is farther away and draws first.
      const faces = [...this.geometry.faces].sort((a, b) => (o.eyeZ >= 0 ? b.depth - a.depth : a.depth - b.depth));
      gl.depthMask(false);
      for (const f of faces) {
        const c = o.parseColor(f.color, f.alpha);
        gl.uniform4f(this.slabs.color, c[0], c[1], c[2], c[3]);
        gl.drawArrays(gl.TRIANGLES, f.first, f.count);
        calls++;
      }
      gl.depthMask(true);
    }
    gl.bindVertexArray(null);
    gl.bindBuffer(gl.ARRAY_BUFFER, null);
    return calls;
  }

  dispose(): void {
    const gl = this.gl;
    for (const b of this.buffers) gl.deleteBuffer(b);
    for (const p of [this.barrels, this.slabs]) {
      gl.deleteVertexArray(p.vao);
      gl.deleteProgram(p.prog);
    }
  }
}

// The parts of a board's stack view that the packed board does not hold (agni issue 1593): a barrel
// for each plated hole, running between the layers it joins, and a translucent slab for each
// dielectric and solder mask, as one face at its top. Depths are millimetres below the board's top
// face, and x and y sit in the packed sheet's rebased frame. The shaders turn a depth into z, so the
// explode control changes two uniforms and uploads nothing.
//
// Every barrel is one shared cylinder, drawn once per hole from a row of five floats: the hole's
// centre, its radius and the depths it runs between. Jetson has some 3,400 plated holes, and a
// cylinder of their own for each took 1.9 MB, 18% of what the board's own buffers hold.

import type { BoardStackView } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import { spanDepths, type StackLayout } from "./stacklayout.js";
import { BODY_FLOATS, type BodyGeometry } from "./stackbodies.js";

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
  // boardFaces are the board's top and bottom faces, at depth 0 and the stack's total, which the
  // board look fills in mask (agni issue 1653). Their vertices sit in slabs after the faces'.
  boardFaces: { first: number; count: number; depth: number }[];
  // boardWall is the board's edge as a wall from the top face to the bottom one, round the outline
  // and each hole cut in it, which the board look fills too (agni issue 1607). Its vertices sit in
  // slabs after the board faces'.
  boardWall: { first: number; count: number };
}

// WALL_SHADE darkens the mask colour for the board's edge, so the edge reads apart from the face
// above it.
export const WALL_SHADE = 0.7;

// boundaryEdges lists the edges of a flat x,y triangle list that only one triangle uses, as
// [ax, ay, bx, by] rows. Those run round the outline and each hole in it.
export function boundaryEdges(tris: readonly number[]): number[][] {
  const count = new Map<string, number[] | null>();
  for (let i = 0; i + 5 < tris.length; i += 6) {
    for (let k = 0; k < 3; k++) {
      const a = [tris[i + 2 * k], tris[i + 2 * k + 1]];
      const b = [tris[i + ((2 * k + 2) % 6)], tris[i + ((2 * k + 3) % 6)]];
      const ka = `${a[0]},${a[1]}`, kb = `${b[0]},${b[1]}`;
      const key = ka < kb ? `${ka}|${kb}` : `${kb}|${ka}`;
      count.set(key, count.has(key) ? null : [a[0], a[1], b[0], b[1]]);
    }
  }
  return [...count.values()].filter((e): e is number[] => e !== null);
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
  const boardFaces: StackGeometry["boardFaces"] = [];
  if (tris.length >= 6) {
    for (const depth of [0, layout.totalMm]) {
      const first = slabs.length / 3;
      for (let i = 0; i + 1 < tris.length; i += 2) slabs.push(tris[i] - originX, tris[i + 1] - originY, depth);
      boardFaces.push({ first, count: slabs.length / 3 - first, depth });
    }
  }
  const wallFirst = slabs.length / 3;
  if (tris.length >= 6) {
    for (const [ax, ay, bx, by] of boundaryEdges(tris)) {
      const x0 = ax - originX, y0 = ay - originY, x1 = bx - originX, y1 = by - originY;
      const top = 0, bottom = layout.totalMm;
      slabs.push(x0, y0, top, x1, y1, top, x1, y1, bottom, x0, y0, top, x1, y1, bottom, x0, y0, bottom);
    }
  }
  const boardWall = { first: wallFirst, count: slabs.length / 3 - wallFirst };
  return { holes: Float32Array.from(holes), nets, slabs: Float32Array.from(slabs), faces, boardFaces, boardWall };
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

// BODY_SRC raises a part's vertex off its side's face by its height, at the board's true scale
// (stackbodies.ts). a_pos.w holds the shade, plus 2 on the back.
const BODY_SRC = `#version 300 es
in vec4 a_pos;
uniform mat4 u_view;
uniform float u_zTop;
uniform float u_zBottom;
uniform float u_hScale;
out float v_shade;
void main() {
  float back = step(1.5, a_pos.w);
  v_shade = a_pos.w - 2.0 * back;
  float z = mix(u_zTop + a_pos.z * u_hScale, u_zBottom - a_pos.z * u_hScale, back);
  gl_Position = u_view * vec4(a_pos.xy, z, 1.0);
}
`;

const BODY_FRAG_SRC = `#version 300 es
precision highp float;
uniform vec4 u_color;
in float v_shade;
out vec4 outColor;
void main() {
  outColor = vec4(u_color.rgb * v_shade, u_color.a);
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

function program(gl: WebGL2RenderingContext, vert: string, frag = FRAG_SRC): WebGLProgram {
  const shader = (type: number, src: string) => {
    const sh = gl.createShader(type);
    if (!sh) throw new Error("createShader failed");
    gl.shaderSource(sh, src);
    gl.compileShader(sh);
    if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) throw new Error(`stack shader compile failed: ${gl.getShaderInfoLog(sh)}`);
    return sh;
  };
  const vs = shader(gl.VERTEX_SHADER, vert);
  const fs = shader(gl.FRAGMENT_SHADER, frag);
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
  // The parts' bodies draw when bodies is set. zBottom is the world z of the board's bottom face,
  // and hScale the world z one millimetre of a part's height spans. litParts names the parts whose
  // bodies draw in a highlight colour.
  bodies?: {
    zBottom: number;
    hScale: number;
    // colorOf is the colour a run of bodies draws in, from a body's colour and whether its height
    // is a default.
    colorOf: (color: string, defaulted: boolean) => Rgba;
    litParts: Map<string, Rgba>;
  };
  // board draws the board look (stacklook.ts): no barrels and no translucent slabs, and an opaque
  // face in mask at the top and the bottom of the board, behind what lies on it.
  board?: { mask: Rgba };
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
  private readonly bodyPart: Part & { zBottom: WebGLUniformLocation | null; hScale: WebGLUniformLocation | null };
  // bytes is what the barrels and slabs hold on the GPU.
  readonly bytes: number;

  constructor(
    private readonly gl: WebGL2RenderingContext,
    private readonly geometry: StackGeometry,
    private readonly bodies: BodyGeometry | null = null,
  ) {
    const part = (src: string, frag?: string): Part => {
      const prog = program(gl, src, frag);
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
    const bodyPart = part(BODY_SRC, BODY_FRAG_SRC);
    this.bodyPart = { ...bodyPart, zBottom: gl.getUniformLocation(bodyPart.prog, "u_zBottom"), hScale: gl.getUniformLocation(bodyPart.prog, "u_hScale") };
    const bodyVerts = bodies?.verts ?? new Float32Array(0);
    gl.bindVertexArray(this.bodyPart.vao);
    upload(bodyVerts);
    const bodyLoc = gl.getAttribLocation(this.bodyPart.prog, "a_pos");
    gl.enableVertexAttribArray(bodyLoc);
    gl.vertexAttribPointer(bodyLoc, BODY_FLOATS, gl.FLOAT, false, 0, 0);
    gl.bindVertexArray(null);
    gl.bindBuffer(gl.ARRAY_BUFFER, null);

    this.bytes = sides.byteLength + geometry.holes.byteLength + geometry.slabs.byteLength + bodyVerts.byteLength;
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

  // draw draws the parts' bodies and the barrels, then the slabs from the farthest to the nearest
  // without writing depth, so every slab tints what lies behind it. It returns the draw calls it made.
  draw(o: StackDrawOptions): number {
    const gl = this.gl;
    let calls = 0;
    if (o.bodies && this.bodies && this.bodies.verts.length > 0) calls += this.drawBodies(o, o.bodies, this.bodies);
    if (o.board) return calls + this.drawBoardFaces(o, o.board);
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

  // drawBoardFaces fills the board's top and bottom faces in mask, opaque. Each sits at its face's
  // depth, pushed a little away from the eye, so the copper and silkscreen the board look draws on
  // that face win the depth test and draw over it. The edge wall draws after, a shade darker.
  private drawBoardFaces(o: StackDrawOptions, b: NonNullable<StackDrawOptions["board"]>): number {
    const gl = this.gl;
    if (this.geometry.boardFaces.length === 0) return 0;
    this.use(this.slabs, o);
    gl.uniform4f(this.slabs.color, b.mask[0], b.mask[1], b.mask[2], b.mask[3]);
    gl.enable(gl.POLYGON_OFFSET_FILL);
    gl.polygonOffset(1, 1);
    let calls = 0;
    for (const f of this.geometry.boardFaces) {
      gl.drawArrays(gl.TRIANGLES, f.first, f.count);
      calls++;
    }
    gl.disable(gl.POLYGON_OFFSET_FILL);
    const w = this.geometry.boardWall;
    if (w.count > 0) {
      gl.uniform4f(this.slabs.color, b.mask[0] * WALL_SHADE, b.mask[1] * WALL_SHADE, b.mask[2] * WALL_SHADE, b.mask[3]);
      gl.drawArrays(gl.TRIANGLES, w.first, w.count);
      calls++;
    }
    gl.bindVertexArray(null);
    gl.bindBuffer(gl.ARRAY_BUFFER, null);
    return calls;
  }

  // drawBodies draws each run of one colour in one call. A lit part draws again over its body in its
  // highlight colour.
  private drawBodies(o: StackDrawOptions, b: NonNullable<StackDrawOptions["bodies"]>, g: BodyGeometry): number {
    const gl = this.gl;
    const p = this.bodyPart;
    this.use(p, o);
    gl.uniform1f(p.zBottom, b.zBottom);
    gl.uniform1f(p.hScale, b.hScale);
    let calls = 0;
    const run = (first: number, count: number, c: Rgba) => {
      if (count <= 0) return;
      gl.uniform4f(p.color, c[0], c[1], c[2], c[3]);
      gl.drawArrays(gl.TRIANGLES, first, count);
      calls++;
    };
    for (const r of g.colorRuns) run(r.first, r.count, b.colorOf(r.color, r.defaulted));
    for (const [ref, c] of b.litParts) {
      const r = g.runs.get(ref);
      if (r) run(r.first, r.count, c);
    }
    return calls;
  }

  dispose(): void {
    const gl = this.gl;
    for (const b of this.buffers) gl.deleteBuffer(b);
    for (const p of [this.barrels, this.slabs, this.bodyPart]) {
      gl.deleteVertexArray(p.vao);
      gl.deleteProgram(p.prog);
    }
  }
}

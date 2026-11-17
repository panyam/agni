// Part models on a board's stack view (agni issue 1652). The board look draws each part as its own 3D
// model, as its maker's tool shows it, where the server has a mesh for it. Every other part keeps its
// block (stackbodies.ts). The meshes come from DesignViewService.GetBoardMeshes, fitted to a triangle
// budget the reader picks with the Quality control. A part's height never depends on the budget,
// since the server takes heights from the full models.
//
// Many parts share one model, such as every 0402 capacitor on a board. Each mesh uploads once, and
// each part that shows it is an instance: a 3x4 matrix the server composed from the part's placement
// (BoardPartBody.models), which takes a mesh point in millimetres to board units in x and y and to
// millimetres off the part's face in z.

import type { ModelMesh } from "@panyam/agni-client/gen/agni/v1/geom/geom_board_pb";
import type { BoardPartBody } from "@panyam/agni-client/gen/agni/v1/geom/geom_packed_pb";
import type { GetBoardMeshesResponse } from "@panyam/agni-client/gen/agni/v1/webapi/design_view_pb";

// Quality is the stack bar's Quality setting, and QUALITY_TRIANGLES the board-wide triangle budget
// each asks the server for.
export type Quality = "low" | "medium" | "high";
export const QUALITY_TRIANGLES: Record<Quality, number> = { low: 75_000, medium: 300_000, high: 1_000_000 };

// defaultQuality picks a budget from the device's memory in gigabytes, as navigator.deviceMemory
// reports it. A browser that reports none, such as Firefox or Safari, gets medium.
export function defaultQuality(deviceMemoryGb: number | undefined): Quality {
  if (deviceMemoryGb === undefined || !Number.isFinite(deviceMemoryGb)) return "medium";
  if (deviceMemoryGb <= 2) return "low";
  if (deviceMemoryGb <= 4) return "medium";
  return "high";
}

// MeshSet is every page of one GetBoardMeshes answer.
export interface MeshSet {
  meshes: ModelMesh[];
  simplified: string[];
  blocks: string[];
  unread: { path: string; reason: string; parts: number }[];
  triangles: number;
}

// fetchMeshes pages through GetBoardMeshes. page asks for one page by its token, "" for the first.
// The first page carries the summary, and later pages only meshes.
export async function fetchMeshes(page: (token: string) => Promise<GetBoardMeshesResponse>): Promise<MeshSet> {
  const out: MeshSet = { meshes: [], simplified: [], blocks: [], unread: [], triangles: 0 };
  let token = "";
  for (let i = 0; i < 10_000; i++) {
    const resp = await page(token);
    if (i === 0) {
      out.simplified = [...resp.simplified];
      out.blocks = [...resp.blocks];
      out.unread = resp.unread.map((u) => ({ path: u.path, reason: u.reason, parts: u.parts }));
      out.triangles = Number(resp.triangles);
    }
    out.meshes.push(...resp.meshes);
    token = resp.nextPageToken;
    if (!token) return out;
  }
  throw new Error("GetBoardMeshes did not stop paging");
}

// meshNote says what the server left out to fit the budget, for the stack bar beside the Parts note.
// It is "" when every part with a model draws its whole mesh.
export function meshNote(set: MeshSet): string {
  const bits: string[] = [];
  if (set.simplified.length > 0) bits.push(`${set.simplified.length} simplified`);
  if (set.blocks.length > 0) bits.push(`${set.blocks.length} as blocks`);
  const unread = set.unread.reduce((n, u) => n + u.parts, 0);
  if (unread > 0) bits.push(`${unread} with no mesh`);
  return bits.length > 0 ? `(models: ${bits.join(", ")})` : "";
}

// MESH_FLOATS is one instance's length: the 3x4 matrix in row order, then 1 for a part on the back.
export const MESH_FLOATS = 13;

// MeshDraw is one mesh's vertices, triangles and instances, as the GPU takes them. positions holds
// x, y and z in millimetres, colors one RGBA byte quad per vertex, and instances MESH_FLOATS per part.
export interface MeshDraw {
  key: string;
  positions: Float32Array;
  colors: Uint8Array;
  indices: Uint32Array;
  instances: Float32Array;
  refs: string[];
}

// MeshGeometry is what the stack view draws for the parts with meshes. meshed names each part that
// draws at least one mesh, so its block stays out of the board look.
export interface MeshGeometry {
  draws: MeshDraw[];
  meshed: Set<string>;
  triangles: number;
  instances: number;
}

// meshGeometry pairs each body's models with the meshes the server sent. originX and originY are the
// packed sheet's origin, which the bodies' board coordinates rebase to, as bodyGeometry does. A model
// whose mesh did not come, because the server left it as a block or could not read it, draws nothing,
// and a part none of whose models came keeps its block.
export function meshGeometry(bodies: readonly BoardPartBody[], set: MeshSet, originX: number, originY: number): MeshGeometry {
  const byKey = new Map(set.meshes.map((m) => [m.key, m]));
  const instances = new Map<string, { floats: number[]; refs: string[] }>();
  const meshed = new Set<string>();
  for (const b of bodies) {
    for (const m of b.models) {
      if (!byKey.has(m.mesh) || m.transform.length !== 12) continue;
      const t = m.transform;
      const list = instances.get(m.mesh) ?? { floats: [], refs: [] };
      list.floats.push(t[0], t[1], t[2], t[3] - originX, t[4], t[5], t[6], t[7] - originY, t[8], t[9], t[10], t[11], b.side === "back" ? 1 : 0);
      list.refs.push(b.refDes);
      instances.set(m.mesh, list);
      meshed.add(b.refDes);
    }
  }
  const draws: MeshDraw[] = [];
  let triangles = 0;
  let count = 0;
  for (const [key, inst] of instances) {
    const mesh = byKey.get(key);
    if (!mesh) continue;
    const draw = packMesh(mesh);
    if (draw.indices.length === 0) continue;
    draws.push({ ...draw, key, instances: Float32Array.from(inst.floats), refs: inst.refs });
    triangles += (draw.indices.length / 3) * inst.refs.length;
    count += inst.refs.length;
  }
  return { draws, meshed, triangles, instances: count };
}

// packMesh joins a mesh's parts into one vertex list, each vertex carrying its part's colour.
function packMesh(mesh: ModelMesh): { positions: Float32Array; colors: Uint8Array; indices: Uint32Array } {
  let verts = 0;
  let idx = 0;
  for (const p of mesh.parts) {
    verts += Math.floor(p.positions.length / 3);
    idx += p.indices.length;
  }
  const positions = new Float32Array(3 * verts);
  const colors = new Uint8Array(4 * verts);
  const indices = new Uint32Array(idx);
  let v = 0;
  let k = 0;
  for (const p of mesh.parts) {
    const n = Math.floor(p.positions.length / 3);
    positions.set(p.positions.slice(0, 3 * n), 3 * v);
    const rgba = p.rgba >>> 0;
    const c = [(rgba >>> 24) & 255, (rgba >>> 16) & 255, (rgba >>> 8) & 255, rgba & 255];
    for (let i = 0; i < n; i++) colors.set(c, 4 * (v + i));
    for (const i of p.indices) {
      if (i < n) indices[k++] = v + i;
    }
    v += n;
  }
  return { positions, colors, indices: indices.subarray(0, k) };
}

// MESH_SRC places a mesh vertex by its instance's matrix, then raises it off its side's face as a
// body's vertex is raised (stackgl.ts BODY_SRC).
const MESH_SRC = `#version 300 es
in vec3 a_pos;
in vec4 a_col;
in vec4 a_m0;
in vec4 a_m1;
in vec4 a_m2;
in float a_back;
uniform mat4 u_view;
uniform float u_zTop;
uniform float u_zBottom;
uniform float u_hScale;
out vec4 v_col;
out vec3 v_world;
void main() {
  vec4 p = vec4(a_pos, 1.0);
  float h = dot(a_m2, p);
  float z = mix(u_zTop + h * u_hScale, u_zBottom - h * u_hScale, a_back);
  v_world = vec3(dot(a_m0, p), dot(a_m1, p), z);
  v_col = a_col;
  gl_Position = u_view * vec4(v_world, 1.0);
}
`;

// MESH_FRAG_SRC shades each face by its own normal, taken from how the position changes across the
// pixel, so a mesh needs no normals of its own.
const MESH_FRAG_SRC = `#version 300 es
precision highp float;
in vec4 v_col;
in vec3 v_world;
out vec4 outColor;
void main() {
  vec3 n = normalize(cross(dFdx(v_world), dFdy(v_world)));
  float light = 0.4 + 0.6 * abs(dot(n, normalize(vec3(0.35, 0.55, 0.75))));
  outColor = vec4(v_col.rgb * light, v_col.a);
}
`;

// MeshFaces is where the board's faces draw in world z, and the world z one millimetre of height spans.
export interface MeshFaces {
  view: Float32Array;
  zTop: number;
  zBottom: number;
  hScale: number;
}

interface Uploaded {
  vao: WebGLVertexArrayObject;
  count: number;
  instances: number;
}

// StackMeshes draws a MeshGeometry on the board view's WebGL context. Its buffers upload once.
export class StackMeshes {
  private readonly prog: WebGLProgram;
  private readonly buffers: WebGLBuffer[] = [];
  private readonly uploaded: Uploaded[] = [];
  private readonly loc: Record<"view" | "zTop" | "zBottom" | "hScale", WebGLUniformLocation | null>;
  // bytes is what the meshes hold on the GPU.
  readonly bytes: number;

  constructor(
    private readonly gl: WebGL2RenderingContext,
    readonly geometry: MeshGeometry,
  ) {
    this.prog = compile(gl, MESH_SRC, MESH_FRAG_SRC);
    this.loc = {
      view: gl.getUniformLocation(this.prog, "u_view"),
      zTop: gl.getUniformLocation(this.prog, "u_zTop"),
      zBottom: gl.getUniformLocation(this.prog, "u_zBottom"),
      hScale: gl.getUniformLocation(this.prog, "u_hScale"),
    };
    const attr = (name: string) => gl.getAttribLocation(this.prog, name);
    const buffer = (target: number, data: ArrayBufferView): WebGLBuffer => {
      const buf = gl.createBuffer();
      if (!buf) throw new Error("failed to create a mesh buffer");
      gl.bindBuffer(target, buf);
      gl.bufferData(target, data, gl.STATIC_DRAW);
      this.buffers.push(buf);
      return buf;
    };
    let bytes = 0;
    for (const d of geometry.draws) {
      const vao = gl.createVertexArray();
      if (!vao) throw new Error("failed to create a mesh vertex array");
      gl.bindVertexArray(vao);
      buffer(gl.ARRAY_BUFFER, d.positions);
      gl.enableVertexAttribArray(attr("a_pos"));
      gl.vertexAttribPointer(attr("a_pos"), 3, gl.FLOAT, false, 0, 0);
      buffer(gl.ARRAY_BUFFER, d.colors);
      gl.enableVertexAttribArray(attr("a_col"));
      gl.vertexAttribPointer(attr("a_col"), 4, gl.UNSIGNED_BYTE, true, 0, 0);
      buffer(gl.ARRAY_BUFFER, d.instances);
      const stride = MESH_FLOATS * 4;
      ["a_m0", "a_m1", "a_m2"].forEach((name, row) => {
        gl.enableVertexAttribArray(attr(name));
        gl.vertexAttribPointer(attr(name), 4, gl.FLOAT, false, stride, row * 16);
        gl.vertexAttribDivisor(attr(name), 1);
      });
      gl.enableVertexAttribArray(attr("a_back"));
      gl.vertexAttribPointer(attr("a_back"), 1, gl.FLOAT, false, stride, 48);
      gl.vertexAttribDivisor(attr("a_back"), 1);
      buffer(gl.ELEMENT_ARRAY_BUFFER, d.indices);
      this.uploaded.push({ vao, count: d.indices.length, instances: d.refs.length });
      bytes += d.positions.byteLength + d.colors.byteLength + d.instances.byteLength + d.indices.byteLength;
    }
    gl.bindVertexArray(null);
    gl.bindBuffer(gl.ARRAY_BUFFER, null);
    this.bytes = bytes;
  }

  // draw draws every mesh, one instanced call each, and returns the calls it made.
  draw(f: MeshFaces): number {
    const gl = this.gl;
    gl.useProgram(this.prog);
    gl.uniformMatrix4fv(this.loc.view, false, f.view);
    gl.uniform1f(this.loc.zTop, f.zTop);
    gl.uniform1f(this.loc.zBottom, f.zBottom);
    gl.uniform1f(this.loc.hScale, f.hScale);
    for (const u of this.uploaded) {
      gl.bindVertexArray(u.vao);
      gl.drawElementsInstanced(gl.TRIANGLES, u.count, gl.UNSIGNED_INT, 0, u.instances);
    }
    gl.bindVertexArray(null);
    return this.uploaded.length;
  }

  dispose(): void {
    const gl = this.gl;
    for (const b of this.buffers) gl.deleteBuffer(b);
    for (const u of this.uploaded) gl.deleteVertexArray(u.vao);
    gl.deleteProgram(this.prog);
  }
}

function compile(gl: WebGL2RenderingContext, vert: string, frag: string): WebGLProgram {
  const shader = (type: number, src: string) => {
    const sh = gl.createShader(type);
    if (!sh) throw new Error("createShader failed");
    gl.shaderSource(sh, src);
    gl.compileShader(sh);
    if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) throw new Error(`mesh shader compile failed: ${gl.getShaderInfoLog(sh)}`);
    return sh;
  };
  const vs = shader(gl.VERTEX_SHADER, vert);
  const fs = shader(gl.FRAGMENT_SHADER, frag);
  const prog = gl.createProgram();
  if (!prog) throw new Error("createProgram failed");
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.linkProgram(prog);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) throw new Error(`mesh program link failed: ${gl.getProgramInfoLog(prog)}`);
  gl.deleteShader(vs);
  gl.deleteShader(fs);
  return prog;
}

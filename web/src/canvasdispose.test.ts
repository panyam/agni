// @vitest-environment jsdom
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { CanvasComponent } from "./canvas.js";
import { PackedSheetSchema, type PackedSheet } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { GROUP_WIRE, KIND_LINE_STRIP } from "./packed.js";
import { fakeGl, type FakeGl } from "./fakegl.js";
import { keyColumns } from "./testpacked.js";

// Each sheet switch built a new Renderer on the same context and left the old one's buffers, VAO and
// program on the GPU (agni issue 1331).

// frames holds the requested animation frames, which this test never runs.
let frames: FrameRequestCallback[] = [];

// wire is a sheet of one wire from (0,0) to (1000,1000), keyed to net N1.
function wire(): PackedSheet {
  const v = new ArrayBuffer(16);
  new Int32Array(v).set([0, 0, 1000, 1000]);
  const p = new DataView(new ArrayBuffer(12));
  p.setUint8(0, KIND_LINE_STRIP);
  p.setUint8(1, GROUP_WIRE);
  p.setUint32(4, 0, true);
  p.setUint32(8, 2, true);
  return create(PackedSheetSchema, {
    sheetId: "S",
    vertices: new Uint8Array(v),
    primitives: new Uint8Array(p.buffer),
    keyColumns: keyColumns([{ primitive: 0, net: "N1" }]),
    groupColors: ["#000000", "#00aa00"],
    backgroundColor: "#ffffff",
  });
}

function mount(): { c: CanvasComponent; f: FakeGl; canvas: HTMLCanvasElement } {
  const f = fakeGl();
  const canvas = document.createElement("canvas");
  canvas.getContext = (() => f.gl) as unknown as HTMLCanvasElement["getContext"];
  document.body.appendChild(canvas);
  const c = new CanvasComponent("c", canvas);
  c.textHost = document.createElement("div");
  c.readout = null;
  c.activate();
  return { c, f, canvas };
}

describe("CanvasComponent's sheet switch", () => {
  beforeEach(() => {
    frames = [];
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => (frames.push(cb), frames.length));
    vi.stubGlobal("cancelAnimationFrame", () => {});
  });
  afterEach(() => vi.unstubAllGlobals());

  it("deletes the last sheet's GPU objects when a new sheet replaces it", () => {
    const { c, f } = mount();
    c.showSheet(wire());
    expect(f.count("deleteBuffer")).toBe(0);
    c.showSheet(wire());
    const made = (name: string) => f.calls.filter((x) => x.name === name).map((x) => x.ret);
    const freed = (name: string) => f.calls.filter((x) => x.name === name).map((x) => x.args[0]);
    expect(freed("deleteBuffer")).toEqual([made("createBuffer")[0]]);
    expect(freed("deleteVertexArray")).toEqual([made("createVertexArray")[0]]);
    expect(freed("deleteProgram")).toEqual([made("createProgram")[0]]);
  });
});

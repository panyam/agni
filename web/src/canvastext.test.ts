// @vitest-environment jsdom
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { CanvasComponent } from "./canvas.js";
import { PackedSheetSchema, type PackedSheet } from "./gen/agni/v1/geom/geom_packed_pb.js";
import { GROUP_WIRE, KIND_LINE_STRIP } from "./packed.js";
import { fakeGl } from "./fakegl.js";

// agni issue 1503: a sheet's copper paints before its text. A board's labels, 5,732 on Jetson, took
// about 12 ms to lay out ahead of the first frame.

let frames: FrameRequestCallback[] = [];

// labelled is a sheet of one wire from (0,0) to (1000,1000) and one label reading text.
function labelled(text: string): PackedSheet {
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
    keys: [{ primitive: 0, net: "N1" }],
    labels: [{ x: 10, y: 10, text, height: 50, justify: "left bottom", color: "#000000" }],
    groupColors: ["#000000", "#00aa00"],
    backgroundColor: "#ffffff",
  });
}

function mount(): { c: CanvasComponent; text: HTMLElement } {
  const f = fakeGl();
  const canvas = document.createElement("canvas");
  canvas.getContext = (() => f.gl) as unknown as HTMLCanvasElement["getContext"];
  Object.defineProperty(canvas, "clientWidth", { value: 800 });
  Object.defineProperty(canvas, "clientHeight", { value: 600 });
  document.body.appendChild(canvas);
  const c = new CanvasComponent("c", canvas);
  const text = document.createElement("div");
  c.textHost = text;
  c.readout = null;
  c.activate();
  return { c, text };
}

// runFrames runs the animation frames requested so far, and the ones they request, up to n rounds.
function runFrames(n: number): void {
  for (let i = 0; i < n; i++) {
    const now = frames;
    frames = [];
    for (const cb of now) cb(performance.now());
  }
}

const words = (el: HTMLElement) => [...el.querySelectorAll("text")].map((t) => t.textContent);

describe("a sheet's text layer", () => {
  beforeEach(() => {
    frames = [];
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => (frames.push(cb), frames.length));
    vi.stubGlobal("cancelAnimationFrame", () => {});
  });
  afterEach(() => vi.unstubAllGlobals());

  it("fills after the sheet's first frame, not before it", () => {
    const { c, text } = mount();
    c.showSheet(labelled("R1"));
    expect(words(text)).toEqual([]);
    runFrames(3);
    expect(words(text)).toEqual(["R1"]);
  });

  it("fills with the sheet on screen when another replaced it before the first frame", () => {
    const { c, text } = mount();
    c.showSheet(labelled("OLD"));
    c.showSheet(labelled("NEW"));
    runFrames(3);
    expect(words(text)).toEqual(["NEW"]);
  });
});

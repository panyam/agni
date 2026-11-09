// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { SheetPane, parseTurn, type SheetPaneDeps } from "./sheetpane.js";
import type { Selection } from "./selection.js";
import type { HighlightSpec } from "./highlights.js";
import type { BoardLayerRow } from "./gen/agni/v1/geom/geom_packed_pb.js";

// ROWS are a board's layer rows as the engine lists them, two inner layers between the outer pair.
const ROWS = ["F.Cu", "In1.Cu", "In2.Cu", "B.Cu"].map(
  (name) => ({ name, kind: "copper", color: "#888", svgSelector: `[data-layer="${name}"]`, groups: [] }) as unknown as BoardLayerRow,
);

const SHEETS = [
  { id: "s1", name: "Root" },
  { id: "s2", name: "Power" },
  { id: "s3", name: "IO" },
  { id: "board", name: "Board" },
];

let panes = 0;

function pane(drawnOn: string[]) {
  const calls = { render: [] as string[], overlay: [] as string[], drawnOn: [] as boolean[] };
  const deps: SheetPaneDeps = {
    sheets: () => SHEETS,
    render: async (id) => {
      calls.render.push(id);
      return { svg: `<svg width="100" height="100" data-sheet="${id}"></svg>`, boardLayers: id === "board" ? ROWS : [] };
    },
    overlay: async (id) => {
      calls.overlay.push(id);
      return `<svg width="100" height="100"><circle r="1"/></svg>`;
    },
    drawnOn: async (_sel, board) => {
      calls.drawnOn.push(board);
      return drawnOn;
    },
    specs: () => [{ components: ["U1"] } as unknown as HighlightSpec],
    onPick: vi.fn(),
    onPickEmpty: vi.fn(),
    onViewChange: vi.fn(),
    onFloat: vi.fn(),
    onClose: vi.fn(),
    onOpened: vi.fn(),
  };
  const p = new SheetPane(`sheetview-${++panes}`, deps);
  document.body.appendChild(p.element);
  return { p, calls, deps };
}

const U1: Selection = { kind: "component", ref: "U1" };
const chips = (p: SheetPane) => [...p.element.querySelectorAll(".sheetview-chip")].map((b) => b.textContent);
const shown = (p: SheetPane) => p.element.querySelector(".sheetview-canvas svg")?.getAttribute("data-sheet");

describe("a sheet view", () => {
  it("draws the sheet it opens, names it in the dock and in its picker", async () => {
    const { p, deps } = pane([]);
    await p.open("s2");
    expect(shown(p)).toBe("s2");
    expect(p.title()).toBe("Power");
    expect((p.element.querySelector(".sheetview-sheet") as HTMLSelectElement).value).toBe("s2");
    expect(deps.onOpened).toHaveBeenCalledWith(p);
  });

  it("switches to the sheet that draws a selection its own does not, and lights it", async () => {
    const { p, calls } = pane(["s3"]);
    await p.open("s1");
    await p.follow(U1);
    expect(shown(p)).toBe("s3");
    expect(calls.overlay[calls.overlay.length - 1]).toBe("s3");
    expect(p.element.querySelector(".highlight-overlay svg")).not.toBeNull();
    expect(chips(p)).toEqual([]);
  });

  it("stays on a sheet that draws it, and offers the other sheets", async () => {
    const { p, calls } = pane(["s1", "s3"]);
    await p.open("s1");
    await p.follow(U1);
    expect(shown(p)).toBe("s1");
    expect(calls.render).toEqual(["s1"]);
    expect(chips(p)).toEqual(["IO"]);
  });

  it("opens an offered sheet from its chip, and then offers the sheet it left", async () => {
    const { p } = pane(["s1", "s3"]);
    await p.open("s1");
    await p.follow(U1);
    (p.element.querySelector(".sheetview-chip") as HTMLButtonElement).click();
    await vi.waitFor(() => expect(shown(p)).toBe("s3"));
    await vi.waitFor(() => expect(chips(p)).toEqual(["Root"]));
  });

  it("asks about the board only while it shows the board", async () => {
    const { p, calls } = pane(["board"]);
    await p.open("s1");
    await p.follow(U1);
    await p.open("board");
    await p.follow(U1);
    expect(calls.drawnOn).toEqual([false, true]);
  });

  it("lights a selection where it is drawn, and never moves, when its link is off", async () => {
    const { p, calls } = pane(["s3"]);
    await p.open("s1");
    p.link = "off";
    await p.follow(U1);
    expect(shown(p)).toBe("s1");
    expect(calls.drawnOn).toEqual([]);
    expect(calls.overlay).toEqual(["s1"]);
  });

  it("lights a pick made in it without asking where else it is drawn", async () => {
    const { p, calls } = pane(["s3"]);
    await p.open("s1");
    await p.lightPick(U1);
    expect(shown(p)).toBe("s1");
    expect(calls.drawnOn).toEqual([]);
    expect(calls.overlay).toEqual(["s1"]);
  });

  it("drops an answer for a selection the reader has already replaced", async () => {
    const { p, deps } = pane([]);
    let release: (v: string[]) => void = () => {};
    deps.drawnOn = () => new Promise((r) => (release = r));
    await p.open("s1");
    const first = p.follow(U1);
    const second = p.lightPick({ kind: "component", ref: "R1" });
    release(["s3"]);
    await Promise.all([first, second]);
    expect(shown(p)).toBe("s1");
  });

  it("offers its own board layer list on the board alone", async () => {
    const { p } = pane([]);
    const button = p.element.querySelector('[data-action="layers"]') as HTMLButtonElement;
    const pop = p.element.querySelector(".sheetview-layer-pop") as HTMLElement;
    await p.open("s1");
    expect(button.style.display).toBe("none");
    expect(p.element.querySelector(".sheetview-layers select, select.sheetview-layers")).toBeNull();
    await p.open("board");
    expect(button.style.display).toBe("");
    expect(pop.hidden).toBe(true);
    button.click();
    expect(pop.hidden).toBe(false);
    expect([...pop.querySelectorAll(".board-layer-name")].map((n) => n.textContent)).toEqual(["F.Cu", "In1.Cu", "In2.Cu", "B.Cu"]);
    await p.open("s1");
    expect(pop.hidden).toBe(true);
  });

  // agni issue 1401: a view beside the canvas hides a layer in its own drawing alone.
  it("hides a layer in its own drawing and no other", async () => {
    const { p } = pane([]);
    const { p: other } = pane([]);
    await p.open("board");
    await other.open("board");
    (p.element.querySelector('[data-action="layers"]') as HTMLButtonElement).click();
    const row = [...p.element.querySelectorAll(".board-layer-row")].find((r) => r.textContent === "In1.Cu") as HTMLElement;
    row.click();
    expect(p.hiddenLayersOf()).toEqual(new Set(["In1.Cu"]));
    expect(p.element.querySelector("style")!.textContent).toBe(`[data-sheetview="${p.id}"] .sheetview-canvas :is([data-layer="In1.Cu"]) { display: none; }`);
    expect((row.querySelector("input") as HTMLInputElement).checked).toBe(false);
    expect(other.hiddenLayersOf().size).toBe(0);
    expect(other.element.querySelector("style")!.textContent).toBe("");
    // A schematic sheet in the same view takes no board rule.
    await p.open("s1");
    expect(p.element.querySelector("style")!.textContent).toBe("");
  });
});

describe("turning a sheet view", () => {
  function turned(saved: Record<string, string> = {}) {
    const store = new Map(Object.entries(saved));
    const t = pane([]);
    t.deps.turns = {
      load: (id) => parseTurn(store.get(id) ?? null),
      save: (id, turn) => void store.set(id, JSON.stringify(turn)),
    };
    return { ...t, store };
  }
  const orientOf = (p: SheetPane) => (p.element.querySelector(".sheetview-canvas > div > div") as HTMLElement).style.transform;

  it("turns and flips the drawing, and keeps the turn for the sheet", async () => {
    const { p, store } = turned();
    await p.open("board");
    (p.element.querySelector(".sheetview-rotate") as HTMLButtonElement).click();
    (p.element.querySelector(".sheetview-flip-h") as HTMLButtonElement).click();
    expect(p.turnOf().orientation).toEqual({ rot: 90, flipH: false, flipV: true });
    expect(orientOf(p)).toContain("rotate(90deg)");
    expect(parseTurn(store.get("board") ?? null)?.orientation.rot).toBe(90);
  });

  it("opens a sheet the way it was last turned", async () => {
    const { p } = turned({ board: JSON.stringify({ orientation: { rot: 270, flipH: true, flipV: false }, bottom: true }) });
    await p.open("board");
    expect(p.turnOf()).toEqual({ orientation: { rot: 270, flipH: true, flipV: false }, bottom: true });
    expect(orientOf(p)).toContain("rotate(270deg)");
  });

  it("turns the board over left to right and draws it from below in a bottom view", async () => {
    const { p, deps } = turned();
    deps.render = async () => ({ svg: `<svg width="100" height="100"><g class="copper-back"></g><g class="copper-front"></g><g class="labels"></g></svg>`, boardLayers: [] });
    await p.open("board");
    (p.element.querySelector(".sheetview-bottom") as HTMLButtonElement).click();
    expect(p.turnOf()).toEqual({ orientation: { rot: 0, flipH: true, flipV: false }, bottom: true });
    const groups = () => [...p.element.querySelector(".sheetview-canvas svg")!.children].map((g) => g.getAttribute("class"));
    expect(groups()).toEqual(["copper-front", "copper-back", "labels"]);
    (p.element.querySelector(".sheetview-bottom") as HTMLButtonElement).click();
    expect(groups()).toEqual(["copper-back", "copper-front", "labels"]);
  });

  it("offers the bottom view on the board alone", async () => {
    const { p } = turned();
    await p.open("s1");
    expect((p.element.querySelector(".sheetview-bottom") as HTMLElement).style.display).toBe("none");
  });

  it("reads back only a turn it wrote", () => {
    expect(parseTurn(null)).toBeNull();
    expect(parseTurn("{not json")).toBeNull();
    expect(parseTurn(JSON.stringify({ orientation: { rot: 45, flipH: false, flipV: false } }))).toBeNull();
    expect(parseTurn(JSON.stringify({ orientation: { rot: 90, flipH: false, flipV: true }, bottom: true }))).toEqual({ orientation: { rot: 90, flipH: false, flipV: true }, bottom: true });
  });
});

// The cross-probe walk turns a board view by these names and reads its turn back from them
// (web/browser/cross-probe-walk.spec.ts).
describe("the names a turned view answers to", () => {
  it("names each turn control, and states its rotation and the side nearest the reader", async () => {
    const { p } = pane([]);
    await p.open("board");
    const act = (a: string) => p.element.querySelector(`[data-action="${a}"]`) as HTMLButtonElement;
    for (const a of ["rotate", "flip-h", "flip-v", "bottom"]) expect(act(a), a).not.toBeNull();
    expect([p.element.dataset.rotation, p.element.dataset.side]).toEqual(["0", "front"]);
    act("rotate").click();
    act("bottom").click();
    expect([p.element.dataset.rotation, p.element.dataset.side]).toEqual(["90", "back"]);
  });
});

// fakeGl stands in for a WebGL drawing, which jsdom cannot make, and records what the view asks of it.
interface FakeGl {
  sheet: string;
  specs: HighlightSpec[];
  strata: { rows: number; fromBelow: boolean };
  orientation: { rot: number; flipH: boolean; flipV: boolean };
  [method: string]: unknown;
}

function fakeGl() {
  const calls: string[] = [];
  const gl: FakeGl = {
    markView: "",
    onPick: null,
    onPickEmpty: null,
    onArea: null,
    onToggle: null,
    onViewChange: null,
    sheet: "",
    specs: [] as HighlightSpec[],
    strata: { rows: 0, fromBelow: false },
    orientation: { rot: 0, flipH: false, flipV: false },
    showSheet: (s: { sheetId: string }) => {
      gl.sheet = s.sheetId;
      calls.push(`draw ${s.sheetId}`);
    },
    setHighlights: (specs: HighlightSpec[]) => (gl.specs = specs),
    setHiddenGroups: () => {},
    setOrientation: (o: typeof gl.orientation) => (gl.orientation = o),
    setStrata: (rows: unknown[], fromBelow: boolean) => (gl.strata = { rows: rows.length, fromBelow }),
    showText: () => {},
    hideText: () => {},
    reveal: (pin?: { ref: string; pin: string }) => calls.push(`reveal ${pin ? `${pin.ref}.${pin.pin}` : ""}`),
    getView: () => null,
    setView: () => {},
    getCentre: () => ({ at: { x: 1, y: 2 }, scale: 3 }),
    setCentre: () => {},
    drawn: () => null,
    membersIn: () => null,
    dispose: () => calls.push("dispose"),
  };
  return { gl, calls };
}

// A view draws in WebGL when its reader picks it (agni issue 1368), from the packed sheet, and lights
// a selection against the keys it holds rather than fetching an overlay.
describe("a sheet view in WebGL", () => {
  function glPane(gl: FakeGl | null, drawnOn: string[] = ["board"]) {
    const asked: string[] = [];
    let overlays = 0;
    const deps: SheetPaneDeps = {
      sheets: () => SHEETS,
      render: async (id, mode) => {
        asked.push(`${id} ${mode}`);
        const packed = mode === "webgl" ? ({ sheetId: id } as never) : undefined;
        return { svg: `<svg width="100" height="100" data-sheet="${id}"></svg>`, packed, boardLayers: id === "board" ? ROWS : [] };
      },
      overlay: async () => {
        overlays++;
        return `<svg width="100" height="100"><circle r="1"/></svg>`;
      },
      drawnOn: async () => drawnOn,
      specs: () => [{ components: ["U1"] } as unknown as HighlightSpec],
      onPick: vi.fn(),
      onPickEmpty: vi.fn(),
      onViewChange: vi.fn(),
      onFloat: vi.fn(),
      onClose: vi.fn(),
      onOpened: vi.fn(),
      renderer: () => "webgl",
      gl: () => gl as never,
    };
    const p = new SheetPane(`sheetview-${++panes}`, deps);
    document.body.appendChild(p.element);
    return { p, asked, overlays: () => overlays };
  }

  it("draws the packed sheet, lights a selection itself and frames the pin", async () => {
    const { gl, calls } = fakeGl();
    const { p, asked, overlays } = glPane(gl);
    await p.open("board");
    expect(asked).toEqual(["board webgl"]);
    expect(gl.sheet).toBe("board");
    expect(p.element.dataset.renderer).toBe("webgl");
    expect(p.element.querySelector<HTMLSelectElement>(".sheetview-renderer")?.value).toBe("webgl");
    await p.follow({ kind: "pin", ref: "U1", pin: "3" });
    expect(gl.specs).toEqual([{ components: ["U1"] }]);
    expect(overlays()).toBe(0);
    expect(calls).toContain("reveal U1.3");
  });

  it("draws in SVG where the browser has no WebGL2, and says so", async () => {
    const { p, asked } = glPane(null);
    await p.open("s1");
    expect(asked).toEqual(["s1 webgl", "s1 svg"]);
    expect(p.element.dataset.renderer).toBe("svg");
    expect(p.element.querySelector<HTMLSelectElement>(".sheetview-renderer")?.value).toBe("svg");
    expect(shown(p)).toBe("s1");
  });

  it("turns a board over in WebGL, back copper nearest, and frees the drawing when it leaves WebGL", async () => {
    const { gl, calls } = fakeGl();
    const { p, asked } = glPane(gl);
    await p.open("board");
    p.element.querySelector<HTMLButtonElement>('[data-action="bottom"]')!.click();
    expect(gl.strata).toEqual({ rows: ROWS.length, fromBelow: true });
    expect(gl.orientation.flipH).toBe(true);
    await p.setRenderer("svg");
    expect(calls).toContain("dispose");
    expect(asked).toEqual(["board webgl", "board svg"]);
    expect(p.element.dataset.renderer).toBe("svg");
  });
});

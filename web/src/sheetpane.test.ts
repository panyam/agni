// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { SheetPane, type SheetPaneDeps } from "./sheetpane.js";
import type { Selection } from "./selection.js";
import type { HighlightSpec } from "./highlights.js";

const SHEETS = [
  { id: "s1", name: "Root" },
  { id: "s2", name: "Power" },
  { id: "s3", name: "IO" },
  { id: "board", name: "Board" },
];

function pane(drawnOn: string[]) {
  const calls = { render: [] as string[], overlay: [] as string[], drawnOn: [] as boolean[] };
  const deps: SheetPaneDeps = {
    sheets: () => SHEETS,
    render: async (id) => {
      calls.render.push(id);
      return `<svg width="100" height="100" data-sheet="${id}"></svg>`;
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
  const p = new SheetPane("sheetview-1", deps);
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

  it("shows the board layer control on the board alone, and hides a side by class", async () => {
    const { p } = pane([]);
    const layers = p.element.querySelector(".sheetview-layers") as HTMLSelectElement;
    await p.open("s1");
    expect(layers.style.display).toBe("none");
    await p.open("board");
    expect(layers.style.display).toBe("");
    layers.value = "front";
    layers.dispatchEvent(new Event("change"));
    expect(p.element.querySelector(".sheetview-canvas")!.classList.contains("board-front")).toBe(true);
  });
});

// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { SheetViews, type SheetViewDock } from "./sheetviews.js";
import type { Selection } from "./selection.js";

function setup(drawnOn: string[] = ["s1"], modes: { canvas?: "svg" | "webgl"; board?: "svg" | "webgl" } = {}) {
  const docked = new Map<string, () => void>();
  const dock: SheetViewDock = {
    add: (id, _t, el, closed) => {
      document.body.appendChild(el);
      docked.set(id, closed);
    },
    setTitle: vi.fn(),
    float: vi.fn(),
    close: (id) => {
      docked.get(id)?.();
      docked.delete(id);
    },
  };
  const asked: string[] = [];
  const rendered: string[] = [];
  const lit: { sheet: string; specs: unknown[] }[] = [];
  const views = new SheetViews(
    dock,
    {
      sheets: () => [{ id: "s1", name: "Root" }, { id: "board", name: "Board" }],
      render: async (id, mode) => {
        rendered.push(`${id}:${mode}`);
        return { svg: `<svg width="100" height="100" data-sheet="${id}"></svg>`, boardLayers: [] };
      },
      renderer: modes.canvas ? () => modes.canvas! : undefined,
      boardRenderer: modes.board ? () => modes.board! : undefined,
      overlay: async (sheet, specs) => {
        lit.push({ sheet, specs });
        return "<svg width='100' height='100'><circle r='1'/></svg>";
      },
      drawnOn: async (sel) => {
        asked.push(sel.ref ?? "");
        return drawnOn;
      },
      specs: () => [{}] as never,
    },
    { onPick: vi.fn(), onPickEmpty: vi.fn() },
  );
  return { views, dock, docked, asked, lit, rendered };
}

const U1: Selection = { kind: "component", ref: "U1" };

describe("the sheet views beside the canvas", () => {
  // A board view opens in WebGL on a hardware GPU beside an SVG canvas, and a schematic view keeps
  // the canvas's SVG (agni issue 1536).
  it("opens a board view in the board renderer and a schematic view in the canvas's", async () => {
    const { views, rendered } = setup(["s1"], { canvas: "svg", board: "webgl" });
    const board = await views.open("board");
    const sheet = await views.open("s1");
    expect(rendered[0]).toBe("board:webgl");
    expect(rendered[rendered.length - 1]).toBe("s1:svg");
    expect(sheet.renderer()).toBe("svg");
    expect(board.id).toBe("sheetview-1");
  });

  it("opens a board view in WebGL beside a WebGL canvas, even on a software GPU", async () => {
    const { views, rendered } = setup(["s1"], { canvas: "webgl", board: "svg" });
    await views.open("board");
    expect(rendered[0]).toBe("board:webgl");
  });

  it("keeps a board view in SVG on a software GPU, and lets a caller name the renderer", async () => {
    const soft = setup(["s1"], { canvas: "svg", board: "svg" });
    await soft.views.open("board");
    expect(soft.rendered).toEqual(["board:svg"]);
    const named = setup(["s1"], { canvas: "svg", board: "webgl" });
    await named.views.open("board", "svg");
    expect(named.rendered).toEqual(["board:svg"]);
  });

  it("opens views with their own ids and docks each one", async () => {
    const { views, docked } = setup();
    const a = await views.open("s1");
    const b = await views.open("board");
    expect([a.id, b.id]).toEqual(["sheetview-1", "sheetview-2"]);
    expect([...docked.keys()]).toEqual(["sheetview-1", "sheetview-2"]);
  });

  // A walk lights every view beside the canvas, and a view opened during the walk (agni issue 1480).
  it("lights the page's underlay in every open view and in a view opened after it", async () => {
    const { views, lit } = setup();
    await views.open("s1");
    await views.open("board");
    const walk = { nets: ["SDA"] };
    await views.setUnderlay([walk] as never);
    expect(lit.filter((l) => l.specs.includes(walk)).map((l) => l.sheet).sort()).toEqual(["board", "s1"]);
    lit.length = 0;
    await views.open("board");
    expect(lit.filter((l) => l.specs.includes(walk)).map((l) => l.sheet)).toEqual(["board"]);
    views.reset();
    lit.length = 0;
    await views.open("s1");
    expect(lit).toEqual([]);
  });

  it("lets the view a pick came from light it, and the others follow it", async () => {
    const { views, asked } = setup(["s1"]);
    const a = await views.open("s1");
    await views.open("board");
    await views.follow(U1, a.id);
    expect(asked).toEqual(["U1"]);
  });

  it("mirrors a locked view's camera onto a locked view on the same sheet, and no other", async () => {
    const { views } = setup();
    const a = await views.open("board");
    const b = await views.open("board");
    const c = await views.open("s1");
    for (const p of [a, b, c]) p.link = "locked";
    const cBefore = c.view.getView();
    a.view.setView({ tx: 12, ty: -3, scale: 4 });
    a.view.onViewChange?.(a.view.getView());
    // A locked peer takes the drawing point at the centre and the zoom, which line up two views of
    // one sheet whatever their sizes or orientations (agni issue 1327).
    expect(b.view.getCentre()).toEqual(a.view.getCentre());
    expect(c.view.getView()).toEqual(cBefore);
  });

  it("closes a view by its id, and refuses an id that names no open view", async () => {
    const { views, docked } = setup();
    const a = await views.open("s1");
    const b = await views.open("board");
    expect(views.get(b.id)).toBe(b);
    expect(views.close(a.id)).toBe(true);
    expect(views.list().map((p) => p.id)).toEqual([b.id]);
    expect([...docked.keys()]).toEqual([b.id]);
    expect(views.get(a.id)).toBeUndefined();
    expect(views.close(a.id)).toBe(false);
    expect(views.close("sheetview-9")).toBe(false);
    expect(views.list().map((p) => p.id)).toEqual([b.id]);
  });

  it("undocks and forgets every view on reset, as opening another design asks (agni issue 1425)", async () => {
    const { views, docked } = setup();
    await views.open("s1");
    await views.open("board");
    expect(views.list()).toHaveLength(2);
    views.reset();
    expect(views.list()).toEqual([]);
    expect([...docked.keys()]).toEqual([]);
  });

  it("forgets a view the dock closes", async () => {
    const { views, dock } = setup();
    const a = await views.open("s1");
    dock.close(a.id);
    expect(views.list()).toEqual([]);
  });
});

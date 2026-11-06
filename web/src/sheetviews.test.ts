// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { SheetViews, type SheetViewDock } from "./sheetviews.js";
import type { Selection } from "./selection.js";

function setup(drawnOn: string[] = ["s1"]) {
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
  const views = new SheetViews(
    dock,
    {
      sheets: () => [{ id: "s1", name: "Root" }, { id: "board", name: "Board" }],
      render: async (id) => `<svg width="100" height="100" data-sheet="${id}"></svg>`,
      overlay: async () => "<svg width='100' height='100'><circle r='1'/></svg>",
      drawnOn: async (sel) => {
        asked.push(sel.ref ?? "");
        return drawnOn;
      },
      specs: () => [{}] as never,
    },
    { onPick: vi.fn(), onPickEmpty: vi.fn() },
  );
  return { views, dock, docked, asked };
}

const U1: Selection = { kind: "component", ref: "U1" };

describe("the sheet views beside the canvas", () => {
  it("opens views with their own ids and docks each one", async () => {
    const { views, docked } = setup();
    const a = await views.open("s1");
    const b = await views.open("board");
    expect([a.id, b.id]).toEqual(["sheetview-1", "sheetview-2"]);
    expect([...docked.keys()]).toEqual(["sheetview-1", "sheetview-2"]);
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

  it("forgets a view the dock closes", async () => {
    const { views, dock } = setup();
    const a = await views.open("s1");
    dock.close(a.id);
    expect(views.list()).toEqual([]);
  });
});

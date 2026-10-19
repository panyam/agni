import { describe, expect, it, vi } from "vitest";
import { LoadFlow, type LoadDeps, type LoadStep } from "./loadflow.js";

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function harness(over: Partial<LoadDeps> = {}) {
  const pushed: LoadStep[][] = [];
  const failed: string[] = [];
  const order: string[] = [];
  const deps: LoadDeps = {
    uri: "mount://m/b.kicad_sch",
    fetches: true,
    listing: async () => ({ stamp: "st", files: 2, bytes: 3 << 20 }),
    drawing: vi.fn(async () => void order.push("drawing")),
    board: vi.fn(async () => (order.push("board"), { boardRead: true, boardPersisted: true })),
    record: vi.fn((s: string) => void order.push(`record ${s}`)),
    open: vi.fn(() => void order.push("open")),
    now: (() => {
      let t = 0;
      return () => (t += 100);
    })(),
    log: vi.fn(),
    ...over,
  };
  const flow = new LoadFlow(deps, { steps: (s) => pushed.push(s), failed: (r) => failed.push(r) });
  const last = () => pushed[pushed.length - 1] ?? flow.steps();
  return { flow, deps, pushed, failed, order, last };
}

// agni issue 1072: the Load page reads a design's drawing, netlist and board into the store, records
// what it read and opens the viewer, and says at each step how far it got.
describe("the Load page's flow", () => {
  it("reads the drawing, then the board, then records the read and opens the viewer", async () => {
    const h = harness();
    expect(await h.flow.run()).toBe(true);
    expect(h.order).toEqual(["drawing", "board", "record st", "open"]);
    expect(h.last().map((s) => [s.id, s.state])).toEqual([["files", "done"], ["drawing", "done"], ["board", "done"]]);
    // The public demo's walk reads this line.
    expect(h.deps.log).toHaveBeenCalledWith("agni loaded mount://m/b.kicad_sch in 100 ms: board persisted");
  });

  it("counts the files as they arrive, each once", async () => {
    const board = deferred<{ boardRead: boolean; boardPersisted: boolean }>();
    const h = harness({ board: () => board.promise });
    const running = h.flow.run();
    await vi.waitFor(() => expect(h.last().find((s) => s.id === "board")?.state).toBe("running"));
    h.flow.fetched("b.kicad_sch", 1 << 20);
    h.flow.fetched("b.kicad_sch", 1 << 20); // the same file again, for another worker
    expect(h.last().find((s) => s.id === "files")?.detail).toBe("1 of 2 files, 1.0 of 3.0 MB");
    board.resolve({ boardRead: false, boardPersisted: false });
    await running;
    expect(h.last().find((s) => s.id === "board")?.detail).toBe("this design has no board");
  });

  it("shows no files step where the page fetches nothing", () => {
    expect(harness({ fetches: false }).flow.steps().map((s) => s.id)).toEqual(["drawing", "board"]);
  });

  // A design that cannot be read stays on the Load page with the reason, rather than opening a viewer
  // that would fail at its first question.
  it("stops at the step that failed, says why, and neither records nor opens", async () => {
    const h = harness({ board: async () => { throw new Error("no netlist in b.kicad_sch"); } });
    expect(await h.flow.run()).toBe(false);
    expect(h.failed).toEqual(["no netlist in b.kicad_sch"]);
    expect(h.last().find((s) => s.id === "board")?.state).toBe("failed");
    expect(h.last().find((s) => s.id === "drawing")?.state).toBe("done");
    expect(h.deps.record).not.toHaveBeenCalled();
    expect(h.deps.open).not.toHaveBeenCalled();
  });
});

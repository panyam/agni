import { describe, it, expect, vi } from "vitest";
import { artifactUri, uriPath } from "./uri.js";
import { DesignPreview, captionFor, pickPreviewSheet, type PreviewView } from "./preview.js";
import { SheetFormat } from "./gen/agni/v1/webapi/design_pb.js";

// Fakes follow the diffpresenter.test.ts harness: plain stubs behind the typed client and a
// recording view. defer lets a test hold a response open so an out-of-order landing is reachable.
function harness(over: { getDesign?: ReturnType<typeof vi.fn>; getSheet?: ReturnType<typeof vi.fn> } = {}) {
  const getDesign =
    over.getDesign ??
    vi.fn(async (_req: object) => ({
      name: "Amplifier",
      sourceFormat: "kicad-sch",
      componentCount: 12,
      netCount: 9,
      sheets: [
        { id: "s1", name: "Top" },
        { id: "s2", name: "Power" },
      ],
    }));
  const getSheet =
    over.getSheet ??
    vi.fn(async (req: { sheet: string }) => ({
      content: { case: "svg" as const, value: `<svg data-sheet="${req.sheet}"/>` },
    }));

  const showSvg = vi.fn();
  const showNote = vi.fn();
  const setCaption = vi.fn();
  const view: PreviewView = { showSvg, showNote, setCaption };
  const client = { getDesign, getSheet } as unknown as ConstructorParameters<typeof DesignPreview>[0];
  return { preview: new DesignPreview(client, view), getDesign, getSheet, showSvg, showNote, setCaption };
}

// deferred is a promise a test resolves by hand, so two loads can be landed in a chosen order.
function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void } {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

describe("DesignPreview", () => {
  it("renders the first sheet as SVG under the server's default layout", async () => {
    const h = harness();
    await h.preview.show("corpus", "boards/amp.kicad_sch");

    // An EMPTY layout is the request that makes the preview faithful-first: the server resolves
    // "" to the faithful layout whenever the file carries geometry. Pinning it stops a later edit
    // from hard-coding a layout and silently drawing every design as a netlist graph.
    expect(h.getDesign).toHaveBeenCalledWith({ uri: artifactUri("corpus", "boards/amp.kicad_sch"), layout: "" });
    expect(h.getSheet).toHaveBeenCalledWith({
      mount: "corpus",
      path: "boards/amp.kicad_sch",
      sheet: "s1",
      layout: "",
      format: SheetFormat.SVG,
    });
    expect(h.showSvg).toHaveBeenCalledWith('<svg data-sheet="s1"/>');
    expect(h.setCaption).toHaveBeenLastCalledWith("Amplifier", "kicad-sch · 12 components · 9 nets · 2 sheets");
  });

  // The board case is why the preview is two calls rather than one, AND why the sheet is chosen
  // rather than taken. Shapes below are the real GetDesign responses, read off the wire from a
  // running server against cmd/agni/testdata/conformance.
  it("previews a board file as its board sheet, not as its netlist graph", async () => {
    const getDesign = vi.fn(async (_req: object) => ({
      name: "Board Geom Fixture",
      sourceFormat: "kicad-pcb",
      componentCount: 2,
      netCount: 2,
      // The board sheet comes LAST: the service appends it after the drawable sheets, so the
      // first sheet of a board file is a synthetic auto-layout of its netlist.
      sheets: [
        { id: "graph", name: "netlist graph" },
        { id: "board", name: "Board" },
      ],
      availableLayouts: ["force", "grid", "layered", "orthogonal", "stress"],
    }));
    const h = harness({ getDesign });
    await h.preview.show("fixtures", "drc.passes.kicad_pcb");

    expect(h.getSheet).toHaveBeenCalledWith(expect.objectContaining({ sheet: "board", format: SheetFormat.SVG }));
    expect(h.showSvg).toHaveBeenCalledWith('<svg data-sheet="board"/>');
  });

  // Clicking down a file list starts a load per file, and a small design behind a large one
  // returns first. The stage must settle on the LAST design asked for, not the last to answer.
  // There are two places a load can go stale, and they need separate cases: a load can still be
  // waiting on GetDesign when a newer one starts, or it can already be waiting on GetSheet. One
  // test only ever reaches whichever guard comes first.
  it("drops a stale load still waiting on the design lookup", async () => {
    const slow = deferred<{ sheets: { id: string; name: string }[] }>();
    const base = { name: "Fast", sourceFormat: "kicad-sch", componentCount: 1, netCount: 1 };
    const getDesign = vi.fn((req: { uri: string }) =>
      uriPath(req.uri) === "slow.kicad_sch" ? slow.promise : Promise.resolve({ ...base, sheets: [{ id: "f1", name: "Fast" }] }),
    );
    const h = harness({ getDesign });

    const first = h.preview.show("corpus", "slow.kicad_sch");
    const second = h.preview.show("corpus", "fast.kicad_sch");
    await second;
    slow.resolve({ ...base, sheets: [{ id: "s1", name: "Slow" }] });
    await first;

    // The stale load must not even ASK for its sheet — that request would race the fast one.
    expect(h.getSheet).toHaveBeenCalledTimes(1);
    expect(h.showSvg).toHaveBeenCalledTimes(1);
    expect(h.showSvg).toHaveBeenCalledWith('<svg data-sheet="f1"/>');
  });

  it("drops a stale load already waiting on the sheet render", async () => {
    const slow = deferred<{ content: { case: "svg"; value: string } }>();
    const getSheet = vi.fn((req: { uri: string }) =>
      uriPath(req.uri) === "slow.kicad_sch" ? slow.promise : Promise.resolve({ content: { case: "svg" as const, value: "<svg id='fast'/>" } }),
    );
    const h = harness({ getSheet });

    const first = h.preview.show("corpus", "slow.kicad_sch");
    // Let the slow load clear its design lookup and reach GetSheet BEFORE the newer load starts,
    // so it is the post-render guard, not the post-lookup one, that has to reject it.
    await Promise.resolve();
    await Promise.resolve();
    expect(h.getSheet).toHaveBeenCalledTimes(1);

    const second = h.preview.show("corpus", "fast.kicad_sch");
    await second;
    slow.resolve({ content: { case: "svg", value: "<svg id='slow'/>" } });
    await first;

    expect(h.showSvg).toHaveBeenCalledTimes(1);
    expect(h.showSvg).toHaveBeenCalledWith("<svg id='fast'/>");
  });

  it("reports a design with no drawable sheet as a note, not an error", async () => {
    const getDesign = vi.fn(async (_req: object) => ({ name: "", sourceFormat: "", componentCount: 0, netCount: 0, sheets: [] }));
    const h = harness({ getDesign });
    await h.preview.show("corpus", "empty.edn");

    expect(h.getSheet).not.toHaveBeenCalled();
    expect(h.showNote).toHaveBeenLastCalledWith(expect.stringContaining("no drawable sheet"), "info");
    expect(h.showSvg).not.toHaveBeenCalled();
  });

  it("shows a load failure in the stage instead of leaving it blank", async () => {
    const getDesign = vi.fn(async (_req: object) => {
      throw new Error("unknown mount");
    });
    const h = harness({ getDesign });
    await h.preview.show("nope", "x.kicad_sch");

    expect(h.showNote).toHaveBeenLastCalledWith(expect.stringContaining("unknown mount"), "error");
  });

  it("clear abandons a load in flight so a folder selection is not overwritten", async () => {
    const slow = deferred<{ content: { case: "svg"; value: string } }>();
    const getSheet = vi.fn(() => slow.promise);
    const h = harness({ getSheet });

    const pending = h.preview.show("corpus", "slow.kicad_sch");
    h.preview.clear();
    slow.resolve({ content: { case: "svg", value: "<svg id='late'/>" } });
    await pending;

    expect(h.showSvg).not.toHaveBeenCalled();
    expect(h.setCaption).toHaveBeenLastCalledWith("", "");
  });

  it("falls back to the file name when the design declares none", async () => {
    const getDesign = vi.fn(async (_req: object) => ({
      name: "",
      sourceFormat: "edif",
      componentCount: 4,
      netCount: 3,
      sheets: [{ id: "p1", name: "Page 1" }],
    }));
    const h = harness({ getDesign });
    await h.preview.show("corpus", "nested/dir/board.edn");

    expect(h.setCaption).toHaveBeenLastCalledWith("board.edn", "edif · 4 components · 3 nets · 1 sheet");
  });
});

describe("pickPreviewSheet", () => {
  // Sheet lists below are the real GetDesign responses for each shape, read off a running server.
  it("keeps the schematic when the design carries faithful geometry", () => {
    const d = {
      sheets: [{ id: "/", name: "Showcase Board (passes)" }],
      availableLayouts: ["faithful", "force", "grid", "layered", "orthogonal", "stress"],
    };
    expect(pickPreviewSheet(d)?.id).toBe("/");
  });

  it("prefers the board sheet when the design has no faithful drawing of its own", () => {
    const d = {
      sheets: [
        { id: "graph", name: "netlist graph" },
        { id: "board", name: "Board" },
      ],
      availableLayouts: ["force", "grid", "layered", "orthogonal", "stress"],
    };
    expect(pickPreviewSheet(d)?.id).toBe("board");
  });

  // A schematic with a board sidecar offers BOTH. Browsing a .kicad_sch means the schematic, so
  // faithful availability has to win over the mere presence of a board sheet.
  it("keeps the schematic of a faithful design that also has a board sidecar", () => {
    const d = {
      sheets: [
        { id: "/", name: "Top" },
        { id: "board", name: "Board" },
      ],
      availableLayouts: ["faithful", "grid"],
    };
    expect(pickPreviewSheet(d)?.id).toBe("/");
  });

  it("takes the first sheet of a netlist-only design, which has no board", () => {
    const d = { sheets: [{ id: "graph", name: "netlist graph" }], availableLayouts: ["grid", "force"] };
    expect(pickPreviewSheet(d)?.id).toBe("graph");
  });

  it("returns nothing for a design with no sheets", () => {
    expect(pickPreviewSheet({ sheets: [], availableLayouts: [] })).toBeUndefined();
  });
});

describe("captionFor", () => {
  it("omits counts a design does not carry", () => {
    // A geometry-only read has no netlist, so its counts are zero and naming them would assert a
    // design with no components rather than one whose components were never read.
    expect(captionFor("kicad-sch", 0, 0, 2)).toBe("kicad-sch · 2 sheets");
    expect(captionFor("edif", 2, 0, 1)).toBe("edif · 2 components · 1 sheet");
    expect(captionFor("", 0, 0, 0)).toBe("");
  });

  // GetDesign fills format and both counts only on the AUTO-layout branch. A design carrying its
  // own geometry takes the faithful branch, where all three are empty — so a caption built from
  // them alone is blank for every KiCad schematic, which is the common case, not an edge one.
  it("still says something for a faithful design, whose netlist fields are empty", () => {
    expect(captionFor("", 0, 0, 1)).toBe("1 sheet");
    expect(captionFor("", 0, 0, 4)).toBe("4 sheets");
  });
});

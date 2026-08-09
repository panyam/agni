// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { browseStage, designUrl, type StageElements, type SvgSurface } from "./browsestage.js";

function harness() {
  const els: StageElements = {
    note: document.createElement("div"),
    name: document.createElement("span"),
    summary: document.createElement("span"),
    open: document.createElement("button"),
  };
  els.open.disabled = true;
  const svg: SvgSurface & { setSvg: ReturnType<typeof vi.fn>; show: ReturnType<typeof vi.fn>; hide: ReturnType<typeof vi.fn> } = {
    setSvg: vi.fn(),
    show: vi.fn(),
    hide: vi.fn(),
  };
  const navigate = vi.fn();
  return { stage: browseStage(els, svg, navigate), els, svg, navigate };
}

describe("browseStage", () => {
  let h: ReturnType<typeof harness>;
  beforeEach(() => {
    h = harness();
  });

  it("swaps the note out when a drawing arrives", () => {
    h.stage.showNote("rendering preview…", "info");
    h.stage.showSvg("<svg id='a'/>");

    expect(h.svg.setSvg).toHaveBeenCalledWith("<svg id='a'/>");
    expect(h.svg.show).toHaveBeenCalled();
    expect(h.els.note.style.display).toBe("none");
  });

  // The inverse matters more: a failed load after a successful one must not leave the previous
  // design's drawing sitting under an error message, which reads as "this design failed to render"
  // while showing a picture of a different design.
  it("hides a previous drawing when a note replaces it", () => {
    h.stage.showSvg("<svg id='a'/>");
    h.stage.showNote("unknown mount", "error");

    expect(h.svg.hide).toHaveBeenCalled();
    expect(h.els.note.style.display).toBe("");
    expect(h.els.note.textContent).toBe("unknown mount");
    expect(h.els.note.className).toBe("br-note error");
  });

  it("styles an info note apart from an error note", () => {
    h.stage.showNote("Select a design to preview it.", "info");
    expect(h.els.note.className).toBe("br-note");
  });

  it("writes the caption into the header", () => {
    h.stage.setCaption("Amplifier", "kicad-sch · 12 components");
    expect(h.els.name.textContent).toBe("Amplifier");
    expect(h.els.summary.textContent).toBe("kicad-sch · 12 components");
  });

  it("enables Open only while a design is selected", () => {
    expect(h.els.open.disabled).toBe(true);
    h.stage.setTarget({ mount: "corpus", path: "boards/amp.kicad_sch" });
    expect(h.els.open.disabled).toBe(false);
    // Selecting a folder is not selecting something to open.
    h.stage.setTarget(null);
    expect(h.els.open.disabled).toBe(true);
  });

  it("navigates to the selected design's work page on click", () => {
    h.stage.setTarget({ mount: "corpus", path: "boards/amp.kicad_sch" });
    h.els.open.click();

    expect(h.navigate).toHaveBeenCalledWith("/designs/corpus/boards/amp.kicad_sch/view");
  });

  it("does nothing when Open fires with no design selected", () => {
    h.stage.open();
    expect(h.navigate).not.toHaveBeenCalled();
  });
});

describe("designUrl", () => {
  it("builds the work-page URL the router parses back", () => {
    expect(designUrl("corpus", "boards/amp.kicad_sch")).toBe("/designs/corpus/boards/amp.kicad_sch/view");
    expect(designUrl("corpus", "top.edn")).toBe("/designs/corpus/top.edn/view");
  });

  it("encodes segments that are not path-safe", () => {
    expect(designUrl("my mounts", "a b/c#d.kicad_sch")).toBe("/designs/my%20mounts/a%20b/c%23d.kicad_sch/view");
  });
});

import { describe, it, expect } from "vitest";
import { hierarchyNote, hierarchyStrip } from "./hierarchy.js";

const sentence =
  "hier.edn is hierarchical and only its top cell was read. 1 sub-cell holding 1 instance was not extracted (SUB: 1). Counts and checks cover the top cell alone.";

describe("hierarchyNote", () => {
  it("is null for a design read whole, and for a server that sends no field", () => {
    expect(hierarchyNote("")).toBeNull();
    expect(hierarchyNote(undefined)).toBeNull();
  });

  it("keeps the served sentence as sent", () => {
    expect(hierarchyNote(sentence)).toBe(sentence);
  });
});

describe("hierarchyStrip", () => {
  it("shows the server's sentence, unchanged, so the page and the CLI say one thing", () => {
    const el = document.createElement("div");
    hierarchyStrip(el)(hierarchyNote(sentence));
    expect(el.classList.contains("on")).toBe(true);
    expect(el.textContent).toBe(sentence);
  });

  it("hides itself entirely when the next design was read whole", () => {
    const el = document.createElement("div");
    const set = hierarchyStrip(el);
    set(hierarchyNote(sentence));
    set(hierarchyNote(""));
    expect(el.classList.contains("on")).toBe(false);
    expect(el.textContent).toBe("");
  });

  it("is a no-op without an element", () => {
    expect(() => hierarchyStrip(null)(sentence)).not.toThrow();
  });
});

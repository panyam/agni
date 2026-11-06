import { describe, it, expect, vi } from "vitest";
import { hierarchyNote, hierarchyStrip } from "./hierarchy.js";

const sentence =
  "hier.edn is hierarchical and only its top cell was read. 1 sub-cell holding 1 instance was not extracted (SUB: 1). Counts and checks cover the top cell alone.";

describe("hierarchyNote", () => {
  it("is null for a design read whole, and for a server that sends no field", () => {
    expect(hierarchyNote("")).toBeNull();
    expect(hierarchyNote(undefined)).toBeNull();
  });

  it("keeps the served sentence as sent, with whether reading every block would read more", () => {
    expect(hierarchyNote(sentence, true)).toEqual({ text: sentence, flattenReadsMore: true });
    expect(hierarchyNote(sentence)).toEqual({ text: sentence, flattenReadsMore: false });
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

  it("offers to read every block, and runs what the page asked when clicked (agni issue 1102)", () => {
    const el = document.createElement("div");
    const readAll = vi.fn();
    hierarchyStrip(el, readAll)(hierarchyNote(sentence, true));
    const b = el.querySelector("button.hierarchy-read-all") as HTMLButtonElement | null;
    expect(b?.textContent).toBe("Read every block");
    expect(el.querySelector("span")?.textContent).toBe(sentence);
    b?.click();
    expect(readAll).toHaveBeenCalledTimes(1);
  });

  it("offers nothing on a page already reading designs whole", () => {
    const el = document.createElement("div");
    hierarchyStrip(el)(hierarchyNote(sentence, true));
    expect(el.querySelector("button")).toBeNull();
  });

  it("offers nothing when reading every block would read no more, as for a missing file (agni issue 1406)", () => {
    const missing =
      "CLAYR6.DSN is hierarchical and 1 of its blocks could not be read, because its file is missing (IMU: .\\MPU9250\\MPU9250.DSN). Counts and checks leave its contents out.";
    const el = document.createElement("div");
    hierarchyStrip(el, vi.fn())(hierarchyNote(missing, false));
    expect(el.classList.contains("on")).toBe(true);
    expect(el.textContent).toBe(missing);
    expect(el.querySelector("button")).toBeNull();
  });

  it("is a no-op without an element", () => {
    expect(() => hierarchyStrip(null)(hierarchyNote(sentence, true))).not.toThrow();
  });
});

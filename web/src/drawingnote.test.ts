// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { drawingNote, drawingStrip } from "./drawingnote.js";

const sentence = "agni laid these sheets out itself, because x.edn holds no schematic drawing.";

describe("drawingNote", () => {
  it("shows the served sentence on a computed sheet", () => {
    expect(drawingNote(sentence, "s1")).toBe(sentence);
  });
  it("shows nothing on the board, which draws itself", () => {
    expect(drawingNote(sentence, "board")).toBeNull();
  });
  it("shows nothing for a drawn design or an older server", () => {
    expect(drawingNote("", "s1")).toBeNull();
    expect(drawingNote(undefined, "s1")).toBeNull();
  });
});

describe("drawingStrip", () => {
  it("shows the note and hides again when it clears", () => {
    const el = document.createElement("div");
    const set = drawingStrip(el);
    set(sentence);
    expect(el.classList.contains("on")).toBe(true);
    expect(el.textContent).toBe(sentence);
    set(null);
    expect(el.classList.contains("on")).toBe(false);
    expect(el.textContent).toBe("");
  });
});

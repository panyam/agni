import { describe, it, expect } from "vitest";
import {
  classifyPointerDown,
  selectionAfterPointerDown,
  classifyKey,
  isFormField,
  type HitTarget,
} from "./pagegestures.js";

const nothing: HitTarget = { deleteButton: false, handle: null, regionId: null, regionKind: null };
const hit = (over: Partial<HitTarget>): HitTarget => ({ ...nothing, ...over });
const key = (k: string, mods: Partial<{ shiftKey: boolean; ctrlKey: boolean; metaKey: boolean; altKey: boolean }> = {}) => ({
  key: k,
  shiftKey: false,
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  ...mods,
});

describe("classifyPointerDown", () => {
  it("pans on empty page, which is the whole point of the change", () => {
    expect(classifyPointerDown(nothing, { selectedId: "", drawIntent: false })).toEqual({ kind: "pan" });
  });

  it("draws when shift or Draw mode set drawIntent", () => {
    expect(classifyPointerDown(nothing, { selectedId: "", drawIntent: true })).toEqual({ kind: "draw" });
  });

  it("draws OVER an existing region rather than moving it", () => {
    // Datasheet tables overlap constantly. If a region body outranked drawIntent, a table covering
    // half the page would make that half un-drawable-over.
    const over = hit({ regionId: "u1", regionKind: "user" });
    expect(classifyPointerDown(over, { selectedId: "u1", drawIntent: true })).toEqual({ kind: "draw" });
  });

  it("keeps resize handles working even in Draw mode", () => {
    // The handles are small deliberate targets, so hitting one is never accidental; swallowing them
    // would mean toggling Draw off to adjust the box just drawn.
    const onHandle = hit({ handle: "se", regionId: "u1", regionKind: "user" });
    expect(classifyPointerDown(onHandle, { selectedId: "u1", drawIntent: true })).toEqual({ kind: "resize", handle: "se" });
  });

  it("moves the SELECTED user region, and pans over an unselected one", () => {
    const over = hit({ regionId: "u1", regionKind: "user" });
    expect(classifyPointerDown(over, { selectedId: "u1", drawIntent: false })).toEqual({ kind: "move" });
    expect(classifyPointerDown(over, { selectedId: "u2", drawIntent: false })).toEqual({ kind: "pan" });
  });

  it("pans over a doc-IR region, which was never draggable", () => {
    const over = hit({ regionId: "t3", regionKind: "table" });
    expect(classifyPointerDown(over, { selectedId: "t3", drawIntent: false })).toEqual({ kind: "pan" });
  });

  it("deletes on the on-box ×, ahead of everything else", () => {
    const x = hit({ deleteButton: true, regionId: "u1", regionKind: "user", handle: "nw" });
    expect(classifyPointerDown(x, { selectedId: "u1", drawIntent: true })).toEqual({ kind: "delete" });
  });

  it("ignores a handle that belongs to a region other than the selected one", () => {
    const stray = hit({ handle: "nw", regionId: "u9", regionKind: "user" });
    expect(classifyPointerDown(stray, { selectedId: "u1", drawIntent: false })).toEqual({ kind: "pan" });
  });
});

describe("selectionAfterPointerDown", () => {
  it("selects the pressed region even when the gesture is a pan", () => {
    const over = hit({ regionId: "t3", regionKind: "table" });
    expect(selectionAfterPointerDown(over, { kind: "pan" })).toBe("t3");
  });

  it("leaves the selection alone for a draw or a delete", () => {
    const over = hit({ regionId: "u1", regionKind: "user" });
    expect(selectionAfterPointerDown(over, { kind: "draw" })).toBeNull();
    expect(selectionAfterPointerDown(over, { kind: "delete" })).toBeNull();
  });
});

describe("classifyKey", () => {
  it("pages with the reader convention", () => {
    expect(classifyKey(key("PageDown"), false)).toEqual({ kind: "page", to: "next" });
    expect(classifyKey(key("PageUp"), false)).toEqual({ kind: "page", to: "prev" });
    expect(classifyKey(key("Home"), false)).toEqual({ kind: "page", to: "first" });
    expect(classifyKey(key("End"), false)).toEqual({ kind: "page", to: "last" });
  });

  it("pages with shift+arrows: left/right step, up/down jump to the ends", () => {
    expect(classifyKey(key("ArrowRight", { shiftKey: true }), false)).toEqual({ kind: "page", to: "next" });
    expect(classifyKey(key("ArrowLeft", { shiftKey: true }), false)).toEqual({ kind: "page", to: "prev" });
    expect(classifyKey(key("ArrowUp", { shiftKey: true }), false)).toEqual({ kind: "page", to: "first" });
    expect(classifyKey(key("ArrowDown", { shiftKey: true }), false)).toEqual({ kind: "page", to: "last" });
  });

  it("leaves a bare arrow alone", () => {
    expect(classifyKey(key("ArrowRight"), false)).toBeNull();
  });

  it("zooms and fits", () => {
    expect(classifyKey(key("+"), false)).toEqual({ kind: "zoom", to: "in" });
    expect(classifyKey(key("="), false)).toEqual({ kind: "zoom", to: "in" });
    expect(classifyKey(key("-"), false)).toEqual({ kind: "zoom", to: "out" });
    expect(classifyKey(key("0"), false)).toEqual({ kind: "zoom", to: "fit" });
  });

  it("toggles and exits Draw mode", () => {
    expect(classifyKey(key("r"), false)).toEqual({ kind: "toggleDraw" });
    expect(classifyKey(key("R"), false)).toEqual({ kind: "toggleDraw" });
    expect(classifyKey(key("Escape"), false)).toEqual({ kind: "exitDraw" });
  });

  it("stays out of the way while a form field has focus", () => {
    // Otherwise typing a parameter value into the transcribe panel pages the document out from
    // under it, or deletes the region being transcribed.
    for (const k of ["PageDown", "Home", "Delete", "r", "0", "-"]) {
      expect(classifyKey(key(k), true)).toBeNull();
    }
    expect(classifyKey(key("ArrowLeft", { shiftKey: true }), true)).toBeNull();
  });

  it("leaves browser shortcuts alone", () => {
    expect(classifyKey(key("0", { metaKey: true }), false)).toBeNull();
    expect(classifyKey(key("r", { ctrlKey: true }), false)).toBeNull();
  });
});

describe("isFormField", () => {
  it("recognizes the controls that swallow keystrokes", () => {
    for (const tag of ["INPUT", "TEXTAREA", "SELECT"]) {
      expect(isFormField({ tagName: tag } as Element)).toBe(true);
    }
    expect(isFormField({ tagName: "DIV" } as Element)).toBe(false);
    expect(isFormField({ tagName: "DIV", isContentEditable: true } as unknown as Element)).toBe(true);
    expect(isFormField(null)).toBe(false);
  });
});

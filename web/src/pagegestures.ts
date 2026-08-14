// pagegestures routes the datasheet workbench's pointer and keyboard input, split out of
// regionview.tsx so the routing is testable without pdf.js or a DOM (regionview.tsx has no test of
// its own, the way regions.ts was split out for the same reason).
//
// The workbench navigates like the schematic viewers: wheel zooms, drag pans (see panzoom.ts). That
// leaves drawing a region, which used to own the plain drag, needing somewhere to go. It gets two
// entrances: hold Shift, or flip the sticky Draw mode on for a long transcription session. Both
// arrive here as one `drawIntent` flag, so nothing downstream knows or cares which was used.
import type { Handle, Region } from "./regions.js";

// PointerIntent is what a pointerdown starts. Everything except "pan" is a direct manipulation of a
// region; "pan" is the fallback, which is why a drag anywhere harmless moves the page.
export type PointerIntent =
  | { kind: "delete" }
  | { kind: "resize"; handle: Handle }
  | { kind: "draw" }
  | { kind: "move" }
  | { kind: "pan" };

// HitTarget describes what sits under the pointer, read off the DOM by the caller so this stays
// pure. `handle` is set only when the pointer is on a resize handle, which only the selected user
// region draws.
export interface HitTarget {
  deleteButton: boolean;
  handle: Handle | null;
  regionId: string | null;
  regionKind: Region["kind"] | null;
}

// classifyPointerDown decides the gesture. The order encodes the priority:
//
//  1. the on-box × deletes, and never starts a drag
//  2. a resize handle resizes, even in Draw mode — the handles are small, deliberate targets, so
//     hitting one is never accidental, and letting Draw mode swallow them would mean toggling the
//     mode off to adjust the box you just drew
//  3. drawIntent draws, outranking a region body so a new box can be drawn ON TOP of an existing
//     one (datasheet tables overlap constantly, and a table region covering half the page would
//     otherwise be un-drawable-over)
//  4. the SELECTED user region's body moves. Requiring selection first is what keeps panning
//     predictable: a drag across a page dense with boxes pans instead of scattering them, and
//     click-then-drag to move is the ordinary direct-manipulation idiom
//  5. everything else pans
export function classifyPointerDown(
  hit: HitTarget,
  opts: { selectedId: string; drawIntent: boolean },
): PointerIntent {
  if (hit.deleteButton) return { kind: "delete" };
  if (hit.handle && hit.regionId === opts.selectedId) return { kind: "resize", handle: hit.handle };
  if (opts.drawIntent) return { kind: "draw" };
  if (hit.regionId && hit.regionId === opts.selectedId && hit.regionKind === "user") return { kind: "move" };
  return { kind: "pan" };
}

// selectionAfterPointerDown returns the region id the pointerdown selects, or null to leave the
// selection alone. Pressing on a region selects it even when the gesture turns out to be a pan, so
// one press still both selects and starts moving the page; a draw leaves the selection alone
// because addUserRegion will select the new box on pointerup.
export function selectionAfterPointerDown(hit: HitTarget, intent: PointerIntent): string | null {
  if (intent.kind === "draw" || intent.kind === "delete") return null;
  return hit.regionId;
}

// NavAction is a keyboard command against the workbench viewport.
export type NavAction =
  | { kind: "page"; to: "first" | "last" | "prev" | "next" }
  | { kind: "zoom"; to: "in" | "out" | "fit" }
  | { kind: "deleteRegion" }
  | { kind: "toggleDraw" }
  | { kind: "exitDraw" };

// KeyEvent is the slice of KeyboardEvent the bindings read.
export interface KeyEvent {
  key: string;
  shiftKey: boolean;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
}

// ZOOM_KEY_FACTOR is one keyboard zoom step. Larger than a wheel notch because a key press is a
// discrete act rather than a continuous gesture.
export const ZOOM_KEY_FACTOR = 1.25;

// classifyKey maps a keypress to a viewport command, or null to leave it to the browser.
//
// Paging carries two bindings on purpose. PageUp/PageDown/Home/End is what every paged-document
// reader uses, so it needs no learning; Shift+arrows is the same four commands under one shape,
// with left/right stepping and up/down jumping to the ends.
//
// `inField` suppresses everything while a form control has focus, so typing a value into the
// transcribe panel cannot page the document out from under it. Ctrl/Cmd combinations are left
// alone so browser shortcuts keep working.
export function classifyKey(e: KeyEvent, inField: boolean): NavAction | null {
  if (inField || e.ctrlKey || e.metaKey) return null;
  switch (e.key) {
    case "PageDown":
      return { kind: "page", to: "next" };
    case "PageUp":
      return { kind: "page", to: "prev" };
    case "Home":
      return { kind: "page", to: "first" };
    case "End":
      return { kind: "page", to: "last" };
    case "ArrowRight":
      return e.shiftKey ? { kind: "page", to: "next" } : null;
    case "ArrowLeft":
      return e.shiftKey ? { kind: "page", to: "prev" } : null;
    case "ArrowUp":
      return e.shiftKey ? { kind: "page", to: "first" } : null;
    case "ArrowDown":
      return e.shiftKey ? { kind: "page", to: "last" } : null;
    case "Delete":
    case "Backspace":
      return { kind: "deleteRegion" };
    case "+":
    case "=":
      return { kind: "zoom", to: "in" };
    case "-":
    case "_":
      return { kind: "zoom", to: "out" };
    case "0":
      return { kind: "zoom", to: "fit" };
    case "r":
    case "R":
      return { kind: "toggleDraw" };
    case "Escape":
      return { kind: "exitDraw" };
    default:
      return null;
  }
}

// isFormField reports whether an element swallows keystrokes, so classifyKey's `inField` can be
// filled from document.activeElement.
export function isFormField(el: Element | null): boolean {
  if (!el) return false;
  if (/^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName)) return true;
  return (el as HTMLElement).isContentEditable === true;
}

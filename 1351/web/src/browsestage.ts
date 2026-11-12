// The browse page's right-hand side (WS9-049 phase 2), the preview surface plus the one action
// that leaves the page. It is a plain-DOM view adapter shaped like expectcaption.ts, with no
// framework and no presenter. So browse.ts remains declarative, and this file is testable without
// booting a page.
import { emptyLocation, locationToUrl } from "./router.js";
import type { PreviewView } from "./preview.js";

// SvgSurface is the slice of SvgView that the stage drives. It is narrow so the stage can be tested
// against a stub. The browse preview uses none of SvgView's overlay, camera or reveal machinery,
// since it shows one document and never touches it again.
export interface SvgSurface {
  setSvg(markup: string): void;
  show(): void;
  hide(): void;
}

// StageElements are the server-rendered holes that the stage drives, resolved once at boot.
export interface StageElements {
  note: HTMLElement;
  name: HTMLElement;
  summary: HTMLElement;
  open: HTMLButtonElement;
}

// DesignTarget names a design the Open action would open.
export interface DesignTarget {
  mount: string;
  path: string;
}

// BrowseStage is the preview surface plus its Open affordance.
export interface BrowseStage extends PreviewView {
  // setTarget marks which design Open would open, enabling the action; null disables it. A folder
  // selection passes null, because a folder is a place to look rather than a thing to open.
  setTarget(target: DesignTarget | null): void;
  // open navigates to the current target's work page, or does nothing when there is none. Exposed
  // so the page can bind the same action to a double-click and Enter as well as the button.
  open(): void;
}

// designUrl is the work-page URL for a design. It goes through locationToUrl rather than
// assembling a path. The work page parses URLs in the same place, so the /view verb, segment order
// and encoding are defined once.
export function designUrl(mount: string, path: string): string {
  return locationToUrl({ ...emptyLocation(), mount, path });
}

// browseStage wires the preview elements into a view, and the DesignPreview pushes to that view.
// The page passes window.location.assign as navigate. It is injected because the work page is a
// different document with a different bundle. Routing is server-owned (C11), so only a real
// navigation reaches the work page.
export function browseStage(els: StageElements, svg: SvgSurface, navigate: (url: string) => void): BrowseStage {
  let target: DesignTarget | null = null;

  const stage: BrowseStage = {
    showSvg: (markup) => {
      // The note and the drawing never show together. This hides the note, and showNote hides the
      // drawing. So a failed load never leaves a stale drawing under the note, and a fresh render
      // never has a message on top of it.
      els.note.style.display = "none";
      svg.show();
      svg.setSvg(markup);
    },
    showNote: (text, kind) => {
      svg.hide();
      els.note.textContent = text;
      els.note.className = kind === "error" ? "br-note error" : "br-note";
      els.note.style.display = "";
    },
    setCaption: (name, summary) => {
      els.name.textContent = name;
      els.summary.textContent = summary;
    },
    setTarget: (t) => {
      target = t;
      els.open.disabled = t === null;
    },
    open: () => {
      if (target) navigate(designUrl(target.mount, target.path));
    },
  };

  els.open.addEventListener("click", () => stage.open());
  return stage;
}

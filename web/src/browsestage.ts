// The browse page's right-hand side (WS9-049 phase 2), the preview surface plus the one action
// that leaves the page. A plain-DOM view adapter in the shape of expectcaption.ts, with no framework
// and no presenter, so browse.ts stays declarative and this is testable without booting a page.
import { emptyLocation, locationToUrl } from "./router.js";
import type { PreviewView } from "./preview.js";

// SvgSurface is the slice of SvgView the stage drives, narrow so the stage can be tested against a
// stub. The browse preview uses none of SvgView's overlay, camera or reveal machinery, since it shows
// one document and never touches it again.
export interface SvgSurface {
  setSvg(markup: string): void;
  show(): void;
  hide(): void;
}

// StageElements are the server-rendered holes the stage drives, resolved once at boot.
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
// assembling a path, so the /view verb, segment order and encoding stay in the one place the work
// page parses them.
export function designUrl(mount: string, path: string): string {
  return locationToUrl({ ...emptyLocation(), mount, path });
}

// browseStage wires the preview elements into a view the DesignPreview pushes to. navigate is
// injected (window.location.assign in the page) because the work page is a different document with
// a different bundle, reached by a real navigation since routing is server-owned (C11).
export function browseStage(els: StageElements, svg: SvgSurface, navigate: (url: string) => void): BrowseStage {
  let target: DesignTarget | null = null;

  const stage: BrowseStage = {
    showSvg: (markup) => {
      // The note and the drawing never show together. Hiding the note here (and the drawing in
      // showNote) keeps a failed load from leaving a stale drawing under it, or a message on top of
      // a fresh render.
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

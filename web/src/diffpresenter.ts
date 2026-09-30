// DiffPresenter coordinates the visual diff (WS9-005). Two files are compared once
// (DiffDesigns), their sheet lists are paired by name, and each side of the current pair
// renders as an SVG sheet with a highlight overlay per change class, through the same
// HighlightSheet path the single-file viewer uses for findings. Like ViewerPresenter it is
// framework-neutral (C3), talking to the sides through DiffRenderView and pushing DiffState to
// the panel. It is independent of ViewerPresenter, so the single-file viewer keeps its own
// file, sheet and highlight state while a diff is open.

import type { Client } from "@connectrpc/connect";
import { artifactUri } from "./uri.js";
import { DesignService, SheetFormat, SymbolSource } from "./gen/agni/v1/webapi/design_pb.js";
import { DiffService, type DiffDesignsResponse } from "./gen/agni/v1/webapi/diff_pb.js";
import {
  changedItems,
  checkAlignment,
  focusSpecs,
  ghostSpecs,
  itemId,
  legendEntries,
  pairSheets,
  sideSpecs,
  svgFrame,
  type ChangedItem,
  type DiffSide,
  type Frame,
  type LegendEntry,
  type SheetPair,
} from "./diff.js";

type DiffClient = Client<typeof DiffService>;
type DesignClient = Client<typeof DesignService>;

// DiffFileRef names one side's file by mount and path, and artifactUri builds the URI each
// RPC takes from it.
export interface DiffFileRef {
  mount: string;
  path: string;
}

// DiffSideView is the command-down interface for one side's canvas. setOverlay stacks a
// highlight overlay framed like the sheet ("" clears), and showPlaceholder replaces the sheet
// when the pair has none on this side or it failed to render. showSvg and showPlaceholder each
// hide the other. reveal pans and zooms to the current overlay's content (the focused item,
// WS9-006) and is a no-op with nothing highlighted. Camera math stays in the view (C3).
export interface DiffSideView {
  showSvg(markup: string): void;
  setOverlay(markup: string): void;
  // setOverlays stacks several overlay documents ([] clears). The union canvas uses it for b's
  // highlight overlay plus a's removed-ghost overlay (WS9-007).
  setOverlays(markups: string[]): void;
  showPlaceholder(text: string): void;
  reveal(): void;
}

// DiffRenderView is the diff view, holding the two sides, one busy indicator, and the
// side-by-side or single-canvas arrangement (WS9-007). In overlay mode the view shows only the
// a-side canvas, full width and relabeled, and the presenter renders the union into it.
export interface DiffRenderView {
  a: DiffSideView;
  b: DiffSideView;
  setBusy(busy: boolean): void;
  setOverlayMode(on: boolean): void;
}

// DiffMode is the diff canvas arrangement, either two synced panes or the union on one canvas.
export type DiffMode = "side" | "overlay";

// DiffState is what the diff panels render, and one state feeds both the chrome bar and the
// changes panel. selected is the focused item (WS9-006). error is set when the comparison
// itself failed, and then pairs is empty.
export interface DiffState {
  active: boolean;
  aLabel: string;
  bLabel: string;
  pairs: SheetPair[];
  activePair: number; // index into pairs, -1 when none
  legend: LegendEntry[];
  items: ChangedItem[];
  selected: string; // itemId of the focused item, "" when none
  // overlayOk says whether the union mode is offered for the CURRENT pair, and overlayReason
  // (set only when not ok) explains the disabled toggle.
  mode: DiffMode;
  overlayOk: boolean;
  overlayReason: string;
  error: string;
}

// emptyDiffState is the inactive state (no comparison open).
export function emptyDiffState(): DiffState {
  return {
    active: false,
    aLabel: "",
    bLabel: "",
    pairs: [],
    activePair: -1,
    legend: [],
    items: [],
    selected: "",
    mode: "side",
    overlayOk: false,
    overlayReason: "",
    error: "",
  };
}

export class DiffPresenter {
  private aRef: DiffFileRef = { mount: "", path: "" };
  private bRef: DiffFileRef = { mount: "", path: "" };
  // Each side renders in the layout the server chose for that design (the effective layout
  // of a plain open), so a faithful-geometry file and a netlist-only one both draw.
  private aLayout = "";
  private bLayout = "";
  private componentStatus: Record<string, string> = {};
  private netStatus: Record<string, string> = {};
  private pairs: SheetPair[] = [];
  private activePair = -1;
  private legend: LegendEntry[] = [];
  private items: ChangedItem[] = [];
  // focusItem is the emphasized changed item (null means selected is ""). While set, the
  // overlays show only it instead of every change.
  private focusItem: ChangedItem | null = null;
  private active = false;
  private error = "";
  // The current pair's sheet documents are retained (docA/docB, null meaning a placeholder
  // showing errA/errB), so the mode toggle (WS9-007) redraws from memory and re-fetches only
  // the overlays. Their frames plus the response's shared placements are the alignment
  // evidence behind overlayOk and overlayReason.
  private mode: DiffMode = "side";
  private docA: string | null = null;
  private docB: string | null = null;
  private errA = "";
  private errB = "";
  private frameA: Frame | null = null;
  private frameB: Frame | null = null;
  private placementsA: DiffDesignsResponse["sharedPlacementsA"] = {};
  private placementsB: DiffDesignsResponse["sharedPlacementsB"] = {};
  private overlayOk = false;
  private overlayReason = "";

  constructor(
    private readonly diff: DiffClient,
    private readonly designs: DesignClient,
    private readonly view: DiffRenderView,
    private readonly onState: (s: DiffState) => void,
  ) {}

  private label(r: DiffFileRef): string {
    return `${r.mount}:${r.path}`;
  }

  private pushState(): void {
    this.onState({
      active: this.active,
      aLabel: this.label(this.aRef),
      bLabel: this.label(this.bRef),
      pairs: this.pairs,
      activePair: this.activePair,
      legend: this.legend,
      items: this.items,
      selected: this.focusItem ? itemId(this.focusItem) : "",
      mode: this.mode,
      overlayOk: this.overlayOk,
      overlayReason: this.overlayReason,
      error: this.error,
    });
  }

  // open runs the comparison. One DiffDesigns call fetches the report and highlight maps and
  // one GetDesign per side fetches the sheet lists, then the first pair with both sides present
  // renders (else the first pair). Either file failing to diff or load fails the whole open,
  // as the RPC has no partial diff, and the panel shows the error over two placeholders.
  async open(a: DiffFileRef, b: DiffFileRef): Promise<void> {
    this.aRef = a;
    this.bRef = b;
    this.view.setBusy(true);
    try {
      const [resp, da, db] = await Promise.all([
        this.diff.diffDesigns({ aUri: artifactUri(a.mount, a.path), bUri: artifactUri(b.mount, b.path) }),
        this.designs.getDesign({ uri: artifactUri(a.mount, a.path), layout: "" }),
        this.designs.getDesign({ uri: artifactUri(b.mount, b.path), layout: "" }),
      ]);
      this.componentStatus = resp.componentStatus;
      this.netStatus = resp.netStatus;
      this.placementsA = resp.sharedPlacementsA;
      this.placementsB = resp.sharedPlacementsB;
      this.mode = "side";
      this.aLayout = da.layout;
      this.bLayout = db.layout;
      this.pairs = pairSheets(da.sheets, db.sheets);
      const both = this.pairs.findIndex((p) => p.aId && p.bId);
      this.activePair = both >= 0 ? both : this.pairs.length > 0 ? 0 : -1;
      this.legend = legendEntries(resp.report);
      this.items = changedItems(resp);
      this.focusItem = null;
      this.error = "";
      this.active = true;
      this.pushState();
      if (this.activePair >= 0) await this.showPair(this.activePair);
    } catch (e) {
      this.active = true;
      this.error = String(e);
      this.pairs = [];
      this.activePair = -1;
      this.legend = [];
      this.items = [];
      this.focusItem = null;
      this.view.a.showPlaceholder("comparison failed");
      this.view.b.showPlaceholder("comparison failed");
      this.pushState();
    } finally {
      this.view.setBusy(false);
    }
  }

  // selectItem focuses one changed item for the changes panel. The overlays emphasize only it
  // and the view reveals its location. When the current pair does not show the item, it first
  // switches to the first pair that does, while an item with no sheet entries (no geometry)
  // stays on the current pair. Clicking the focused item again clears the focus. An explicit
  // pair (a sheet-badge click) never toggles off, matching the findings panel's badges.
  async selectItem(id: string, pair?: number): Promise<void> {
    const item = this.items.find((i) => itemId(i) === id);
    const toggleOff = !item || (this.focusItem !== null && itemId(this.focusItem) === id && pair === undefined);
    this.focusItem = toggleOff ? null : item!;
    let target = pair ?? this.activePair;
    if (!toggleOff && pair === undefined && !this.pairShowsItem(this.pairs[this.activePair], item!)) {
      const i = this.pairs.findIndex((p) => this.pairShowsItem(p, item!));
      if (i >= 0) target = i;
    }
    const pairChanged = target !== this.activePair && target >= 0 && target < this.pairs.length;
    if (pairChanged) this.activePair = target;
    this.pushState();
    if (pairChanged) await this.showPair(this.activePair);
    else await this.refreshOverlays();
    if (!toggleOff) this.revealFocus(this.focusItem!);
  }

  private pairShowsItem(p: SheetPair | undefined, item: ChangedItem): boolean {
    if (!p) return false;
    return (p.aId !== "" && item.aSheets.includes(p.aId)) || (p.bId !== "" && item.bSheets.includes(p.bId));
  }

  // revealFocus asks the side that owns the focused item to center on it, the old side for
  // removed/deleted entities and the new side otherwise. The view mirrors the camera to its
  // sibling so both panes land there. In overlay mode everything draws on the a-canvas, so
  // that is always the target.
  private revealFocus(item: ChangedItem): void {
    if (this.mode === "overlay") {
      this.view.a.reveal();
      return;
    }
    const side = item.cls === "removed" || item.cls === "deleted" ? this.view.a : this.view.b;
    side.reveal();
  }

  // setMode switches between the two synced panes and the single union canvas (WS9-007),
  // redrawing the retained sheet documents and re-fetching only the overlays. Entering overlay
  // mode on an unaligned pair is ignored, since the UI disables that toggle with the reason.
  async setMode(mode: DiffMode): Promise<void> {
    if (mode === this.mode) return;
    if (mode === "overlay" && !this.overlayOk) return;
    this.mode = mode;
    this.pushState();
    const p = this.pairs[this.activePair];
    if (!p) return;
    this.view.setBusy(true);
    try {
      if (this.mode === "overlay") {
        await this.renderUnion(p);
      } else {
        this.view.setOverlayMode(false);
        this.showDoc("a");
        this.showDoc("b");
        await this.refreshOverlays();
      }
    } finally {
      this.view.setBusy(false);
    }
  }

  // selectPair shows another sheet pair for the selector. An out-of-range index is ignored
  // rather than clearing the view.
  async selectPair(i: number): Promise<void> {
    if (i < 0 || i >= this.pairs.length || i === this.activePair) return;
    this.activePair = i;
    this.pushState();
    await this.showPair(i);
  }

  // close ends the comparison and resets to the inactive state. It only clears the
  // presenter's state, and the host closes the dock panel.
  close(): void {
    this.aRef = { mount: "", path: "" };
    this.bRef = { mount: "", path: "" };
    this.componentStatus = {};
    this.netStatus = {};
    this.pairs = [];
    this.activePair = -1;
    this.legend = [];
    this.items = [];
    this.focusItem = null;
    this.mode = "side";
    this.docA = this.docB = null;
    this.frameA = this.frameB = null;
    this.placementsA = {};
    this.placementsB = {};
    this.overlayOk = false;
    this.overlayReason = "";
    this.active = false;
    this.error = "";
    this.pushState();
  }

  // showPair renders one sheet pair. It fetches and retains both documents with their frames,
  // which the alignment check and the mode toggle both need, re-judges overlay availability,
  // and draws in the active mode. An unaligned pair in overlay mode falls back to side-by-side,
  // and the state carries the reason.
  private async showPair(i: number): Promise<void> {
    const p = this.pairs[i];
    if (!p) return;
    this.view.setBusy(true);
    try {
      await Promise.all([this.fetchDoc("a", p), this.fetchDoc("b", p)]);
      const verdict = checkAlignment(p, this.placementsA, this.placementsB, this.frameA, this.frameB);
      this.overlayOk = verdict.ok;
      this.overlayReason = verdict.reason;
      if (this.mode === "overlay" && !this.overlayOk) this.mode = "side";
      this.pushState();
      if (this.mode === "overlay") {
        await this.renderUnion(p);
      } else {
        this.view.setOverlayMode(false);
        this.showDoc("a");
        this.showDoc("b");
        await this.refreshOverlays();
      }
    } finally {
      this.view.setBusy(false);
    }
  }

  // fetchDoc loads one side's sheet document into its retained slot. A missing sheet or a
  // failed render retains null plus the placeholder text instead.
  private async fetchDoc(side: DiffSide, p: SheetPair): Promise<void> {
    const sheetId = side === "a" ? p.aId : p.bId;
    const ref = side === "a" ? this.aRef : this.bRef;
    const layout = side === "a" ? this.aLayout : this.bLayout;
    let doc: string | null = null;
    let err = `no sheet "${p.name}" in ${side.toUpperCase()}`;
    if (sheetId) {
      try {
        const resp = await this.designs.getSheet({
          uri: artifactUri(ref.mount, ref.path),
          sheet: sheetId,
          layout,
          format: SheetFormat.SVG,
          symbols: SymbolSource.GLYPH,
        });
        doc = resp.content.case === "svg" ? resp.content.value : "";
      } catch (e) {
        err = `error: ${String(e)}`;
      }
    }
    if (side === "a") {
      this.docA = doc;
      this.errA = err;
      this.frameA = doc ? svgFrame(doc) : null;
    } else {
      this.docB = doc;
      this.errB = err;
      this.frameB = doc ? svgFrame(doc) : null;
    }
  }

  // showDoc shows one side's retained document, or its placeholder.
  private showDoc(side: DiffSide): void {
    const v = side === "a" ? this.view.a : this.view.b;
    const doc = side === "a" ? this.docA : this.docB;
    if (doc !== null) v.showSvg(doc);
    else v.showPlaceholder(side === "a" ? this.errA : this.errB);
  }

  // renderUnion draws the union (WS9-007) into the a-canvas. b's document is the neutral base,
  // since it holds the unchanged, added and changed geometry, and b's highlight overlay and a's
  // removed-ghost overlay stack above it. A pair with no b side is never aligned, so the null
  // guard should be unreachable.
  private async renderUnion(p: SheetPair): Promise<void> {
    this.view.setOverlayMode(true);
    if (this.docB === null) {
      this.view.a.showPlaceholder(this.errB);
      return;
    }
    this.view.a.showSvg(this.docB);
    await this.sendUnionOverlays(p);
  }

  // sendUnionOverlays fetches the union canvas's two layers, b's specs on b's sheet and the
  // removed/deleted ghosts on a's sheet, which land in place because the frames are aligned.
  // A focused item narrows both layers as it does the side-by-side overlays.
  private async sendUnionOverlays(p: SheetPair): Promise<void> {
    const bSpecs = this.specsFor("b");
    const gSpecs = this.focusItem
      ? this.focusItem.cls === "removed" || this.focusItem.cls === "deleted"
        ? focusSpecs(this.focusItem, "a")
        : []
      : ghostSpecs(this.componentStatus, this.netStatus);
    const [bo, go] = await Promise.all([
      bSpecs.length > 0 && p.bId ? this.fetchOverlay(this.bRef, this.bLayout, p.bId, bSpecs) : Promise.resolve(""),
      gSpecs.length > 0 && p.aId ? this.fetchOverlay(this.aRef, this.aLayout, p.aId, gSpecs) : Promise.resolve(""),
    ]);
    this.view.a.setOverlays([bo, go].filter((m) => m !== ""));
  }

  // specsFor returns what a side highlights, which is the focused item alone while one is
  // selected (its class colors, filtered to the side) and every change on that side otherwise.
  private specsFor(side: DiffSide) {
    return this.focusItem ? focusSpecs(this.focusItem, side) : sideSpecs(this.componentStatus, this.netStatus, side);
  }

  // refreshOverlays re-fetches only the highlight layers for the current pair, for a focus
  // change on the same sheet. In overlay mode it recomposes the union layers instead of the
  // per-side overlays.
  private async refreshOverlays(): Promise<void> {
    const p = this.pairs[this.activePair];
    if (!p) return;
    if (this.mode === "overlay") {
      await this.sendUnionOverlays(p);
      return;
    }
    // A side showing a placeholder (no sheet, failed render) has nothing to overlay.
    await Promise.all([
      p.aId && this.docA !== null ? this.sendOverlay("a", this.aRef, this.aLayout, p.aId) : Promise.resolve(),
      p.bId && this.docB !== null ? this.sendOverlay("b", this.bRef, this.bLayout, p.bId) : Promise.resolve(),
    ]);
  }

  // sendOverlay fetches and stacks one side's highlight overlay, clearing it when there are
  // no specs.
  private async sendOverlay(side: DiffSide, ref: DiffFileRef, layout: string, sheetId: string): Promise<void> {
    const v = side === "a" ? this.view.a : this.view.b;
    const specs = this.specsFor(side);
    if (specs.length === 0) {
      v.setOverlay("");
      return;
    }
    v.setOverlay(await this.fetchOverlay(ref, layout, sheetId, specs));
  }

  // fetchOverlay makes the one HighlightSheet call. It returns the overlay document, or ""
  // when the fetch fails, so the sheet still shows unhighlighted.
  private async fetchOverlay(ref: DiffFileRef, layout: string, sheetId: string, specs: ReturnType<typeof sideSpecs>): Promise<string> {
    try {
      const o = await this.designs.highlightSheet({
        uri: artifactUri(ref.mount, ref.path),
        sheet: sheetId,
        layout,
        symbols: SymbolSource.GLYPH,
        format: SheetFormat.SVG,
        specs,
      });
      return o.content.case === "svg" ? o.content.value : "";
    } catch {
      return "";
    }
  }
}

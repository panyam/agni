// DiffPresenter coordinates the visual diff (WS9-005): two files are compared once
// (DiffDesigns), their sheet lists paired by name, and each side of the current pair is
// rendered as an SVG sheet with a per-change-class highlight overlay (the same
// HighlightSheet path the single-file viewer uses for findings). Like ViewerPresenter it is
// framework-neutral (C3): it talks to the sides through DiffRenderView and pushes DiffState
// to the panel, never a DOM node. It is deliberately independent of ViewerPresenter — the
// single-file viewer keeps its own file/sheet/highlight state while a diff is open.

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

// DiffFileRef addresses one side's file the way every other RPC does: (mount, path).
export interface DiffFileRef {
  mount: string;
  path: string;
}

// DiffSideView is the command-down surface for one side's canvas: show a sheet document,
// stack the highlight overlay above it (framed like the sheet, "" clears), or show a
// placeholder message instead of a sheet (the pair has no sheet on this side, or it failed
// to render). showSvg and showPlaceholder are mutually exclusive — each hides the other.
// reveal pans/zooms to the current overlay's content (the focused item's location,
// WS9-006); a view with nothing highlighted treats it as a no-op. Camera math is
// view-local (C3) — the presenter only says "show me what's highlighted".
export interface DiffSideView {
  showSvg(markup: string): void;
  setOverlay(markup: string): void;
  // setOverlays stacks several overlay documents (the union canvas composes b's highlight
  // overlay with a's removed-ghost overlay, WS9-007); [] clears.
  setOverlays(markups: string[]): void;
  showPlaceholder(text: string): void;
  reveal(): void;
}

// DiffRenderView is the whole diff surface: the two sides, one busy indicator, and the
// side-by-side vs single-canvas arrangement (WS9-007): in overlay mode the view shows only
// the a-side canvas (full width, relabeled) — the presenter renders the union into it.
export interface DiffRenderView {
  a: DiffSideView;
  b: DiffSideView;
  setBusy(busy: boolean): void;
  setOverlayMode(on: boolean): void;
}

// DiffMode is the diff canvas arrangement: two synced panes, or the union on one canvas.
export type DiffMode = "side" | "overlay";

// DiffState is what the diff panels render: whether a comparison is open, the two file
// labels, the sheet pairing and which pair is shown, the legend, the changed-item list and
// which item is focused (WS9-006, "" for none), and the error when the comparison itself
// failed (error and pairs are mutually exclusive). One state feeds both the chrome bar and
// the changes panel.
export interface DiffState {
  active: boolean;
  aLabel: string;
  bLabel: string;
  pairs: SheetPair[];
  activePair: number; // index into pairs, -1 when none
  legend: LegendEntry[];
  items: ChangedItem[];
  selected: string; // itemId of the focused item, "" when none
  // mode is the canvas arrangement; overlayOk says whether the union mode is offered for
  // the CURRENT pair, and overlayReason (set only when not ok) explains the disabled toggle.
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
  // focusItem is the changed item being emphasized ("" selection when null): while set, the
  // overlays show just it instead of every change.
  private focusItem: ChangedItem | null = null;
  private active = false;
  private error = "";
  // mode is the canvas arrangement (WS9-007). The current pair's sheet documents are
  // retained (docA/docB, null = placeholder with errA/errB) so a mode toggle redraws from
  // memory and only the overlays are re-fetched; their frames plus the response's shared
  // placements are the alignment evidence behind overlayOk/overlayReason.
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

  // open runs the comparison: one DiffDesigns call for the report + highlight maps, one
  // GetDesign per side for the sheet lists, then the first pair with both sides present is
  // rendered (falling back to the first pair at all). Either file failing to diff or load
  // fails the whole open — there is no partial diff (mirroring the RPC's contract) — and the
  // panel shows the error while both sides show a placeholder.
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

  // selectItem focuses one changed item (the changes panel's intent): the overlays switch to
  // emphasizing just it, and the view reveals its location. When the current sheet pair does
  // not show the item, it first switches to the first pair that does (the sheet maps drive
  // this; an item with no sheet entries — no geometry, KiCad nets pre-WS1-022 — stays on the
  // current pair and emphasizes there). Clicking the focused item again clears the focus and
  // restores the all-changes overlays. An explicit pair (a sheet-badge click) never toggles
  // off, mirroring the findings panel's badge semantics.
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

  // revealFocus asks the side that owns the focused item to center on it: the old side for
  // entities that only exist there (removed/deleted), the new side otherwise. The view
  // mirrors the resulting camera to its sibling, so both panes land on the location. In
  // overlay mode everything draws on the a-canvas, so that is always the reveal target.
  private revealFocus(item: ChangedItem): void {
    if (this.mode === "overlay") {
      this.view.a.reveal();
      return;
    }
    const side = item.cls === "removed" || item.cls === "deleted" ? this.view.a : this.view.b;
    side.reveal();
  }

  // setMode switches between the two synced panes and the single union canvas (WS9-007).
  // Both sheet documents are retained, so the toggle redraws from memory and re-fetches
  // only the overlays — no sheet reload. Entering overlay mode while the current pair is
  // not aligned is ignored (the toggle is disabled with the reason in the UI).
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

  // selectPair shows another sheet pair (the selector's intent). Out-of-range indices are
  // ignored rather than clearing the view.
  async selectPair(i: number): Promise<void> {
    if (i < 0 || i >= this.pairs.length || i === this.activePair) return;
    this.activePair = i;
    this.pushState();
    await this.showPair(i);
  }

  // close ends the comparison and resets to the inactive state. The host owns the panel
  // chrome (closing the dock panel); this only clears the presenter's state.
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

  // showPair renders one sheet pair: both documents are fetched (and retained, with their
  // frames — the alignment evidence and the mode toggle both need them), the pair's overlay
  // availability is re-judged, and the active mode draws. A pair that stops being aligned
  // while in overlay mode falls back to side-by-side (the state carries the reason).
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

  // fetchDoc loads one side's sheet document into the retained slot; a missing sheet or a
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

  // showDoc reveals one side's retained document (or its placeholder).
  private showDoc(side: DiffSide): void {
    const v = side === "a" ? this.view.a : this.view.b;
    const doc = side === "a" ? this.docA : this.docB;
    if (doc !== null) v.showSvg(doc);
    else v.showPlaceholder(side === "a" ? this.errA : this.errB);
  }

  // renderUnion draws the single-canvas union (WS9-007) into the a-canvas: b's document is
  // the neutral base (it holds unchanged + added + changed geometry), then b's highlight
  // overlay and a's removed-ghost overlay stack above it. Requires b's document — a pair
  // with no b side is never aligned, so this is unreachable then, but guard anyway.
  private async renderUnion(p: SheetPair): Promise<void> {
    this.view.setOverlayMode(true);
    if (this.docB === null) {
      this.view.a.showPlaceholder(this.errB);
      return;
    }
    this.view.a.showSvg(this.docB);
    await this.sendUnionOverlays(p);
  }

  // sendUnionOverlays fetches the union canvas's two layers: b's specs joined to b's sheet,
  // and the removed/deleted ghosts joined to a's sheet (aligned frames make them land in
  // place). A focused item narrows both layers the same way the side-by-side overlays do.
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

  // specsFor is what a side currently highlights: the focused item alone while one is
  // selected (its class colors, side-filtered), else every change on that side.
  private specsFor(side: DiffSide) {
    return this.focusItem ? focusSpecs(this.focusItem, side) : sideSpecs(this.componentStatus, this.netStatus, side);
  }

  // refreshOverlays re-fetches just the highlight layers for the current pair (a focus
  // change on an unchanged sheet), skipping the sheet documents. Mode-aware: in overlay
  // mode the union layers are recomposed instead of the per-side overlays.
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

  // sendOverlay fetches and stacks one side's highlight overlay. A failed overlay just
  // leaves the sheet unhighlighted — the sheet is still worth seeing; no specs clears it.
  private async sendOverlay(side: DiffSide, ref: DiffFileRef, layout: string, sheetId: string): Promise<void> {
    const v = side === "a" ? this.view.a : this.view.b;
    const specs = this.specsFor(side);
    if (specs.length === 0) {
      v.setOverlay("");
      return;
    }
    v.setOverlay(await this.fetchOverlay(ref, layout, sheetId, specs));
  }

  // fetchOverlay is the one HighlightSheet call: returns the overlay document, or "" when
  // the fetch fails (the sheet is still worth seeing unhighlighted).
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

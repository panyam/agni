// The read-only design preview behind the /designs browse page (WS9-049 phase 2). It renders one
// static picture of a design's first sheet and nothing else: no presenter, no WebGL context, no
// checks/query/diff machinery, no render-mode or layout controls. Browsing is about deciding what
// to open, so the preview answers "is this the design I mean?" and stops there.
//
// It lives apart from browse.ts (the page's boot wiring) so the load sequence is unit-testable
// against fake clients without standing up a DOM root.
import type { Client } from "@connectrpc/connect";
import { artifactUri } from "./uri.js";
import { DesignService, SheetFormat } from "./gen/agni/v1/webapi/design_pb.js";

type DesignClient = Client<typeof DesignService>;

// PreviewView is the preview's rendering surface: a plain view adapter, no framework and no
// presenter coupling (the same shape SvgView already satisfies for the viewer).
export interface PreviewView {
  // showSvg reveals a rendered sheet document, replacing whatever the stage held.
  showSvg(markup: string): void;
  // showNote replaces the stage with a status line. "error" is styled distinctly; the distinction
  // is real and not cosmetic, because "this file has no drawable sheet" is a fact about the design
  // while a transport failure is a fact about this attempt.
  showNote(text: string, kind: "info" | "error"): void;
  // setCaption sets the preview header: the design's display name and a one-line summary. Both are
  // "" when nothing is selected, which is also the signal to disable the Open action.
  setCaption(name: string, summary: string): void;
}

// captionFor builds the preview's one-line summary. It deliberately does NOT reuse the viewer's
// summaryLine: importing viewer.ts here would pull ViewerPresenter and its whole transitive graph
// (canvas, webgl, findings, highlights) into the browse bundle, which is exactly the coupling this
// page exists to avoid. The lines also differ — the browse page has no layout knobs to report, so
// naming the effective layout would describe a control the reader cannot see.
//
// The sheet count leads because it is the only field GetDesign fills for EVERY design. The netlist
// fields are populated only when the server resolves an AUTO-layout: a design that carries its own
// geometry takes the faithful branch, which reports the design ref and leaves format and both
// counts zero. Keying the whole line on those fields left it blank for every KiCad schematic —
// that is, for the common case.
export function captionFor(format: string, comps: number, nets: number, sheets: number): string {
  const parts: string[] = [];
  if (format) parts.push(format);
  if (comps) parts.push(`${comps} components`);
  if (nets) parts.push(`${nets} nets`);
  if (sheets) parts.push(sheets === 1 ? "1 sheet" : `${sheets} sheets`);
  return parts.join(" · ");
}

// BOARD_SHEET_ID is the synthetic sheet a board sidecar contributes, mirroring boardSheetID in
// internal/service/design.go. It is deliberately non-numeric so it cannot collide with a sheet index.
const BOARD_SHEET_ID = "board";

// PreviewSheets is the slice of GetDesignResponse pickPreviewSheet reads.
interface PreviewSheets {
  sheets: { id: string; name?: string }[];
  availableLayouts?: string[];
}

// pickPreviewSheet chooses which sheet stands for a design in the browse list.
//
// It is NOT simply the first sheet. GetDesign appends the synthetic board sheet AFTER the drawable
// ones, so a board file (.kicad_pcb, IPC-2581) answers with [netlist graph, board]: its first sheet
// is a synthetic auto-layout of the board's netlist, and previewing that shows a schematic-shaped
// graph for a file whose whole content is a physical board.
//
// The discriminator is whether "faithful" is an available layout, not the file extension — a
// question about what the design CARRIES rather than what it is named, so it holds for every board
// format without a per-format list. A board file offers only auto-layouts, so its board sheet is
// the only faithful drawing it has. A schematic with a board sidecar offers "faithful" and keeps
// its schematic, which is what someone browsing a .kicad_sch means by that file.
export function pickPreviewSheet(d: PreviewSheets): { id: string; name?: string } | undefined {
  const hasFaithful = (d.availableLayouts ?? []).includes("faithful");
  if (!hasFaithful) {
    const board = d.sheets.find((s) => s.id === BOARD_SHEET_ID);
    if (board) return board;
  }
  return d.sheets[0];
}

// DesignPreview loads and shows one design at a time.
export class DesignPreview {
  // seq is the staleness token. Clicking down a file list starts a load per file and the responses
  // can land out of order (a small design behind a large one returns first), so each load captures
  // the token it started with and drops its own result once a newer load has begun. Without it the
  // stage can settle on a design the user already moved off.
  private seq = 0;

  constructor(
    private readonly client: DesignClient,
    private readonly view: PreviewView,
  ) {}

  // show renders the sheet that stands for a design. It is two calls, not one, because the sheet
  // has to be named explicitly: GetSheet's empty selector means "index 0 of the geometry for this
  // layout", which reaches neither the synthetic board sheet nor pickPreviewSheet's choice between
  // them. GetDesign is what knows the sheet list.
  //
  // Both calls request an empty layout, which is what makes the preview faithful-first for free:
  // the server's layoutForFile resolves "" to the faithful layout whenever the file carries
  // geometry, and falls back to the default auto-layout when it does not.
  async show(mount: string, path: string): Promise<void> {
    const token = ++this.seq;
    this.view.setCaption(baseName(path), "");
    this.view.showNote("rendering preview…", "info");
    try {
      const d = await this.client.getDesign({ uri: artifactUri(mount, path), layout: "" });
      if (token !== this.seq) return;
      const sheet = pickPreviewSheet(d);
      if (!sheet) {
        this.view.showNote("This file has no drawable sheet.", "info");
        return;
      }
      const resp = await this.client.getSheet({ uri: artifactUri(mount, path), sheet: sheet.id, layout: "", format: SheetFormat.SVG });
      if (token !== this.seq) return;
      if (resp.content.case !== "svg" || !resp.content.value) {
        this.view.showNote("The server returned no drawing for this sheet.", "error");
        return;
      }
      this.view.setCaption(d.name || baseName(path), captionFor(d.sourceFormat, d.componentCount, d.netCount, d.sheets.length));
      this.view.showSvg(resp.content.value);
    } catch (e) {
      if (token !== this.seq) return;
      this.view.showNote(String(e), "error");
    }
  }

  // clear returns the stage to its empty state and abandons any load in flight, so a folder
  // selection cannot be overwritten moments later by the previously selected design.
  clear(): void {
    this.seq++;
    this.view.setCaption("", "");
    this.view.showNote("Select a design to preview it.", "info");
  }
}

// baseName is the file's own name, used as the caption until the server reports the design's
// declared name (which many formats leave empty).
function baseName(path: string): string {
  const i = path.lastIndexOf("/");
  return i < 0 ? path : path.slice(i + 1);
}

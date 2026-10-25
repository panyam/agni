// PropertiesPresenter keeps what the Properties panel describes and pushes it (agni issue 902).
//
// It is its own presenter rather than more state on ViewerPresenter, which already holds too much
// (agni issue 390). It listens on the channels the viewer presenter already fans out (sheets, the
// summary line, the conversion report, findings, the datasheet join) and makes two reads of its own:
// the design's IR under INVENTORY_PATHS once per open design, and where a selected part or net is
// drawn, once per selection.

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { ConversionReport, GetDesignRequestSchema, GetDesignResponse } from "./gen/agni/v1/webapi/design_pb.js";
import type { FindingsState, VerdictItem } from "./findings.js";
import type { EntityQueryItem } from "./query.js";
import {
  type DatasheetJoin,
  type Focus,
  INVENTORY_PATHS,
  type Inventory,
  type SelectionChecks,
  SERVED_QUERIES,
  inventoryFromDesign,
  propertiesFor,
} from "./properties.js";
import type { PropertiesView } from "./propertiespanel.js";
import { type Selection, sameSelection } from "./selection.js";
import type { SubjectRef } from "./router.js";
import { artifactUri } from "@agni/web-shared/uri.js";

export interface PropertiesSources {
  // getDesign is DesignService.GetDesign of the design uri names, on whichever engine serves the page,
  // asking for readMask's paths. The source names the snapshot (agni issue 1061).
  getDesign: (uri: string, readMask: string[]) => Promise<GetDesignResponse>;
  // drawn reads what the sheet on screen draws, or null when no drawing is on screen.
  drawn: () => { refs: string[]; nets: string[] } | null;
  // drawnOn asks the engine which sheets draw a component or a net. Absent, the panel lists none.
  drawnOn?: (uri: string, sel: Selection) => Promise<string[]>;
  // ask runs a query on the engine serving the page and returns its rows' cells, which is how the
  // inventory gets the pin names and test points the engine decides (SERVED_QUERIES). Absent, or
  // failing, the panel shows no pin names and no test points rather than guessing them.
  ask?: (uri: string, query: string) => Promise<string[][]>;
  // check asks the selected rules about one net, part or pin alone (agni issue 897), null when no rule
  // is selected. Absent, the panel lists the run's findings for the selection instead.
  check?: (subject: SubjectRef) => Promise<{ verdicts: VerdictItem[]; notAsked: { name: string; reason: string }[] } | null>;
  // checkKey names what a check is asked under (the selected rules and the vocabulary), so the panel
  // asks again when it changes.
  checkKey?: () => string;
  // onInventory hears each design's inventory once it is read, and null when a new design clears it,
  // so another panel (the trace pickers, agni issue 1133) reads the same index rather than asking again.
  onInventory?: (inv: Inventory | null) => void;
  // onSelect hears every focus the panel takes, so a viewer room can tell agents what the reader
  // selected (agni issue 1173).
  onSelect?: (focus: Focus) => void;
}

// subjectOf is the subject a selection's checks are asked about, or null for a bus, which no scope
// covers.
export function subjectOf(sel: Selection): SubjectRef | null {
  switch (sel.kind) {
    case "net":
      return sel.net ? { kind: "net", ref: sel.net, pin: "" } : null;
    case "component":
      return sel.ref ? { kind: "component", ref: sel.ref, pin: "" } : null;
    case "pin":
      return sel.ref && sel.pin ? { kind: "pin", ref: sel.ref, pin: sel.pin } : null;
    default:
      return null;
  }
}

export class PropertiesPresenter {
  private mount = "";
  private path = "";
  private sheets: { id: string; name: string }[] = [];
  private activeSheet = "";
  private summary = "";
  private designName = "";
  private report: ConversionReport | null = null;
  private findings: FindingsState | null = null;
  private datasheet: DatasheetJoin | null = null;
  private inventory: Inventory | null = null;
  // inventoryKey is the design the inventory (or its fetch in flight) belongs to, so a slow answer
  // for a design the reader has since left never lands on the new one.
  private inventoryKey = "";
  private focus: Focus = { level: "design" };
  private questions: EntityQueryItem[] = [];
  // located is where the selected entity is drawn, for the selection it was asked for, so an answer
  // that arrives after the reader moved on is dropped rather than shown against the new selection.
  private located: { sel: Selection; sheets: string[] } | null = null;
  // checked is the selection's own check answer, for the selection and the rules it was asked about,
  // so an answer for a selection the reader left, or under rules since changed, is never shown.
  private checked: { sel: Selection; key: string; value: SelectionChecks } | null = null;

  constructor(
    private readonly view: PropertiesView,
    private readonly src: PropertiesSources,
  ) {}

  // setSheets is the sheet channel. A new design resets the selection to the design and starts its
  // inventory read. A sheet focus follows the sheet on screen, since it names the sheet being looked
  // at; an entity focus stays put, since it names a thing rather than a place.
  setSheets(s: { mount: string; path: string; sheets: { id: string; name: string }[]; activeId: string }): void {
    const changed = s.mount !== this.mount || s.path !== this.path;
    this.mount = s.mount;
    this.path = s.path;
    this.sheets = s.sheets;
    this.activeSheet = s.activeId;
    if (changed) {
      this.focus = { level: "design" };
      this.located = null;
      this.checked = null;
      this.inventory = null;
      this.src.onInventory?.(null);
      this.designName = "";
      this.datasheet = null;
      void this.loadInventory();
    } else if (this.focus.level === "sheet" && s.activeId) {
      this.focus = { level: "sheet", sheetId: s.activeId };
    }
    this.push();
  }

  setSummary(text: string): void {
    this.summary = text;
    this.push();
  }

  setReport(report: ConversionReport | null): void {
    this.report = report;
    this.push();
  }

  setFindings(state: FindingsState): void {
    const runEnded = (this.findings?.running ?? false) && !state.running;
    this.findings = state;
    this.push();
    // The rule selection or the vocabulary may have changed with it, which the answer was asked under.
    // A failed answer is asked again when a run ends, since a cancelled run ends the worker a
    // selection's check was queued on (agni issue 918); a design that cannot be checked at all fails
    // once per run rather than on every push.
    const failedByRun = runEnded && this.checked?.value.state === "failed";
    if (this.focus.level === "entity" && this.checked && (this.checked.key !== this.checkKey() || failedByRun)) void this.check(this.focus.sel);
  }

  setDatasheet(join: DatasheetJoin): void {
    this.datasheet = join;
    this.push();
  }

  // setQuestions takes the served questions about a selection, which the Ask rows list.
  setQuestions(questions: EntityQueryItem[]): void {
    this.questions = questions;
    this.push();
  }

  select(focus: Focus): void {
    this.focus = focus;
    this.push();
    this.src.onSelect?.(focus);
    if (focus.level === "entity") {
      void this.locate(focus.sel);
      void this.check(focus.sel);
    }
  }

  // selectEntity is a pick from the drawing or a result cell, made on the sheet on screen.
  selectEntity(sel: Selection): void {
    this.select({ level: "entity", sel, sheetId: this.activeSheet || undefined });
  }

  // selectSheet is a click on empty space, which names the sheet the reader is looking at.
  selectSheet(): void {
    this.select(this.activeSheet ? { level: "sheet", sheetId: this.activeSheet } : { level: "design" });
  }

  selectDesign(): void {
    this.select({ level: "design" });
  }

  currentFocus(): Focus {
    return this.focus;
  }

  // pinNets is a part's pins and the nets they land on, from the inventory this panel read, so an area
  // can name every net it touches (agni issue 904). Empty until the inventory arrives.
  pinNets(ref: string): { pin: string; net: string }[] {
    return this.inventory?.pinsOf.get(ref) ?? [];
  }

  // activeSheetId is the sheet on screen, as the sheet channel last said.
  activeSheetId(): string {
    return this.activeSheet;
  }

  // locate asks where a selected part or net is drawn. A pin is described under its part and a bus by
  // its members, so neither asks.
  private async locate(sel: Selection): Promise<void> {
    if (this.located && sameSelection(this.located.sel, sel)) return;
    this.located = null;
    if (!this.src.drawnOn || !this.mount || !this.path) return;
    if (sel.kind !== "component" && sel.kind !== "net") return;
    try {
      const sheets = await this.src.drawnOn(artifactUri(this.mount, this.path), sel);
      if (!this.isSelected(sel)) return;
      this.located = { sel, sheets };
    } catch {
      if (!this.isSelected(sel)) return;
      this.located = { sel, sheets: [] };
    }
    this.push();
  }

  private checkKey(): string {
    return this.src.checkKey?.() ?? "";
  }

  // check asks the selected rules about the selection alone (agni issue 897). An answer arriving after
  // the reader moved on, or after the rules changed, is dropped, since the next ask is already out.
  private async check(sel: Selection): Promise<void> {
    const subject = subjectOf(sel);
    if (!this.src.check || !subject) {
      this.checked = null;
      return;
    }
    const key = this.checkKey();
    if (this.checked && sameSelection(this.checked.sel, sel) && this.checked.key === key && this.checked.value.state !== "failed") return;
    this.checked = { sel, key, value: { state: "asking" } };
    this.push();
    let value: SelectionChecks;
    try {
      const r = await this.src.check(subject);
      value = r ? { state: "answered", verdicts: r.verdicts, notAsked: r.notAsked } : { state: "no-rules" };
    } catch (e) {
      value = { state: "failed", error: e instanceof Error ? e.message : String(e) };
    }
    if (!this.isSelected(sel) || this.checked?.key !== key || !sameSelection(this.checked.sel, sel)) return;
    this.checked = { sel, key, value };
    this.push();
  }

  // checks is the selection's check answer for the panel: undefined where nothing asks, so the panel
  // falls back to the run's findings.
  private checks(): SelectionChecks | undefined {
    if (this.focus.level !== "entity" || !this.checked) return undefined;
    return sameSelection(this.checked.sel, this.focus.sel) ? this.checked.value : undefined;
  }

  // drawnOn is what the panel says about where the selection is drawn: nothing without an engine to
  // ask, null while the answer is on its way.
  private drawnOn(): string[] | null | undefined {
    if (!this.src.drawnOn) return undefined;
    if (this.focus.level !== "entity") return null;
    return this.located && sameSelection(this.located.sel, this.focus.sel) ? this.located.sheets : null;
  }

  private isSelected(sel: Selection): boolean {
    return this.focus.level === "entity" && sameSelection(this.focus.sel, sel);
  }

  private async loadInventory(): Promise<void> {
    if (!this.mount || !this.path) return;
    const key = artifactUri(this.mount, this.path);
    this.inventoryKey = key;
    try {
      const asked = (q: string) => (this.src.ask ? this.src.ask(key, q).catch(() => [] as string[][]) : Promise.resolve([] as string[][]));
      const [resp, names, tps] = await Promise.all([
        this.src.getDesign(key, ["name", ...INVENTORY_PATHS]),
        asked(SERVED_QUERIES.pinNames),
        asked(SERVED_QUERIES.testPoints),
      ]);
      if (this.inventoryKey !== key) return;
      this.designName = resp.name;
      const served = {
        pinNames: names.map((c) => [c[0] ?? "", c[1] ?? "", c[2] ?? ""] as [string, string, string]),
        testPoints: tps.map((c) => c[0] ?? ""),
      };
      this.inventory = resp.design ? inventoryFromDesign(resp.design, served) : null;
    } catch {
      // A design whose IR cannot be read still has a drawing and findings to describe, so the panel
      // goes on without the inventory and says so per entity.
      if (this.inventoryKey !== key) return;
      this.inventory = null;
    }
    this.src.onInventory?.(this.inventory);
    this.push();
  }

  private push(): void {
    this.view.setState(
      propertiesFor({
        focus: this.focus,
        designName: this.designName || this.path.split("/").pop() || "",
        summary: this.summary,
        sheets: this.sheets,
        report: this.report,
        inventory: this.inventory,
        findings: this.findings,
        datasheet: this.datasheet,
        drawn: this.focus.level === "sheet" && this.focus.sheetId === this.activeSheet ? this.src.drawn() : null,
        questions: this.questions,
        drawnOn: this.drawnOn(),
        checks: this.checks(),
      }),
    );
  }
}

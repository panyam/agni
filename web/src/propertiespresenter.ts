// PropertiesPresenter keeps what the Properties panel describes and pushes it (agni issue 902).
//
// It is its own presenter rather than more state on ViewerPresenter, which already holds too much
// (agni issue 390). It listens on channels the viewer presenter already fans out, which are sheets,
// the summary line, the conversion report, findings and the datasheet join. It also makes two reads
// of its own. It reads the design's IR under INVENTORY_PATHS once per open design, and it reads
// where a selected part or net is drawn, once per selection.

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
  // getDesign calls DesignService.GetDesign for the design that uri names, on whichever engine
  // serves the page, and asks for readMask's paths. The source names the snapshot (agni issue
  // 1061).
  getDesign: (uri: string, readMask: string[]) => Promise<GetDesignResponse>;
  // drawn reads what the sheet on screen draws, or null when no drawing is on screen.
  drawn: () => { refs: string[]; nets: string[] } | null;
  // drawnOn asks the engine which sheets draw a component or a net. Absent, the panel lists none.
  drawnOn?: (uri: string, sel: Selection) => Promise<string[]>;
  // ask runs a query on the engine serving the page and returns its rows' cells. The inventory gets
  // its pin names and test points this way, as the engine decides them (SERVED_QUERIES). Absent, or
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

// subjectOf returns the subject to ask a selection's checks about. It returns null for a bus, since
// no scope covers one.
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

// areaSubjects names each part, loose pin and net in an area, so the engine can say which sheets
// draw any of them (agni issue 1589). A loose pin is "REF.PIN" in the area.
export function areaSubjects(sel: Selection): SubjectRef[] {
  const out: SubjectRef[] = [];
  for (const ref of sel.refs ?? []) out.push({ kind: "component", ref, pin: "" });
  for (const p of sel.pins ?? []) {
    const dot = p.lastIndexOf(".");
    if (dot > 0) out.push({ kind: "pin", ref: p.slice(0, dot), pin: p.slice(dot + 1) });
  }
  for (const net of sel.nets ?? []) out.push({ kind: "net", ref: net, pin: "" });
  return out;
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
  // inventoryKey names the design that owns the inventory or the fetch in flight. A slow answer can
  // arrive after the reader has left its design, and the key keeps it off the new one.
  private inventoryKey = "";
  private focus: Focus = { level: "design" };
  private questions: EntityQueryItem[] = [];
  // located is where the selected entity is drawn, for the selection it was asked for, so an answer
  // that arrives after the reader moved on is dropped rather than shown against the new selection.
  private located: { sel: Selection; sheets: string[] } | null = null;
  // checked is the selection's own check answer, kept with the selection and the rules it was asked
  // about. An answer is never shown once the reader has left its selection or the rules have
  // changed.
  private checked: { sel: Selection; key: string; value: SelectionChecks } | null = null;

  constructor(
    private readonly view: PropertiesView,
    private readonly src: PropertiesSources,
  ) {}

  // setSheets is the sheet channel. A new design resets the selection to the design and starts its
  // inventory read. A sheet focus follows the sheet on screen, since it names the sheet being
  // looked at. An entity focus does not move, since it names a thing rather than a place.
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

  // reloadInventory reads the inventory again after the vocabulary changed. The read stamps a net's
  // roles and a part's classes under the convention it names.
  reloadInventory(): void {
    this.checked = null;
    void this.loadInventory();
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
    // The answer was asked under a rule selection and a vocabulary, and either may have changed
    // with this push. A failed answer is asked again when a run ends. A selection's check is queued
    // on a worker, and a cancelled run ends that worker (agni issue 918). A design that cannot be
    // checked at all fails once per run rather than on every push.
    const failedByRun = runEnded && this.checked?.value.state === "failed";
    if (this.focus.level === "entity" && this.checked && (this.checked.key !== this.checkKey() || failedByRun)) void this.check(this.focus.sel);
  }

  setDatasheet(join: DatasheetJoin): void {
    this.datasheet = join;
    this.push();
  }

  // setQuestions takes the served questions about a selection, for the Ask rows to list.
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

  // pinNets lists a part's pins and the nets they connect to, from this panel's inventory, so an
  // area can name every net it touches (agni issue 904). Empty until the inventory arrives.
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

  // checks returns the selection's check answer for the panel. It is undefined where nothing asks,
  // and the panel then falls back to the run's findings.
  private checks(): SelectionChecks | undefined {
    if (this.focus.level !== "entity" || !this.checked) return undefined;
    return sameSelection(this.checked.sel, this.focus.sel) ? this.checked.value : undefined;
  }

  // drawnOn says where the selection is drawn, for the panel. It is undefined without an engine to
  // ask, and null while the answer is on its way.
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

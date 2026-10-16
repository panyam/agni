// PropertiesPresenter keeps what the Properties panel describes and pushes it (agni issue 902).
//
// It is its own presenter rather than more state on ViewerPresenter, which already holds too much
// (agni issue 390). It listens on the channels the viewer presenter already fans out (sheets, the
// summary line, the conversion report, findings, the datasheet join) and makes two reads of its own:
// the design's IR under INVENTORY_PATHS once per open design, and where a selected part or net is
// drawn, once per selection.

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { ConversionReport, GetDesignRequestSchema, GetDesignResponse } from "./gen/agni/v1/webapi/design_pb.js";
import type { FindingsState } from "./findings.js";
import type { EntityQueryItem } from "./query.js";
import {
  type DatasheetJoin,
  type Focus,
  INVENTORY_PATHS,
  type Inventory,
  SERVED_QUERIES,
  inventoryFromDesign,
  propertiesFor,
} from "./properties.js";
import type { PropertiesView } from "./propertiespanel.js";
import { type Selection, sameSelection } from "./selection.js";
import { artifactUri } from "@agni/web-shared/uri.js";

export interface PropertiesSources {
  // getDesign is DesignService.GetDesign on whichever engine serves the page.
  getDesign: (req: MessageInitShape<typeof GetDesignRequestSchema>) => Promise<GetDesignResponse>;
  // drawn reads what the sheet on screen draws, or null when no drawing is on screen.
  drawn: () => { refs: string[]; nets: string[] } | null;
  // drawnOn asks the engine which sheets draw a component or a net. Absent, the panel lists none.
  drawnOn?: (uri: string, sel: Selection) => Promise<string[]>;
  // ask runs a query on the engine serving the page and returns its rows' cells, which is how the
  // inventory gets the pin names and test points the engine decides (SERVED_QUERIES). Absent, or
  // failing, the panel shows no pin names and no test points rather than guessing them.
  ask?: (uri: string, query: string) => Promise<string[][]>;
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
      this.inventory = null;
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
    this.findings = state;
    this.push();
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
    if (focus.level === "entity") void this.locate(focus.sel);
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
        this.src.getDesign({ uri: key, readMask: { paths: ["name", ...INVENTORY_PATHS] } }),
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
      }),
    );
  }
}

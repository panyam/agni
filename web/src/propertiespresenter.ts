// PropertiesPresenter keeps what the Properties panel describes and pushes it (agni issue 902).
//
// It is its own presenter rather than more state on ViewerPresenter, which already holds too much
// (agni issue 390). It listens on the channels the viewer presenter already fans out (sheets, the
// summary line, the conversion report, findings, the datasheet join) and makes one read of its own,
// the design's IR under INVENTORY_PATHS, once per open design.

import type { MessageInitShape } from "@bufbuild/protobuf";
import type { ConversionReport, GetDesignRequestSchema, GetDesignResponse } from "./gen/agni/v1/webapi/design_pb.js";
import type { FindingsState } from "./findings.js";
import {
  type DatasheetJoin,
  type Focus,
  INVENTORY_PATHS,
  type Inventory,
  inventoryFromDesign,
  propertiesFor,
} from "./properties.js";
import type { PropertiesView } from "./propertiespanel.js";
import type { Selection } from "./selection.js";
import { artifactUri } from "@agni/web-shared/uri.js";

export interface PropertiesSources {
  // getDesign is DesignService.GetDesign on whichever engine serves the page.
  getDesign: (req: MessageInitShape<typeof GetDesignRequestSchema>) => Promise<GetDesignResponse>;
  // drawn reads what the sheet on screen draws, or null when no drawing is on screen.
  drawn: () => { refs: string[]; nets: string[] } | null;
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

  select(focus: Focus): void {
    this.focus = focus;
    this.push();
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

  private async loadInventory(): Promise<void> {
    if (!this.mount || !this.path) return;
    const key = artifactUri(this.mount, this.path);
    this.inventoryKey = key;
    try {
      const resp = await this.src.getDesign({ uri: key, readMask: { paths: ["name", ...INVENTORY_PATHS] } });
      if (this.inventoryKey !== key) return;
      this.designName = resp.name;
      this.inventory = resp.design ? inventoryFromDesign(resp.design) : null;
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
      }),
    );
  }
}

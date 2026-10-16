import { Code, ConnectError, type Client } from "@connectrpc/connect";
import { emptyProject, type ProjectState, type ProjectBarView } from "./project.js";
import { artifactUri, uriPath } from "@agni/web-shared/uri.js";
import { DesignService, SheetFormat, SymbolSource, type SheetRef, type ConversionReport } from "./gen/agni/v1/webapi/design_pb.js";
import { CheckService, CheckReportFormat, CheckDesignResponseSchema, type CheckDesignResponse } from "./gen/agni/v1/webapi/checks_pb.js";
import { BROWSER_MOUNT } from "./wasm/drop.js";
import { QueryService, TextPositionSchema, type QuerySet, type RunQueryResponse } from "./gen/agni/v1/webapi/query_pb.js";
import type { TableAnswer } from "./tables.js";
import { ReviewService, type Review } from "./gen/agni/v1/webapi/review_pb.js";
import { ProjectService } from "./gen/agni/v1/webapi/project_pb.js";
import type { ResolveDesignResponse } from "./gen/agni/v1/webapi/project_pb.js";
import { OverlayConfigSchema, type OverlayConfig } from "./gen/agni/v1/webapi/checks_pb.js";
import { type NamingConvention } from "./gen/agni/v1/config/naming_pb.js";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import type { CanvasComponent } from "./canvas.js";
import type { SheetsView } from "./sheets.js";
import type { ControlsView } from "./controls.js";
import { type ViewerLocation, bindsFromLink } from "./router.js";
import { DocumentStore } from "./document.js";
import { type FindingItem, type FindingsState, type FindingsView, type SheetBadge, type VerdictItem, subjectsToSpecs, findingSpec, entitySpecs, focusStack, selectionFromFinding, contextFromWire, outcomeWord, verdictProofStack } from "./findings.js";
import { sheetTiles, type OverviewView } from "./sheetoverview.js";
import { reconcile, expectationSpecs, expectationCaption, type RuleExpectationItem, type ExpectationRow, type ExpectationCaption } from "./expectations.js";
import { undrawnNote, type UndrawnNote } from "./undrawn.js";
import { staleLinkNote, type StaleLinkNote } from "./stalelink.js";
import { type RuleItem, type RulesView, defaultSelection } from "./rules.js";
import type { Selection } from "./selection.js";
import { withFocusShape, type FocusStyle, type HighlightSpec } from "./highlights.js";
import { type QueryResult, type QueryView, LocateReason, emptyResult, errorResult, reasonMessage, resultFromResponse } from "./query.js";
import { type CoverageView, coverageFromResponse, emptyCoverage } from "./coverage.js";
import { type TraceView, errorTrace, loadingTrace, parseEndpoint, splitTraceParam, traceFromResponse, traceSubjects , traceSheet} from "./trace.js";
import { type PartsView, partsFromResponse, emptyParts } from "./parts.js";
import { create } from "@bufbuild/protobuf";
import { type ConventionBarView } from "./conventions.js";
import {
  type ChecklistOption,
  type ReviewState,
  type ReviewView,
  checklistOptions,
  projectChecklistOptions,
  emptyReview,
  reviewFromWire,
} from "./review.js";

type DesignClient = Client<typeof DesignService>;
type CheckClient = Client<typeof CheckService>;
type QueryClient = Client<typeof QueryService>;
type ReviewClient = Client<typeof ReviewService>;
type ProjectClient = Client<typeof ProjectService>;
type WorkspaceClient = Client<typeof WorkspaceService>;

// RenderMode selects which renderer draws the current sheet: the WebGL packer, the SVG
// verification backend, or the format's native tool (the golden reference). It maps to a
// GetSheet SheetFormat.
export type RenderMode = "webgl" | "svg" | "native";

function formatFor(mode: RenderMode): SheetFormat {
  switch (mode) {
    case "svg":
      return SheetFormat.SVG;
    case "native":
      return SheetFormat.NATIVE;
    default:
      return SheetFormat.PACKED;
  }
}

// RenderView is the command-down surface the presenter uses to reveal a rendered sheet,
// without touching the DOM itself (CONSTRAINTS C3). The composition root implements it over
// the canvas and the SVG host.
export interface RenderView {
  showWebgl(): void;
  showSvg(markup: string): void;
  // setSvgOverlay stacks a transparent highlight-overlay SVG document (framed exactly like
  // the base document) above the shown sheet; "" clears it. Only meaningful in SVG mode.
  setSvgOverlay(markup: string): void;
  // setBusy shows or hides a loading indicator around a render, which is slow on large schematics
  // and slower for the native shell-out. label names the current phase (loading, rendering, running
  // checks) and is left unchanged when omitted.
  setBusy(busy: boolean, label?: string): void;
  // getView / setView snapshot and restore the active renderer's pan/zoom for the given mode (WebGL
  // -> canvas, SVG/Native -> the SVG host). The value is opaque to the presenter, which stores and
  // replays whatever the renderer returns. getView returns null when there is nothing to snapshot
  // (no sheet drawn yet in that renderer).
  getView(mode: RenderMode): unknown;
  setView(mode: RenderMode, view: unknown): void;
  // setBoardLayers applies board layer visibility ("all" | "front" | "back") to the shown document
  // (WS7-034). It is a pure view concern, since BoardSVG stratifies into classed groups and the
  // host toggles CSS classes with no re-render. A schematic document has no such classes, so the
  // setting is harmless there.
  setBoardLayers(side: string): void;
}

// ViewSink bundles every view surface the presenter pushes state to (WS9-019), one typed object
// instead of a positional callback per panel, so adding a panel is one field here and one line in
// the composition root. Each field is a command-down surface, which the presenter pushes state into
// and never reads back from. A panel field is that panel's framework-neutral view interface, so the
// presenter calls setState and never touches a DOM node or a Solid signal (C3). summary, report,
// and location stay plain functions because their hosts are not islands.
export interface ViewSink {
  // sheetNavs are the sheet-navigation surfaces (the sheet tabs today), kept in sync from this one
  // source, so a second surface joins without presenter changes.
  sheetNavs: SheetsView[];
  summary: (text: string) => void;
  // controls receives the full control-bar state (active mode, native availability, layout
  // axis) whenever any of those change.
  controls: ControlsView;
  // findings receives the findings list + selection.
  findings: FindingsView;
  // expectationCaption receives the conformance sidecar's non-anchored verdict (WS9-045), meaning
  // the set-equality counts and the fires:{} silent assertion, shown as a canvas strip. null hides
  // it when there is no sidecar. The anchored assertions render as the status-colored highlight
  // overlay instead.
  expectationCaption: (caption: ExpectationCaption | null) => void;
  // undrawnNote receives "this drawing is incomplete" for the open design, or null when it is
  // complete (agni issue 354). A render that lost its symbols still draws every ref des and wire, so
  // this is the only thing on screen that can say the sheet is short.
  undrawnNote: (note: UndrawnNote | null) => void;
  // staleLinkNote receives "the link you arrived on was computed against different bytes", or null
  // when there is nothing to say (agni issue 392). Nothing else on screen can say it, because a
  // verdict id is derived from a rule name and a subject ref, so it resolves against an edited
  // design just as readily and draws its proof on whatever now answers to that ref.
  staleLinkNote: (note: StaleLinkNote | null) => void;
  // rules receives the rule catalog + active selection.
  rules: RulesView;
  // report receives the auto-layout conversion report, or null to hide the panel (the
  // faithful layout draws real symbols, so there is nothing to classify).
  report: (report: ConversionReport | null) => void;
  // location receives the URL-addressable state (open file + sheet + view knobs) whenever it
  // changes, so the composition root can reflect it into the browser URL (deep-linkable
  // refresh/back-forward). Optional so tests and non-routed hosts need not wire it.
  location?: (loc: ViewerLocation) => void;
  // overview receives the per-sheet violation tiles (WS9-025) whenever the findings or the
  // shown sheet change. Optional like query.
  overview?: OverviewView;
  // query receives the datalog query results (WS9-036) after each run. Optional, since hosts
  // without the panel need not wire it; the presenter no-ops runQuery when it is absent.
  query?: QueryView;
  // coverage receives the per-interface coverage matrix (WS9-041) on each design load. Optional
  // like query; the presenter no-ops refreshCoverage when it is absent.
  coverage?: CoverageView;
  // trace receives each pin-to-pin answer (agni issue 600). Optional like coverage; runTrace no-ops
  // when the host left the panel out.
  trace?: TraceView;
  // The datasheet-params panel (WS9-035), optional like coverage; refreshParts no-ops when absent.
  parts?: PartsView;
  // review receives the project's checklist verdict for the open design (WS9-052). Optional like
  // coverage; every review method no-ops when it is absent.
  review?: ReviewView;
  // conventionBar reports which naming vocabulary the answers were computed under (WS9-128).
  // Optional like the panels; setConvention no-ops when it is absent, so a host that offers no
  // convention picker always uses the server's default.
  conventionBar?: ConventionBarView;
  // projectBar states which project the open design resolved to and whether the built-in catalog is
  // in effect (agni issue 175). Optional like the rest; a host without it never says.
  projectBar?: ProjectBarView;
  // download hands a file the presenter wrote to the host to save, and linkBase is the viewer's own
  // base URL, which a saved report's rows link back to (agni issue 127). Optional like the panels;
  // saveReport no-ops when the host offers no download.
  download?: (file: SavedFile) => void;
  linkBase?: string;
}

// SavedFile is one file the presenter asks the host to save.
export interface SavedFile {
  filename: string;
  contentType: string;
  content: Uint8Array;
}

// ViewerPresenter coordinates the viewer's semantic loop. An opened file is loaded (GetDesign), its
// sheets fill the navigator, and the chosen sheet is rendered in the active mode (GetSheet ->
// WebGL, or GetSheetSvg -> the SVG reference). It is framework-neutral, calling the canvas, the
// navigator's view interface, and RenderView and never a DOM node or a Solid signal (C3). It runs
// in-process TS because file, sheet and mode switches are low-frequency (C7).
export class ViewerPresenter {
  // doc is the document half (agni issue 390): what is true of the open design and its config. This
  // class is the interaction half and reads it; document.ts reads nothing back.
  readonly doc = new DocumentStore();
  private currentSheet = "";
  // Default to SVG, since it renders faithfully (with text) for every design, where WebGL is
  // currently lower-fidelity and native is per-format/opt-in.
  private mode: RenderMode = "svg";
  // currentLayout is the geometry axis (faithful / grid / layered / ...), empty until the first
  // design load fills it with the effective layout. It is orthogonal to mode.
  private currentLayout = "";
  // faithfulSymbols draws an auto-layout's nodes with the design's own symbols instead of the
  // synthetic glyphs. Only meaningful when the design ships symbols (availableLayouts has
  // "faithful") and the current layout is an auto-layout.
  private faithfulSymbols = false;

  // findings holds the rule findings for the open design, and selectedSubject the one the user
  // clicked (highlighted in the viewer), "" for none.
  private findings: FindingItem[] = [];
  private selectedSubject = "";
  // selectedNetId is the per-instance net id of the focused finding (WS9), so focusing one of two
  // same-named nets targets ITS wires; "" when the focus is a component/pin or a name-only net.
  private selectedNetId = "";
  // checksRunning is true while runChecks is in flight (the on-demand Run), so the panel disables its
  // Run button and the presenter guards against a re-entrant run.
  private checksRunning = false;

  // focusedVerdict is the id currently drawn as a proof, "" when none. It rides the URL so a link
  // reopens on the same proof, and it is cleared by anything that replaces the highlight, since a
  // stale id in the address bar would promise a drawing the canvas is no longer showing.
  private focusedVerdict = "";
  // linkHash is the design revision the viewer's URL claimed, "" when it claimed none.
  // designContentHash is the revision this server actually read, off GetDesign. The stale-link note
  // is computed from the pair, kept apart rather than reduced to a boolean so the note can tell
  // "different bytes" from "could not check".
  //
  // linkHash outlives the focused verdict. The claim is about the DESIGN this viewer read, so it
  // still holds after the reader dismisses a proof and re-focuses it; opening a different design
  // drops it.
  private linkHash = "";

  // tracedPins is the "from,to" the panel last asked about ("" when none), and tracedHops the radius
  // it asked at (0 for the server default). They are URL state in the same category as the focused
  // verdict, naming which question the open viewer is looking at.
  private tracedPins = "";
  private tracedHops = 0;
  // highlights are the active highlight layers (the selection API): each spec names
  // components/nets/pins and its color/alpha. The WebGL canvas resolves them locally against
  // the packed keys; SVG mode composites a server-rendered overlay (HighlightSheet).
  private highlights: HighlightSpec[] = [];
  // The active user focus-highlighter style (WS9-044), or undefined for the built-in look.
  private highlightStyle: FocusStyle | undefined;

  private busyDepth = 0;

  // viewMemory remembers each renderer's pan/zoom keyed by mode|mount|path|sheet, so switching
  // renderer/sheet/file restores where you left it. shownKey/shownMode track what is currently
  // on screen, so its view can be saved just before the next render replaces it.
  private readonly viewMemory = new Map<string, unknown>();
  private shownKey = "";
  private shownMode: RenderMode = "svg";

  // viewKey includes the layout, so a design's faithful view and its grid view remember their
  // pan/zoom independently.
  private viewKey(mode: RenderMode, sheetId: string): string {
    // Include the symbol source, because faithful symbols scale the layout and need their own pan/zoom.
    return `${mode}|${this.currentLayout}|${this.faithfulSymbols ? "f" : "g"}|${this.doc.mount}|${this.doc.path}|${sheetId}`;
  }

  constructor(
    private readonly client: DesignClient,
    // checks is the CheckService client (rules, findings, expectations, the severity report), a
    // separate service (WS9-026).
    private readonly checks: CheckClient,
    private readonly canvas: CanvasComponent,
    private readonly render: RenderView,
    private readonly views: ViewSink,
    // query is the QueryService client for the datalog search panel (WS9-036). Optional so
    // hosts/tests without the panel construct the presenter unchanged; runQuery no-ops without it.
    private readonly query?: QueryClient,
    // reviews is the ReviewService client for the review panel (WS9-052), and workspace lists the
    // design's own directory to find the checklists beside it. Both optional, like query.
    private readonly reviews?: ReviewClient,
    private readonly workspace?: WorkspaceClient,
    // projects resolves the open design to its project, so a review is stored under the project
    // whose config scored it. Optional like the rest; without it a run stores unparented.
    private readonly projects?: ProjectClient,
  ) {}

  // runQuery evaluates an ad-hoc datalog query over the open design (WS9-036) and pushes the result
  // to the query panel. With no file open it reports that as the panel's error rather than calling
  // the service. A parse error or an unloadable design comes back as InvalidArgument, whose message
  // the panel shows inline rather than as a toast. The busy flag rides in the pushed result so the
  // panel disables Run while a query runs.
  //
  // bindings are the values the query's variables are bound to (agni issue 793), sent as text. limit
  // caps the rows (agni issue 896), 0 for the whole answer. tab names the query panel tab that asked
  // (agni issue 902), and the answer goes back to it, "" meaning the tab in front.
  //
  // answers holds each tab's answer as the engine sent it, so Save CSV tabulates the answer the
  // reader is looking at rather than running the query again (agni issues 898 and 956).
  private answers = new Map<string, RunQueryResponse>();

  async runQuery(text: string, bindings: Record<string, string> = {}, limit = 0, tab = ""): Promise<void> {
    if (!this.views.query || !this.query) return;
    const push = (r: QueryResult) => (tab ? this.views.query?.setState(r, tab) : this.views.query?.setState(r));
    if (!this.doc.mount || !this.doc.path) {
      push(errorResult("Open a design first."));
      return;
    }
    push(emptyResult(true));
    try {
      // The overlay goes here too, because the vocabulary bar names the convention the answers on
      // screen were computed under, and a Query panel answering under the server's would contradict
      // it (WS3-113).
      const wire = Object.fromEntries(Object.entries(bindings).map(([k, value]) => [k, { kind: { case: "text" as const, value } }]));
      const resp = await this.query.runQuery({ uri: artifactUri(this.doc.mount, this.doc.path), query: text, overlay: this.overlay(), bindings: wire, limit });
      this.answers.set(tab, resp);
      push(resultFromResponse(resp, (ids) => this.doc.sheetBadges(ids)));
    } catch (e) {
      this.answers.delete(tab);
      // A query that would not parse carries where it stopped, as an error detail.
      const at = e instanceof ConnectError ? e.findDetails(TextPositionSchema)[0] : undefined;
      push(errorResult(e instanceof Error ? e.message : String(e), at?.caret ?? ""));
    }
  }

  // loadQuerySets lists the query sets the design's project keeps (agni issue 745) for the query
  // panel's Sets menu. A failure, or a later design opened meanwhile, leaves the menu with none
  // rather than another design's sets.
  private async loadQuerySets(mount: string, path: string): Promise<void> {
    if (!this.views.query || !this.query) return;
    this.views.query.setQuerySets([]);
    try {
      const resp = await this.query.listQuerySets({ designUri: artifactUri(mount, path) });
      if (mount !== this.doc.mount || path !== this.doc.path) return;
      this.views.query.setQuerySets(resp.querySets.filter((s) => s.set).map((s) => ({ name: s.name, source: s.source, set: s.set! })));
    } catch {
      // A project whose sets do not parse offers none; `agni query --set` names the file.
    }
  }

  // runQuerySet answers a set the query panel opened as tabs, one tab per query in the set's order,
  // over one read of the design (RunQueries). A query the engine could not answer shows its error in
  // its own tab, and the others still answer.
  async runQuerySet(set: QuerySet, tabs: string[]): Promise<void> {
    if (!this.views.query || !this.query) return;
    const push = (r: QueryResult, tab: string) => this.views.query?.setState(r, tab);
    if (!this.doc.mount || !this.doc.path) {
      for (const t of tabs) push(errorResult("Open a design first."), t);
      return;
    }
    for (const t of tabs) push(emptyResult(true), t);
    try {
      const resp = await this.query.runQueries({ uri: artifactUri(this.doc.mount, this.doc.path), set, overlay: this.overlay() });
      resp.results.forEach((r, i) => {
        const tab = tabs[i];
        if (!tab) return;
        if (r.result) {
          this.answers.set(tab, r.result);
          push(resultFromResponse(r.result, (ids) => this.doc.sheetBadges(ids)), tab);
        } else {
          this.answers.delete(tab);
          push(errorResult(r.error || "This query was not answered.", r.errorAt?.caret ?? ""), tab);
        }
      });
    } catch (e) {
      for (const t of tabs) {
        this.answers.delete(t);
        push(errorResult(e instanceof Error ? e.message : String(e)), t);
      }
    }
  }

  // forgetQueryTab drops a closed tab's answer.
  forgetQueryTab(tab: string): void {
    this.answers.delete(tab);
  }

  // releaseFocus puts the drawing back to the findings alone, for a query tab coming forward that
  // has located nothing, so the canvas never shows a cell from an answer no longer in front.
  async releaseFocus(): Promise<void> {
    this.views.query?.setLocateNote("");
    await this.setHighlights(focusStack(this.field(), [], []));
  }

  // locateEntity focuses a query result cell's entity (WS9-038). It navigates to the given sheet (a
  // badge click), then stacks the entity as a translucent focus over any active findings highlight,
  // the same two-layer emphasis selectFinding uses. Native rendering shows the tool's own document
  // with no overlay, so it hops to WebGL to reveal the tint. A query entity is not a finding, so
  // the highlight is built from the bare (kind, subject) via entitySpecs.
  async locateEntity(
    kind: string,
    subject: string,
    sheet?: string,
    reason: LocateReason = LocateReason.UNSPECIFIED,
    pin = "",
  ): Promise<void> {
    if (!subject) return;
    if (sheet && sheet !== this.currentSheet) await this.showSheet(sheet);
    if (this.mode === "native") await this.setMode("webgl");
    // A picked PIN is a different target from its component, since the spec builder keys a pin
    // highlight by (ref, pin) and an empty pin would silently widen the focus to the whole part.
    // Net and component callers pass nothing.
    const focus = withFocusShape(entitySpecs(kind, subject, pin), this.highlightStyle);
    await this.setHighlights(focusStack(this.field(), [{ kind, subject, pin: "" }], focus));
    // Explain an entity the faithful view doesn't draw (WS9-039). The server sets a reason only for
    // an entity absent from the geometry, so a drawn rail (e.g. VBUS) reports none; the note shows
    // only on a faithful layout, since an auto-layout draws every entity and always resolves.
    const faithful = this.currentLayout === "" || this.currentLayout === "faithful";
    const note = faithful && reason !== LocateReason.UNSPECIFIED ? reasonMessage(reason, kind, subject) : "";
    this.views.query?.setLocateNote(note);
  }

  // locateVerdict focuses ONE verdict by its derived id and draws its proof, with the verdict's
  // context as ground and its subject as the figure on top.
  //
  // This is the CLI-to-viewer hop. A row printed by `agni check --verdicts` carries the same id
  // this resolves, computed independently on both sides from the same fields. An unknown id draws
  // nothing and returns false rather than clearing the canvas, because a stale or mistyped link
  // should leave what the reader was looking at alone.
  //
  // Returns whether the id resolved, so the caller can say so. A verdict whose rule has not run yet
  // is absent, the same "not computed" state the findings list models.
  async locateVerdict(id: string): Promise<boolean> {
    const v = this.doc.findVerdict(id);
    if (!v) return false;
    if (this.mode === "native") await this.setMode("webgl");
    // Every entity in the tuple is the figure, so a clearance verdict lights both nets rather than
    // half of a distance.
    const focus = withFocusShape(subjectsToSpecs(v.subjects), this.highlightStyle);
    await this.setHighlights(verdictProofStack(v, focus));
    this.focusedVerdict = v.id;
    // Pushed so the panel learns which verdict is focused, since setHighlights above cleared the
    // field and pushed nothing.
    this.pushFindings();
    this.syncLocation();
    return true;
  }


  // verdictList is the considered set of the selected rules the last run answered, rule by rule in
  // the selection's order, for the panel to render.
  verdictList(): VerdictItem[] {
    return this.doc.verdictList();
  }



  // refreshReport fetches how the current auto-layout maps each component (device class,
  // glyph/box/provided/unresolved) under the active symbol source, and pushes it to the panel.
  // The faithful layout has no such classification, so it clears the panel.
  private async refreshReport(): Promise<void> {
    if (this.currentLayout === "" || this.currentLayout === "faithful") {
      this.views.report(null);
      return;
    }
    const symbols = this.faithfulSymbols ? SymbolSource.FAITHFUL : SymbolSource.GLYPH;
    const resp = await this.client.getLayoutReport({ uri: artifactUri(this.doc.mount, this.doc.path), symbols });
    this.views.report(resp.report ?? null);
  }

  // pushSheets fans the current sheet state out to every navigation surface.
  private pushSheets(activeId: string): void {
    const state = { mount: this.doc.mount, path: this.doc.path, sheets: this.doc.sheets, activeId };
    for (const nav of this.views.sheetNavs) nav.setState(state);
    this.pushOverview(activeId);
  }

  // pushOverview reflects the per-sheet violation tiles (WS9-025), each sheet counted from the
  // current findings' sheet badges. Re-pushed when the sheet set, the shown sheet, or the findings
  // change.
  private pushOverview(activeId = this.currentSheet): void {
    this.views.overview?.setState({ tiles: sheetTiles(this.doc.sheets, this.findings), activeId, ruleCount: this.doc.selectedRules.length });
  }

  // boardLayers is the board layer-visibility setting, kept across sheet switches so
  // returning to the board restores the chosen view.
  private boardLayers = "all";

  // pushControls reflects the current renderer/native/layout state to the control bar.
  private pushControls(): void {
    this.views.controls.setState({
      mode: this.mode,
      nativeAvailable: this.doc.nativeAvailable,
      layouts: this.doc.availableLayouts,
      layout: this.currentLayout,
      providedSymbols: this.doc.availableLayouts.includes("faithful"),
      faithfulSymbols: this.faithfulSymbols,
      board: this.currentSheet === "board",
      boardLayers: this.boardLayers,
      hasHighlights: this.highlights.length > 0,
    });
  }

  // setBoardLayers adopts a layer-visibility choice for the board sheet (WS7-034), a CSS toggle on
  // the shown document with no re-render.
  setBoardLayers(side: string): void {
    this.boardLayers = side;
    this.render.setBoardLayers(side);
    this.pushControls();
  }

  // openFile loads a design, populates the sheet navigator, reports whether NATIVE is available for
  // it, and renders a sheet in the active mode. wantSheet names the sheet to open (a deep-link
  // restore), falling back to the first sheet when empty or absent from this design.
  //
  // A DIFFERENT design starts from the server's default layout (faithful when the file carries
  // geometry) instead of inheriting the previous design's, since a sticky auto-layout would draw a
  // geometry-bearing design as a graph. keepLayout opts out for deep-link restores, where the URL's
  // layout must win; a same-file re-open (setLayout) keeps the chosen layout because mount/path are
  // unchanged.
  async openFile(mount: string, path: string, wantSheet = "", keepLayout = false): Promise<void> {
    const newFile = mount !== this.doc.mount || path !== this.doc.path;
    if (!keepLayout && newFile) this.currentLayout = "";
    // A different design retires the link's claim, since the hash described the bytes behind the
    // OLD path. restore() re-sets it after this call, which is why it clears here rather than
    // reading a location.
    if (newFile) {
      this.linkHash = "";
      this.views.staleLinkNote(null);
      this.doc.catalogLoading = true;
      this.pushFindings();
    }
    const changed = mount !== this.doc.mount || path !== this.doc.path;
    if (changed) this.answers.clear();
    this.doc.mount = mount;
    this.doc.path = path;
    if (changed) void this.loadQuerySets(mount, path);
    this.setBusy(true, "loading design…");
    try {
      const d = await this.client.getDesign({ uri: artifactUri(mount, path), layout: this.currentLayout });
      this.doc.sheets = d.sheets;
      // Adopt the effective layout the server chose (the request may have been empty or
      // unavailable for this file), and report the options to the UI.
      this.currentLayout = d.layout;
      this.doc.availableLayouts = d.availableLayouts ?? [];
      // Default the symbol source to faithful when a newly opened design provides its own
      // symbols (availableLayouts has "faithful"), so it draws with real artwork rather than
      // glyphs even after switching to an auto-layout. A restore keeps the URL's choice.
      if (!keepLayout && newFile) this.faithfulSymbols = this.doc.availableLayouts.includes("faithful");
      this.doc.summary = summaryLine(path, d.name, d.layout, d.sourceFormat, d.componentCount, d.netCount);
      this.views.summary(this.doc.summary);
      // Pushed with the summary because it is a property of THIS read, and a reader needs it before
      // trusting the drawing.
      this.views.undrawnNote(undrawnNote(d.undrawn));
      // "" means the server could not hash the file, and it MUST NOT be read as agreement with a
      // link that named a revision; staleLinkNote keeps it a third state.
      this.doc.designContentHash = d.contentHash ?? "";
      // If the newly opened file can't render natively but we're in native mode, fall back to
      // SVG (the always-works renderer) so switching files doesn't error.
      if (!d.nativeAvailable && this.mode === "native") this.mode = "svg";
      this.doc.nativeAvailable = d.nativeAvailable;
      this.pushControls();
      // Resolve the project before the report and parts work, since a reader wants to know whose
      // config is in effect while the rest loads, and the pickers below filter by it.
      await this.resolveProject();
      const target = (wantSheet && d.sheets.find((s) => s.id === wantSheet)) || d.sheets[0];
      this.pushSheets(target?.id ?? "");
      if (target) await this.showSheet(target.id);
      else this.syncLocation(); // no sheets to render, but the file selection still owns the URL
      this.doc.run = null; // a run is of one design, with its own tiers and so its own skipped rules
      this.setBusyPhase("loading rules…");
      await this.loadRules(mount, path); // catalog + default selection; checks run on demand (Run button)
      this.assembleFindings(); // empty until a run; pushes the "press Run" state + zero overview counts
      await this.setHighlights([]); // clear any highlight carried over from the previously open design
      this.clearExpectations(); // the sidecar reconcile is part of the on-demand run, not the load
      this.setBusyPhase("building report…");
      await this.refreshReport(); // the auto-layout conversion report is not a check, so it stays eager
      await this.refreshParts(); // the datasheet-params join is a per-design read, not a check
      // One resolution feeds both pickers, each with the kind it can actually parse.
      const choices =
        this.views.conventionBar || this.views.review
          ? await this.pickerChoices()
          : { conventions: [], checklists: [] };
      this.conventionChoices = choices.conventions;
      this.checklistChoices = choices.checklists;
      this.pushConvention();
      await this.refreshReviews(); // stored review runs for this design, so a reviewed board opens on its latest verdict
    } catch (e) {
      this.views.summary(`error: ${String(e)}`);
    } finally {
      this.setBusy(false);
      // An open that failed before loadRules would otherwise leave Run disabled for good.
      if (this.doc.catalogLoading) {
        this.doc.catalogLoading = false;
        this.pushFindings();
      }
    }
  }

  // restore reopens the viewer at a URL-derived location. It adopts the mode/layout/symbol knobs
  // from the location (so the design loads and renders in that view) and opens the file at the
  // requested sheet. Used on initial load and on browser back/forward, so a refresh or a shared
  // link lands where you left off rather than on the empty shell. It sets the knobs directly (not
  // via setMode/setLayout) to avoid their re-render side effects, so the single openFile below does
  // one load and its native-unavailable guard still corrects an impossible mode.
  async restore(loc: ViewerLocation): Promise<void> {
    if (loc.mode) this.mode = loc.mode;
    this.currentLayout = loc.layout; // empty -> server picks the effective layout, as on a fresh open
    this.faithfulSymbols = loc.symbols;
    await this.openFile(loc.mount, loc.path, loc.sheet, true);
    // A verdict id in the link RUNS THE CHECKS before resolving it, a deliberate exception to
    // checks being on-demand (WS9). A cold load has an empty verdict cache, so without this a
    // pasted link lands on "Press Run checks" and resolves nothing
    // (docsite/content/architecture/web-client.md#a-panel-that-works-on-click-can-still-be-broken-on-arrival).
    // The hash is adopted and reported BEFORE the checks run and the proof is drawn, so the warning
    // is on screen before the highlight it warns about.
    this.linkHash = loc.hash;
    this.views.staleLinkNote(staleLinkNote(this.linkHash, this.doc.designContentHash));
    if (loc.verdict) {
      await this.runChecksForLink(loc.rule);
      await this.locateVerdict(loc.verdict);
    }
    // A trace link needs no run first, because it carries the QUESTION rather than a derived id, so
    // the viewer just asks it.
    if (loc.trace) {
      const [from, to] = splitTraceParam(loc.trace);
      await this.runTrace(from, to, loc.traceHops);
    }
    // A query link carries the question too (agni issue 634): the panel opens a tab holding it and
    // asks it whole, as `agni query` did.
    if (loc.query) this.views.query?.setQuery(loc.query, bindsFromLink(loc.binds), { title: loc.title, whole: true });
    if (loc.set && loc.q) await this.openSetMember(loc.set, loc.q);
  }

  // openSetMember opens one query of the design's project sets, found by the set's name and the
  // query's, as the Sets menu would open it. A name the project no longer keeps says so in the panel
  // rather than opening nothing.
  private async openSetMember(setName: string, name: string): Promise<void> {
    if (!this.views.query || !this.query || !this.doc.mount || !this.doc.path) return;
    try {
      const resp = await this.query.listQuerySets({ designUri: artifactUri(this.doc.mount, this.doc.path) });
      const set = resp.querySets.find((s) => s.name === setName)?.set;
      const member = set?.queries.find((q) => q.name === name);
      if (set && member) {
        this.views.query.openSet({ ...set, queries: [member] });
        return;
      }
    } catch {
      // Falls through to the note: a link that cannot be followed says so.
    }
    this.views.query.setLocateNote(`This link asks "${name}" from the query set "${setName}", which this design's project does not keep.`);
  }

  // currentLoc snapshots the URL-addressable state.
  private currentLoc(): ViewerLocation {
    return {
      mount: this.doc.mount,
      path: this.doc.path,
      isDir: false, // the presenter only ever addresses an open file
      sheet: this.currentSheet,
      mode: this.mode,
      layout: this.currentLayout,
      symbols: this.faithfulSymbols,
      verdict: this.focusedVerdict,
      hash: this.linkHash,
      // Named from the focused verdict rather than remembered from the URL, so a verdict reached by
      // clicking in the panel produces the same cheap link as one arrived at from a report.
      rule: this.focusedVerdict ? (this.doc.findVerdict(this.focusedVerdict)?.rule ?? "") : "",
      // The trace the panel is showing, so a typed route is addressable and the address bar never
      // disagrees with the panel.
      trace: this.tracedPins,
      traceHops: this.tracedHops,
      // A query link is read on arrival and not written back, since the panel's tabs are the reader's
      // own from then on.
      query: "",
      title: "",
      binds: [],
      set: "",
      q: "",
    };
  }

  // syncLocation reports the current URL-addressable state to the host (which reflects it into the
  // browser URL). Cheap and idempotent, since the host dedupes against the address bar.
  private syncLocation(): void {
    this.views.location?.(this.currentLoc());
  }

  // loadRules fetches the rule catalog for the open design (ListRules), defaults the active ruleset
  // to every available rule, and pushes both to the rules panel. A failure leaves an empty catalog
  // (the panel shows "No rules.") rather than erroring the open.
  private async loadRules(mount: string, path: string): Promise<void> {
    if (!this.doc.catalogLoading) {
      this.doc.catalogLoading = true;
      this.pushFindings();
    }
    await this.doc.loadCatalog(this.checks, artifactUri(mount, path), this.overlay());
    this.pushRules();
  }

  // pushRules reflects the current catalog + active selection + per-rule fired counts to the rules
  // panel (the fired count comes from the finding cache, so a rule's badge shows how many findings
  // it produced this run).
  private pushRules(): void {
    // fired counts only the currently selected rules, matching the checks panel, so a rule that ran
    // and was then deselected keeps its cached findings but drops out of the fired counts.
    const fired: Record<string, number> = {};
    const fs = this.doc.run?.resp.findings ?? [];
    for (const name of this.doc.selectedRules) fired[name] = fs.filter((f) => f.rule === name).length;
    this.views.rules.setState({ rules: this.doc.rules, selected: this.doc.selectedRules, fired });
  }

  // setRuleSelection adopts a new active ruleset (from the rules panel's checkboxes) and re-renders
  // the findings over just that subset of the last run. It fetches nothing (checks are on-demand), so
  // toggling a rule is instant. A selected rule the last run did not answer shows as pending until
  // the next Run; a deselected rule's findings are hidden.
  async setRuleSelection(names: string[]): Promise<void> {
    this.doc.selectedRules = names;
    this.assembleFindings();
    await this.setHighlights(subjectsToSpecs(this.field()));
  }

  // runChecksForLink resolves a verdict a URL named, running ONE rule when the link says which (see
  // ViewerLocation.rule in router.ts). A cold load has an empty verdict cache, so a link must run
  // something first. Running the whole catalog was 56 rules and about fifteen seconds per click on
  // the board this was measured against, since each click is a fresh page load.
  //
  // An unnamed rule (an older or hand-typed link) or one the catalog does not have falls back to
  // the full run, because the alternative is resolving nothing and reporting it as a stale link.
  // The rest of the catalog stays PENDING afterwards; the reader who wants it presses Run.
  private async runChecksForLink(rule: string): Promise<void> {
    if (!rule || !this.doc.rulesByName.has(rule)) {
      await this.runChecks();
      return;
    }
    const restore = this.doc.selectedRules;
    this.doc.selectedRules = [rule];
    try {
      await this.runChecks();
    } finally {
      this.doc.selectedRules = restore;
    }
    this.assembleFindings();
  }

  // runChecks is the on-demand check trigger (the Run button). It fetches the selected rules not
  // yet cached, reconciles the expectation sidecar (a full run), and refreshes interface coverage.
  // It is the only place a design is evaluated, so a large design does not stall the open.
  // Re-entrancy, a catalog still loading and no-open-file are guarded, and running disables the
  // button meanwhile.
  async runChecks(): Promise<void> {
    if (this.checksRunning || this.doc.catalogLoading || !this.doc.mount || !this.doc.path) return;
    this.checksRunning = true;
    this.pushFindings(); // reflect running=true (disables the Run button)
    this.setBusy(true, "running checks…");
    try {
      await this.runSelection();
      await this.refreshExpectations(); // the sidecar reconcile's full run is deferred to here too
      await this.refreshCoverage();
    } finally {
      this.checksRunning = false;
      this.setBusy(false);
      this.pushFindings(); // running=false + refreshed pending count
    }
  }

  // runSelection asks CheckDesign for the selected rules the run lacks and folds the answer into it,
  // then assembles the visible list and lights up every current finding when the reader asked for
  // that. A geometry-only file (no netlist) makes CheckDesign fail, which leaves the run as it was and
  // those rules pending.
  private async runSelection(): Promise<void> {
    await this.doc.runMissing(this.checks, this.overlay());
    this.assembleFindings();
    await this.setHighlights(subjectsToSpecs(this.field())); // every finding at once, when the reader asked for it
  }

  // assembleFindings rebuilds the visible list from the last run for the active ruleset (a
  // stable rule-then-subject base order; the panel re-sorts), clears the focus, and pushes the checks,
  // rules (fired counts), and overview (per-sheet counts) panels. It fetches nothing.
  private assembleFindings(): void {
    const selected = new Set(this.doc.selectedRules);
    this.findings = (this.doc.run?.resp.findings ?? [])
      .filter((f) => selected.has(f.rule))
      .map((f) => this.doc.findingItem(f))
      .sort((a, b) => (a.rule !== b.rule ? (a.rule < b.rule ? -1 : 1) : a.subject < b.subject ? -1 : a.subject > b.subject ? 1 : 0));
    this.selectedSubject = "";
    this.pushFindings();
    this.pushRules(); // fired counts changed
    this.pushOverview(); // per-sheet counts follow the same findings
  }

  // pushFindings reflects the current findings + focus + selection to the checks panel, including
  // the pending count (selected rules not yet run, which badges the Run button and tells "press
  // Run" from "ran clean") and the rule catalog summaries (the per-rule one-liners the merged panel
  // shows).
  private pushFindings(): void {
    const pending = this.doc.selectedRules.filter((n) => !this.doc.ran(n)).length;
    const skippedAt = new Map((this.doc.run?.resp.skipped ?? []).map((sk) => [sk.name, sk.reason]));
    const ruleSummaries: Record<string, string> = {};
    for (const r of this.doc.rules) if (r.summary) ruleSummaries[r.name] = r.summary;
    const state: FindingsState = {
      findings: this.findings,
      verdicts: this.verdictList(),
      focusedVerdict: this.focusedVerdict,
      selected: this.selectedSubject,
      ruleCount: this.doc.selectedRules.length,
      pending,
      running: this.checksRunning,
      catalogLoading: this.doc.catalogLoading,
      // Only the SELECTED rules, since a rule nobody ticked being unavailable is not news. Sorted so
      // the list is stable across runs rather than following response order.
      skipped: this.doc.selectedRules
        .filter((n) => skippedAt.has(n))
        .map((n) => ({ rule: n, reason: skippedAt.get(n) ?? "" }))
        .sort((a, b) => a.rule.localeCompare(b.rule)),
      ruleSummaries,
      saveNote: this.saveNote,
    };
    this.views.findings.setState(state);
    // The query panel projects the SAME state onto whatever is selected (agni issue 259), so an
    // entity view is a projection of one pass rather than a second evaluation. It gets the whole
    // state rather than a count because only pending/running separate "nothing is wrong with this
    // net" from "nobody has checked it", and this code does not know the selection.
    this.views.query?.setFindings(state);
  }


  private saveNote = "";

  // saveReport writes the selection's run as a file to keep, the check report page or the results
  // document, the same bytes `agni check` writes. The engine renders it from the run the page holds,
  // so saving runs no rules. It needs every selected rule run, which the panel enforces by enabling
  // the control only then.
  async saveReport(format: "html" | "json"): Promise<void> {
    if (!this.doc.mount || !this.doc.path || !this.views.download) return;
    const held = this.doc.heldRun();
    if (!held) return;
    const { names, run } = held;
    const links = this.reportLinks();
    try {
      const resp = await this.checks.renderCheckReport({
        uri: artifactUri(this.doc.mount, this.doc.path),
        overlay: this.overlay(),
        rules: names,
        run,
        format: format === "html" ? CheckReportFormat.HTML : CheckReportFormat.RESULTS_JSON,
        links,
      });
      this.views.download({ filename: resp.filename, contentType: resp.contentType, content: resp.content });
      this.saveNote = "";
    } catch (err) {
      this.saveNote = `Could not save the report: ${err instanceof Error ? err.message : String(err)}`;
    }
    this.pushFindings();
  }

  // reportLinks is where a saved report's rows link back to: this viewer, under the page's own base.
  // A dropped design lives in this tab's worker alone, so a link to it would open nothing later.
  private reportLinks(): { withheld?: string; urlBase?: string; designUri?: string } {
    return this.doc.mount === BROWSER_MOUNT
      ? { withheld: "This design was opened from the reader's own files, which the browser keeps only in the tab that opened them, so a link to it would open nothing." }
      : { urlBase: this.views.linkBase ?? "", designUri: artifactUri(this.doc.mount, this.doc.path) };
  }

  // reviewWire holds each run the review panel shows as the server returned it, by name, so Save
  // report sends the run itself and nothing is scored again (agni issue 1030).
  private reviewWire = new Map<string, Review>();

  // saveReviewReport writes the selected checklist run as the page `agni review --format html` writes,
  // through the same rpc, and downloads it. A failure is said in the panel.
  async saveReviewReport(): Promise<void> {
    if (!this.reviews || !this.views.download) return;
    const review = this.reviewWire.get(this.reviewState.selected);
    if (!review) return;
    try {
      const resp = await this.reviews.renderReviewReport({ review, links: this.reportLinks() });
      this.views.download({ filename: resp.filename, contentType: resp.contentType, content: resp.content });
      this.reviewState.saveNote = "";
    } catch (err) {
      this.reviewState.saveNote = `Could not save the report: ${err instanceof Error ? err.message : String(err)}`;
    }
    this.pushReview();
  }


  // checksAnswer is the held run as an answer for Tabulate, undefined until the selection has run.
  checksAnswer(): TableAnswer | undefined {
    const held = this.doc.heldRun();
    return held ? { case: "check", value: held.run } : undefined;
  }

  // queryAnswer is one query tab's answer for Tabulate, undefined before it has answered.
  queryAnswer(tab = ""): TableAnswer | undefined {
    const answer = this.answers.get(tab);
    return answer ? { case: "query", value: answer } : undefined;
  }

  // designStem names a saved file after the open design, its file name without the extension.
  designStem(): string {
    const base = (this.doc.path ?? "").split("/").filter(Boolean).pop() ?? "";
    return base.replace(/\.[^.]+$/, "") || "agni";
  }

  // saveChecksCSV saves the held run's findings and verdicts as csv through save, the Tables
  // presenter's saveAnswer (agni issue 956), and shows a failure where Save report shows its own.
  async saveChecksCSV(save: (answer: TableAnswer, stem: string, names: string[]) => Promise<string>): Promise<void> {
    const answer = this.checksAnswer();
    if (!answer) return;
    this.saveNote = await save(answer, this.designStem(), ["findings", "verdicts"]);
    this.pushFindings();
  }

  // saveQueryCSV saves one query tab's answer as csv the same way.
  async saveQueryCSV(save: (answer: TableAnswer, stem: string, names: string[]) => Promise<string>, tab = ""): Promise<void> {
    const answer = this.queryAnswer(tab);
    if (!answer) return;
    const note = await save(answer, this.designStem(), []);
    if (note) this.views.query?.setLocateNote(note);
  }


  // ---- Naming convention (WS9-128) -----------------------------------------------------------
  //
  // A request convention REPLACES the server's startup default (WS3-124; see conventions.ts). The
  // presenter holds the chosen one and stamps it onto every rule-running call, so the checks panel,
  // the report, and a review run answer under one vocabulary.


  // overlayConfig is overlay for a rule-running call the composition root makes on the page's behalf,
  // such as the Tables panel running the checks over a comparison's side, so it checks under the
  // convention the page shows.
  overlayConfig(): OverlayConfig | undefined {
    return this.overlay();
  }

  // overlay is what every rule-running call carries. Every call site that runs rules goes through
  // it, so one that did not would stand out from its neighbours.
  private overlay(): OverlayConfig | undefined {
    if (!this.doc.convention && !this.projectState.plain) return undefined;
    return create(OverlayConfigSchema, {
      // The convention rides inside the shared AnalysisConfig, the same message a Project declares, so
      // a config tier added there is one a request can carry without a second edit here.
      config: { conventions: this.doc.convention ?? undefined },
      // ignore_project is the "show me the built-in catalog" choice. It rides on every rule-running
      // request because each surface composes its own overlay, and a toggle that reached only some
      // would show findings from one catalog beside the rule set of another.
      ignoreProject: this.projectState.plain,
    });
  }

  // resolveProject asks which project the open design belongs to and pushes the answer to the bar.
  // The viewer resolves and SHOWS rather than letting the server's loader silently swap in the
  // design's entry, because the user picked one file and would be looking at a different one (see
  // project.ts).
  private async resolveProject(): Promise<void> {
    if (!this.views.projectBar && !this.views.review && !this.views.conventionBar) return;
    const plain = this.projectState.plain;
    this.projectState = { ...emptyProject(), plain, busy: true };
    this.pushProject();
    if (!this.projects || !this.doc.mount || !this.doc.path) {
      this.projectState = { ...emptyProject(), plain };
      this.pushProject();
      return;
    }
    const uri = artifactUri(this.doc.mount, this.doc.path);
    try {
      const resp = await this.projects.resolveDesign({ uri });
      const d = resp.design;
      this.projectState = {
        ...emptyProject(),
        plain,
        project: resp.project?.name ?? "",
        title: resp.project?.title ?? "",
        design: d?.name ?? "",
        entry: d?.entryUri ?? "",
        // A design that resolved to nothing has no entry to differ from, so the file the user opened
        // is trivially the one being read.
        namedIsEntry: !d || d.entryUri === "" || d.entryUri === uri,
      };
      this.projectResolved = resp;
    } catch (e) {
      this.projectState = { ...emptyProject(), plain, error: messageOf(e) };
      this.projectResolved = undefined;
    }
    this.pushProject();
  }

  private pushProject(): void {
    this.views.projectBar?.setState(this.projectState);
  }

  // setPlainCatalog switches between this design's project config and the built-in catalog, then
  // re-runs whatever is on screen. The toggle answers "whose rules are these" by subtraction, which
  // needs the second run.
  async setPlainCatalog(plain: boolean): Promise<void> {
    if (this.projectState.plain === plain) return;
    this.projectState = { ...this.projectState, plain };
    this.pushProject();
    // The rule LIST is recomposed too, or the panel would list one catalog's rules beside the
    // other's findings.
    await this.loadRules(this.doc.mount, this.doc.path);
    await this.runChecks();
  }

  // setConvention resolves a stored convention config and applies it to subsequent runs, or clears
  // back to the server's default when ref is empty.
  //
  // It resolves through the SERVER (GetNamingConvention) rather than parsing YAML here, because the
  // browser holds a ref and no filesystem, and a pattern that will not compile is then rejected
  // once instead of on every run.
  //
  // Cached findings are dropped because a convention changes which rules exist and what the engine
  // believes a rail IS, so every cached finding answered a different question.
  async setConvention(ref: string): Promise<void> {
    if (!this.views.conventionBar) return;
    if (ref === "") {
      this.doc.convention = null;
      this.doc.conventionRef = "";
      await this.reloadForConvention();
      this.pushConvention();
      return;
    }
    this.pushConvention(true);
    try {
      const resp = await this.checks.getNamingConvention({ uri: artifactUri(this.doc.mount, ref) });
      this.doc.convention = resp.convention ?? null;
      this.doc.conventionRef = ref;
      await this.reloadForConvention();
      this.pushConvention();
    } catch (e) {
      this.pushConvention(false, messageOf(e));
    }
  }

  // reloadForConvention re-reads the rule CATALOG and drops the held run after the vocabulary
  // changes. The catalog reload, dropping the run, and the
  // query-result reset must happen together, since a request convention changes which rules EXIST
  // (docsite/content/architecture/web-client.md, "Two caches for one answer will drift"). A client
  // that kept the old catalog would run none of the selected naming rules, which reads as a design
  // with no naming problems.
  private async reloadForConvention(): Promise<void> {
    // A verdict is keyed by rule name, which is what a convention changes, so a surviving run could
    // claim coverage for a rule nobody ran.
    this.doc.run = null;
    this.focusedVerdict = "";
    // Query results were computed under the previous vocabulary too (`net.rail` answers differently),
    // so leaving them would show two vocabularies at once.
    this.answers.clear();
    this.views.query?.clearAnswers();
    if (this.doc.mount && this.doc.path) await this.loadRules(this.doc.mount, this.doc.path);
    this.assembleFindings();
  }

  // pushConvention reports which vocabulary is in effect (WS9-128). Nothing in a findings list
  // distinguishes a finding that changed with the vocabulary from one that changed with the design,
  // so the bar has to say.
  private pushConvention(busy = false, error = ""): void {
    this.views.conventionBar?.setState({
      choices: this.conventionChoices,
      active: this.doc.conventionRef,
      name: this.doc.convention?.name ?? "",
      busy,
      error,
    });
  }

  // conventionChoices and checklistChoices are what each picker offers, filled on load. They are
  // separate fields rather than one shared list because the two pickers resolve their ref through
  // different rpcs against different schemas, so a file that is a valid answer to one is a parse
  // error in the other.
  private conventionChoices: ChecklistOption[] = [];
  private checklistChoices: ChecklistOption[] = [];
  // projectState is what the project bar shows, and projectResolved is the resolution it came from,
  // kept so the pickers can offer what the PROJECT declares rather than every YAML beside the design.
  private projectState: ProjectState = emptyProject();
  private projectResolved?: ResolveDesignResponse;

  // ---- Review (WS9-052) ----------------------------------------------------------------------
  //
  // A review RUN is a resource (WS9-053; see review.ts), so the panel shows the runs already stored
  // for this design and creating one is an explicit act.

  // reviewState is the panel's pushed state, held here because the presenter owns the interaction
  // loop (which run is selected, which checklist is chosen) and the panel is a humble view (C3).
  private reviewState: ReviewState = emptyReview();

  // pushReview sends the current state to the panel, a no-op when the panel is unwired.
  private pushReview(): void {
    this.views.review?.setState({ ...this.reviewState });
  }

  // refreshReviews loads the runs stored for the open design plus the checklists sitting beside it.
  // It runs on each design load, so opening a reviewed board shows its latest verdict rather than
  // an empty panel.
  //
  // A server started without --review-store answers FailedPrecondition, recorded as
  // storeConfigured=false rather than as an error, because "this deployment keeps no runs" and
  // "nobody has reviewed this board" look identical in an empty list.
  private async refreshReviews(): Promise<void> {
    if (!this.views.review || !this.reviews) return;
    this.reviewState = emptyReview();
    this.reviewWire.clear();
    if (!this.doc.mount || !this.doc.path) {
      this.pushReview();
      return;
    }
    this.reviewState.checklists = this.checklistChoices;
    this.reviewState.checklist = this.reviewState.checklists[0]?.ref ?? "";
    try {
      const resp = await this.reviews.listReviews({ filter: `design="${artifactUri(this.doc.mount, this.doc.path)}"` });
      this.reviewState.runs = resp.reviews.map(reviewFromWire);
      for (const r of resp.reviews) this.reviewWire.set(r.name, r);
      // Newest first comes from the server, so the head is the latest run.
      this.reviewState.selected = this.reviewState.runs[0]?.name ?? "";
    } catch (e) {
      if (isStoreUnconfigured(e)) this.reviewState.storeConfigured = false;
      else this.reviewState.error = messageOf(e);
    }
    this.pushReview();
  }

  // pickerChoices are what the convention and checklist pickers offer, ONE LIST PER KIND. When the
  // design resolves to a project that declares either, they are what the PROJECT declares: its named
  // checklists, carried as values, and no convention files, since the project's conventions are a
  // section of project.yaml that every run already composes (agni issue 828).
  //
  // The kinds stay APART because a checklist and a naming convention are parsed by different rpcs,
  // and offering a checklist in the vocabulary picker fails with a YAML error naming a field the
  // other schema has never heard of. Profiles appear in NEITHER, since a profile is composed into
  // the catalog rather than selected.
  //
  // A design with no project falls back to listing the siblings for both. Nothing declared
  // anything, and offering a file that turns out not to be a checklist costs one clear error where
  // hiding a real one costs a user their own file.
  private async pickerChoices(): Promise<{ conventions: ChecklistOption[]; checklists: ChecklistOption[] }> {
    const cfg = this.projectResolved?.project?.config;
    let declared = cfg?.checklists ?? [];
    // The descriptor's own config misses what the project inherits through extends, so the checklists
    // come from ListChecklists, which answers from the composed config the CLI's review reads (agni
    // issues 829, 859). A failure keeps the descriptor's own list rather than offering nothing.
    if (this.projectResolved?.project && this.reviews) {
      try {
        declared = (await this.reviews.listChecklists({ designUri: artifactUri(this.doc.mount, this.doc.path) })).checklists;
      } catch {
        // the descriptor's own checklists stand
      }
    }
    if (cfg && (cfg.conventions || declared.length > 0)) {
      return { conventions: [], checklists: projectChecklistOptions(declared) };
    }
    const siblings = await this.yamlSiblings();
    return { conventions: siblings, checklists: siblings };
  }

  // yamlSiblings lists the design's own directory and keeps the YAML files. It does not read them,
  // because the server decides whether a YAML file is a checklist or a naming convention
  // (GetReviewManifest / GetNamingConvention), and guessing here would either hide a user's real
  // file or duplicate two parsers in the browser. One listing feeds BOTH pickers, since a project
  // keeps its review.yaml and its conventions.yaml side by side.
  private async yamlSiblings(): Promise<ChecklistOption[]> {
    if (!this.workspace) return [];
    const slash = this.doc.path.lastIndexOf("/");
    const dir = slash < 0 ? "" : this.doc.path.slice(0, slash);
    try {
      const resp = await this.workspace.listDir({ uri: artifactUri(this.doc.mount, dir) });
      return checklistOptions(resp.entries.map((e) => ({ name: e.name, path: uriPath(e.uri), isDir: e.isDir })));
    } catch {
      return [];
    }
  }

  // designParent is the project resource name the open design belongs to, "" when it belongs to
  // none or when resolution fails. A failure is not surfaced, because the run still stores
  // unparented, which beats refusing to record a review that ran.
  private async designParent(): Promise<string> {
    if (!this.projects || !this.doc.mount || !this.doc.path) return "";
    try {
      const resp = await this.projects.resolveDesign({ uri: artifactUri(this.doc.mount, this.doc.path) });
      return resp.project?.name ?? "";
    } catch {
      return "";
    }
  }

  // createReview runs the chosen checklist against the open design and stores the result, then
  // shows it. The manifest travels as a VALUE (C22): a project's checklist already is one, and a file
  // beside the design is resolved server-side first (GetReviewManifest), because the browser holds a
  // ref and no filesystem.
  async createReview(): Promise<void> {
    if (!this.views.review || !this.reviews) return;
    if (this.reviewState.running) return;
    if (!this.doc.mount || !this.doc.path || !this.reviewState.checklist) return;
    this.reviewState = { ...this.reviewState, running: true, error: "" };
    this.pushReview();
    this.setBusy(true, "running review…");
    try {
      // A project's checklist is already a value. Only a file found beside the design is resolved.
      const chosen = this.checklistChoices.find((c) => c.ref === this.reviewState.checklist);
      const manifest =
        chosen?.manifest ??
        (await this.reviews.getReviewManifest({ uri: artifactUri(this.doc.mount, this.reviewState.checklist) })).manifest;
      const created = await this.reviews.createReview({
        // Resolved here rather than re-derived by the server, so the run is filed under the project
        // whose config scored it. An empty parent is a real answer, since a design on a mounted
        // folder often belongs to no project.
        parent: await this.designParent(),
        designUri: artifactUri(this.doc.mount, this.doc.path),
        manifest,
        overlay: this.overlay(),
      });
      const run = reviewFromWire(created);
      this.reviewWire.set(created.name, created);
      // Prepend rather than refetch, since the new run is the newest and the server already handed
      // back the whole document.
      this.reviewState.runs = [run, ...this.reviewState.runs];
      this.reviewState.selected = run.name;
    } catch (e) {
      if (isStoreUnconfigured(e)) this.reviewState.storeConfigured = false;
      else this.reviewState.error = messageOf(e);
    } finally {
      this.reviewState.running = false;
      this.setBusy(false);
      this.pushReview();
    }
  }

  // showReview selects a stored run. The document is already held, so this is local.
  showReview(name: string): void {
    if (!this.views.review) return;
    this.reviewState.selected = name;
    this.pushReview();
  }

  // setChecklist chooses which manifest the next run scores.
  setChecklist(ref: string): void {
    if (!this.views.review) return;
    this.reviewState.checklist = ref;
    this.pushReview();
  }

  // runTrace walks from one named pin to another over the open design and lights the answer.
  //
  // The highlight goes through the SAME two-layer stack a query cell and a verdict use, with the
  // findings layer as context underneath and the route as the figure on top. traceSubjects decides
  // what to light for each outcome (see trace.ts), including the two nets that fail to join on a
  // no-route.
  //
  // A malformed pin is caught HERE rather than at the server, since it is a typo in the box the
  // panel is showing.
  async runTrace(fromText: string, toText: string, hops = 0): Promise<void> {
    const view = this.views.trace;
    if (!view) return;
    const from = parseEndpoint(fromText);
    const to = parseEndpoint(toText);
    if (!from || !to) {
      view.setState(errorTrace("Name each end as <ref-des>.<pin>, for example U1.3"));
      return;
    }
    if (!this.doc.mount || !this.doc.path) {
      view.setState(errorTrace("Open a design first"));
      return;
    }
    view.setState(loadingTrace());
    let state;
    try {
      const resp = await this.client.traceDesign({ uri: artifactUri(this.doc.mount, this.doc.path), from, to, hops });
      if (!resp.trace) {
        view.setState(errorTrace("The server answered with no trace"));
        return;
      }
      state = traceFromResponse(resp.trace);
    } catch (e) {
      view.setState(errorTrace(e instanceof Error ? e.message : String(e)));
      return;
    }
    view.setState(state);
    // Recorded whatever the outcome, so a link can carry a NEGATIVE answer too.
    this.tracedPins = `${from.refDes}.${from.pin},${to.refDes}.${to.pin}`;
    this.tracedHops = hops;
    this.syncLocation();
    const subjects = traceSubjects(state);
    if (subjects.length === 0) return;
    if (this.mode === "native") await this.setMode("webgl");
    // Go to a sheet the answer is ON, the way selectFinding does, or a cold link draws on the
    // design's first sheet, which on an 82-sheet export is a table of contents with no wires (agni
    // issue 657). traceSheet prefers the FROM endpoint's sheet (see trace.ts), and an answer drawn
    // nowhere leaves the view alone.
    const target = traceSheet(state);
    if (target && target !== this.currentSheet) await this.showSheet(target);
    const focus = withFocusShape(subjectsToSpecs(subjects), this.highlightStyle);
    await this.setHighlights(focusStack(this.field(), subjects, focus));
  }

  // refreshCoverage fetches the interface-coverage matrix for the open design (WS9-041) and pushes
  // it to the coverage panel, a no-op when the panel is unwired. A design with no detected
  // interface (or a fetch failure) pushes an empty state, so the panel shows its empty message
  // rather than stale coverage from the previous design.
  private async refreshCoverage(): Promise<void> {
    if (!this.views.coverage) return;
    if (!this.doc.mount || !this.doc.path) {
      this.views.coverage.setState(emptyCoverage());
      return;
    }
    try {
      const resp = await this.checks.getInterfaceCoverage({ uri: artifactUri(this.doc.mount, this.doc.path), overlay: this.overlay() });
      this.views.coverage.setState(coverageFromResponse(resp));
    } catch {
      this.views.coverage.setState(emptyCoverage());
    }
  }

  // refreshParts fetches the datasheet-params join for the open design (WS9-035) and pushes it to
  // the parts panel, a no-op when the panel is unwired. A design with no joined specs (or a serve
  // started without --params, or a fetch failure) pushes an empty state, so the panel shows its
  // empty hint rather than stale parts from the previous design.
  private async refreshParts(): Promise<void> {
    if (!this.views.parts) return;
    if (!this.doc.mount || !this.doc.path) {
      this.views.parts.setState(emptyParts());
      return;
    }
    try {
      const resp = await this.checks.getComponentParams({ uri: artifactUri(this.doc.mount, this.doc.path) });
      this.views.parts.setState(partsFromResponse(resp));
    } catch {
      this.views.parts.setState(emptyParts());
    }
  }

  // clearExpectations resets the expectations panel to empty and pushes it. Called on design load
  // because the sidecar reconcile runs with the on-demand Run (refreshExpectations), so before the
  // first Run the panel shows its neutral empty state instead of reconciling against zero findings,
  // which would mislabel every non-pending expectation "missing".
  private clearExpectations(): void {
    this.doc.expectations = [];
    this.doc.expectationFindings = [];
    this.doc.hasSidecar = false;
    this.pushExpectations();
  }

  // refreshExpectations loads the design's expectation sidecar and reconciles it for the panel. It
  // reconciles against a FULL check run (every rule), not the checks-panel's selection, so a
  // "missing" row means the rule ran and did not fire, not that it was deselected. Runs as part of the
  // on-demand check run (runChecks); a design with no sidecar leaves the panel empty.
  private async refreshExpectations(): Promise<void> {
    this.doc.expectations = [];
    this.doc.expectationFindings = [];
    this.doc.hasSidecar = false;
    if (this.doc.mount && this.doc.path) {
      try {
        const exp = await this.checks.getExpectations({ uri: artifactUri(this.doc.mount, this.doc.path) });
        this.doc.hasSidecar = exp.hasSidecar;
        this.doc.expectations = exp.expectations.map((e) => ({ rule: e.rule, subjects: e.subjects, pending: e.pending, why: e.why }));
        if (this.doc.hasSidecar) {
          const resp = await this.checks.checkDesign({ uri: artifactUri(this.doc.mount, this.doc.path), rules: [], overlay: this.overlay() });
          this.doc.expectationFindings = resp.findings.map((f) => ({
            rule: f.rule,
            category: "",
            profile: "",
            severity: f.severity,
            kind: f.subject?.kind ?? "",
            subject: f.subject?.ref ?? "",
            pin: f.subject?.pin ?? "",
            netId: f.subject?.netId ?? "",
            busId: f.subject?.busId ?? "",
            message: f.message,
            inconclusive: f.inconclusive ?? false,
            context: contextFromWire(f.context),
            sheets: this.doc.sheetBadges(f.sheets ?? []),
            locateReason: f.locateReason ?? 0,
          }));
        }
      } catch {
        this.doc.hasSidecar = false; // no netlist / bad path: no overlay/caption rather than erroring
      }
    }
    // A conformance fixture's assertions become the status-colored highlight overlay (the anchored
    // half), replacing the plain "all findings lit" base, plus the caption (the non-anchored
    // verdict). A real design with no sidecar keeps the findings base and hides the caption.
    if (this.doc.hasSidecar) {
      await this.setHighlights(expectationSpecs(this.reconcileExpectations(), this.doc.expectationFindings));
    }
    this.pushExpectations();
  }

  // reconcileExpectations joins the sidecar against the full-run findings, stamping each row with its
  // rule's catalog summary (WS9-020). Empty when the design has no sidecar.
  private reconcileExpectations(): ExpectationRow[] {
    return this.doc.hasSidecar
      ? reconcile(this.doc.expectations, this.doc.expectationFindings, (rule) => this.doc.rulesByName.get(rule)?.summary ?? "")
      : [];
  }

  // pushExpectations reflects the conformance caption (WS9-045), the set-equality verdict and
  // counts for a design with a sidecar, null (hidden) otherwise. It carries no highlight, because
  // refreshExpectations applies the anchored overlay once and a later focus change must not disturb
  // it.
  private pushExpectations(): void {
    this.views.expectationCaption(this.doc.hasSidecar ? expectationCaption(this.reconcileExpectations()) : null);
  }

  // setHighlights replaces the active highlight layers (the selection API). Each spec selects
  // components/nets/pins with a color/alpha; an empty array clears everything. The WebGL canvas
  // applies specs locally since it holds the packed keys, and SVG mode fetches the matching overlay
  // document and composites it above the sheet. Native mode shows the format's own golden document,
  // which the overlay cannot frame-match, so highlights are not drawn there until the next switch
  // to WebGL/SVG.
  async setHighlights(specs: HighlightSpec[]): Promise<void> {
    this.highlights = specs;
    // Any change to the highlight drops the focused verdict, because every path that replaces the
    // highlight invalidates the claim the URL makes. locateVerdict re-sets it AFTER calling this,
    // so the one path that owns a proof keeps it.
    this.focusedVerdict = "";
    this.canvas.setHighlights(specs);
    // Pushed HERE rather than at each of the seven call sites, because every one has to update the
    // clear control's enabled state.
    this.pushControls();
    await this.refreshSvgOverlay();
  }

  // clearHighlights turns the highlight field off entirely (agni issue 348). Toggling the focused
  // row off is NOT this, because that path restores the base layer.
  //
  // It drops the FOCUS too, so the panel does not keep a selected row beside a cleared drawing and
  // the next style change does not repaint a dismissed focus.
  async clearHighlights(): Promise<void> {
    this.selectedSubject = "";
    this.selectedNetId = "";
    this.views.findings.setFindingLocateNote("");
    this.pushFindings();
    await this.setHighlights([]);
  }

  // refreshSvgOverlay fetches the highlight overlay for the current sheet and stacks it above the
  // SVG document; no highlights (or a failed fetch) clears it instead. Called whenever the specs
  // change or an SVG sheet (re)renders, because the overlay is framed per sheet and must be
  // re-fetched with the base document.
  private async refreshSvgOverlay(): Promise<void> {
    if (this.mode !== "svg" || !this.currentSheet) return;
    if (this.highlights.length === 0) {
      this.render.setSvgOverlay("");
      return;
    }
    try {
      const resp = await this.client.highlightSheet({
        uri: artifactUri(this.doc.mount, this.doc.path),
        sheet: this.currentSheet,
        layout: this.currentLayout,
        symbols: this.faithfulSymbols ? SymbolSource.FAITHFUL : SymbolSource.GLYPH,
        format: SheetFormat.SVG,
        specs: this.highlights,
      });
      this.render.setSvgOverlay(resp.content.case === "svg" ? resp.content.value : "");
    } catch {
      this.render.setSvgOverlay("");
    }
  }

  // selectFinding focuses one finding, highlighting just its subject by kind (net, component, or
  // pin) in whichever renderer is showing. Clicking the focused finding again clears the focus and
  // returns to highlighting every current finding. sheet names an explicit sheet to show (a badge
  // click); without it, a subject absent from the current sheet navigates to the first sheet it
  // lives on (WS9-024). An explicit sheet never toggles the focus off, so clicking a second badge
  // of a focused spanning net just switches sheets.
  //
  // It returns what the focused finding is about, so the host can select it, and null when the click
  // toggled the focus off or named no finding.
  async selectFinding(subject: string, sheet?: string, netId = ""): Promise<Selection | null> {
    const toggleOff = this.selectedSubject === subject && this.selectedNetId === netId && !sheet;
    this.selectedSubject = toggleOff ? "" : subject;
    this.selectedNetId = toggleOff ? "" : netId;
    this.pushFindings();
    // Native shows the tool's own golden document (no overlay); hop to WebGL to see the tint.
    if (!toggleOff && this.mode === "native") await this.setMode("webgl");
    if (toggleOff) {
      this.views.findings.setFindingLocateNote("");
      await this.setHighlights(subjectsToSpecs(this.field())); // back to the field, or to nothing
      return null;
    }
    // A subject may be clicked from the expectations panel (a full-run finding) even when its rule
    // is deselected in the checks panel, so fall back to the full-run findings for the spec.
    const focused = this.findFinding(subject, netId);
    // A bus finding whose bus has no drawn wire (WS7-042c) highlights nothing; surface the
    // server-authoritative reason instead of silently doing nothing, gated on a faithful layout
    // (an auto-layout draws every entity, so the reason does not apply). Mirrors locateEntity.
    const faithful = this.currentLayout === "" || this.currentLayout === "faithful";
    const note = focused && faithful && focused.locateReason !== LocateReason.UNSPECIFIED ? reasonMessage(focused.locateReason, focused.kind, subject) : "";
    this.views.findings.setFindingLocateNote(note);
    const onCurrent = focused?.sheets.some((b) => b.id === this.currentSheet) ?? true;
    const target = sheet ?? (focused && focused.sheets.length > 0 && !onCurrent ? focused.sheets[0].id : "");
    if (target && target !== this.currentSheet) await this.showSheet(target);
    // Focus stacks two layers (WS9-017). The other findings keep their outline, and the focused
    // subject frames on top, a component/pin as a translucent bounding rect and a net as a
    // translucent PATH highlighter along its wire (WS9-040). focusStack drops the focused NET from
    // the base layer so the opaque underlay does not show through its highlighter.
    await this.reapplyFocus(subject, netId);
    return focused ? selectionFromFinding(focused) : null;
  }

  // findFinding locates the focused finding by its per-instance id when given (a net id, or a bus
  // id for a bus finding per WS7-042b; the two namespaces are disjoint), so one of two same-named
  // nets or anonymous buses is picked; else by subject name. It falls back to the full-run
  // expectation findings, since a subject can be clicked from the expectations panel with its rule
  // deselected.
  private findFinding(subject: string, id: string): FindingItem | undefined {
    const match = (f: FindingItem) => (id !== "" ? f.netId === id || f.busId === id : f.subject === subject);
    return this.findings.find(match) ?? this.doc.expectationFindings.find(match);
  }

  // reapplyFocus (re)builds the focus highlight for a subject/instance on the current sheet, stamping
  // the active user style (WS9-044). Shared by selectFinding's non-toggle tail and setHighlightStyle,
  // so changing the style live-updates the currently focused subject.
  private async reapplyFocus(subject: string, netId = ""): Promise<void> {
    const focused = this.findFinding(subject, netId);
    const focusLayers = focused ? withFocusShape(findingSpec(focused), this.highlightStyle) : [];
    await this.setHighlights(focusStack(this.field(), focused ? [{ kind: focused.kind, subject: focused.subject, pin: focused.pin ?? "", netId: focused.netId ?? "" }] : [], focusLayers));
  }

  // showAllFindings says whether the drawing shows every finding of the run at once (agni issue 356).
  // Off by default, since on a sheet with a few dozen findings the field lit most of the drawing in
  // the focus colour before anything was clicked. The Sheets panel's per-sheet counts carry the
  // overview, and a clicked finding is drawn either way.
  private showAllFindings = false;

  // field is the findings drawn as the run-wide field: every one when showAllFindings is on, none
  // when it is off. Every path that paints the field, or stacks a focus over it, reads it here.
  private field(): FindingItem[] {
    return this.showAllFindings ? this.findings : [];
  }

  // setShowAllFindings turns the run-wide field on or off and repaints: a focused finding over the
  // field, or the field alone. Whatever else was located is replaced, as any repaint replaces it.
  async setShowAllFindings(on: boolean): Promise<void> {
    if (this.showAllFindings === on) return;
    this.showAllFindings = on;
    if (this.selectedSubject) await this.reapplyFocus(this.selectedSubject, this.selectedNetId);
    else await this.setHighlights(subjectsToSpecs(this.field()));
  }

  // setHighlightStyle applies a user highlight style (WS9-044) to the focus marker. Subsequent
  // focuses use it, and a currently focused subject re-renders in it immediately. The presenter
  // does not persist the style (that is dock chrome); passing undefined restores the built-in look.
  setHighlightStyle(style: FocusStyle | undefined): void {
    this.highlightStyle = style;
    if (this.selectedSubject) void this.reapplyFocus(this.selectedSubject, this.selectedNetId);
  }

  // showSheet renders one sheet of the current file in the active mode and marks it active.
  // Native rendering shells out to an external tool, which can fail on a given board/sheet; if
  // it does, fall back to the always-available SVG render rather than leaving an error on screen.
  async showSheet(sheetId: string): Promise<void> {
    this.currentSheet = sheetId;
    // The query panel marks the badge pointing at the sheet on screen, so it has to hear about
    // EVERY navigation and not only the ones a result cell started. Navigating by the sheet tabs
    // would otherwise leave the mark on the sheet the reader has left.
    this.views.query?.setCurrentSheet(sheetId);
    this.saveShownView(); // remember the outgoing view before this render replaces it
    this.setBusy(true, "rendering…");
    try {
      try {
        await this.renderSheet(sheetId, formatFor(this.mode));
        this.views.summary(this.doc.summary); // clear any prior transient error
      } catch (e) {
        if (this.mode !== "native") throw e;
        await this.renderSheet(sheetId, SheetFormat.SVG);
        this.views.summary(`${this.doc.summary} · native unavailable, showing SVG`);
      }
    } catch (e) {
      this.views.summary(`error: ${String(e)}`);
    } finally {
      this.setBusy(false);
      this.syncLocation(); // the selected file/sheet/view is the URL, error or not
    }
  }

  // renderSheet fetches one sheet in the given format, draws it in the matching renderer, then
  // restores a remembered pan/zoom for the current (mode, file, sheet); otherwise the fresh fit
  // stands. It keys the view on the active mode, so a native-mode SVG fallback still restores the
  // native slot's view (native and SVG share the SVG host). Throws on RPC failure.
  private async renderSheet(sheetId: string, format: SheetFormat): Promise<void> {
    const symbols = this.faithfulSymbols ? SymbolSource.FAITHFUL : SymbolSource.GLYPH;
    const resp = await this.client.getSheet({ uri: artifactUri(this.doc.mount, this.doc.path), sheet: sheetId, layout: this.currentLayout, format, symbols });
    switch (resp.content.case) {
      case "packed":
        this.canvas.showSheet(resp.content.value);
        this.render.showWebgl();
        break;
      case "svg":
        this.render.showSvg(resp.content.value);
        break;
    }
    const key = this.viewKey(this.mode, sheetId);
    const saved = this.viewMemory.get(key);
    if (saved != null) this.render.setView(this.mode, saved);
    this.shownKey = key;
    this.shownMode = this.mode;
    this.pushSheets(sheetId);
    this.pushControls(); // the board flag follows the shown sheet
    if (sheetId === "board") this.render.setBoardLayers(this.boardLayers); // restore the chosen view
    // Re-stack the highlight overlay for the freshly drawn sheet (showSvg cleared the old
    // one); a no-op in WebGL mode (the canvas reapplies its specs itself) or with no specs.
    await this.refreshSvgOverlay();
  }

  // saveShownView snapshots the currently displayed renderer's pan/zoom into viewMemory, so it
  // can be restored when this (mode, file, sheet) is shown again.
  private saveShownView(): void {
    if (!this.shownKey) return;
    const v = this.render.getView(this.shownMode);
    if (v != null) this.viewMemory.set(this.shownKey, v);
  }

  // setBusy toggles the loading indicator, counting nested renders (openFile -> showSheet) so
  // it only clears once the outermost finishes. An optional label names the phase being started.
  private setBusy(busy: boolean, label?: string): void {
    this.busyDepth += busy ? 1 : -1;
    this.render.setBusy(this.busyDepth > 0, label);
  }

  // setBusyPhase updates the loader's label mid-load without touching the nesting count, so a
  // long openFile can report each stage (loading -> rendering -> running checks) while the overlay
  // stays up. A no-op visually when nothing is busy.
  private setBusyPhase(label: string): void {
    this.render.setBusy(this.busyDepth > 0, label);
  }

  // setMode switches renderer and re-renders the current sheet in the new mode. Native renders
  // the tool's own faithful pages, so entering native snaps the layout to faithful (which
  // re-derives the design and renders it) when it is not already faithful.
  async setMode(mode: RenderMode): Promise<void> {
    if (mode === this.mode) return;
    this.mode = mode;
    if (mode === "native" && this.currentLayout !== "faithful" && this.doc.availableLayouts.includes("faithful")) {
      await this.setLayout("faithful"); // re-opens and pushes controls with the snapped layout
      return;
    }
    this.pushControls();
    if (this.currentSheet) await this.showSheet(this.currentSheet);
  }

  // setLayout switches the geometry layout and re-derives the design (the sheet set changes),
  // then renders. A no-op if the layout is unchanged.
  async setLayout(layout: string): Promise<void> {
    if (layout === this.currentLayout) return;
    this.currentLayout = layout;
    await this.openFile(this.doc.mount, this.doc.path);
  }

  // setSymbols switches an auto-layout's node artwork between synthetic glyphs and the design's
  // own symbols, then re-renders the current sheet. The sheet set is unchanged (same layout), so
  // this re-renders rather than re-deriving the design.
  async setSymbols(faithful: boolean): Promise<void> {
    if (faithful === this.faithfulSymbols) return;
    this.faithfulSymbols = faithful;
    this.pushControls();
    if (this.currentSheet) await this.showSheet(this.currentSheet);
    await this.refreshReport();
  }
}

function summaryLine(path: string, name: string, layout: string, format: string, comps: number, nets: number): string {
  const parts = [name || path, layout];
  if (format) parts.push(format);
  if (comps) parts.push(`${comps} comps`);
  if (nets) parts.push(`${nets} nets`);
  return parts.join(" · ");
}

// isStoreUnconfigured reports the one server condition the review panel must not show as an error,
// a deployment started without --review-store that keeps no runs. Connect maps the service's
// sentinel to FailedPrecondition, and matching on the CODE keeps this working when the message
// wording changes.
function isStoreUnconfigured(e: unknown): boolean {
  return ConnectError.from(e).code === Code.FailedPrecondition;
}

// messageOf pulls a human-readable message off a thrown value for inline display.
function messageOf(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}




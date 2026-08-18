import { Code, ConnectError, type Client } from "@connectrpc/connect";
import { emptyProject, type ProjectState, type ProjectBarView } from "./project.js";
import { artifactUri, uriPath } from "./uri.js";
import { DesignService, SheetFormat, SymbolSource, type SheetRef, type ConversionReport } from "./gen/agni/v1/webapi/design_pb.js";
import { CheckService } from "./gen/agni/v1/webapi/checks_pb.js";
import { QueryService } from "./gen/agni/v1/webapi/query_pb.js";
import { ReviewService } from "./gen/agni/v1/webapi/review_pb.js";
import { ProjectService } from "./gen/agni/v1/webapi/project_pb.js";
import type { ResolveDesignResponse } from "./gen/agni/v1/webapi/project_pb.js";
import { OverlayConfigSchema, type OverlayConfig } from "./gen/agni/v1/webapi/checks_pb.js";
import { type NamingConvention } from "./gen/agni/v1/webapi/config_pb.js";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import type { CanvasComponent } from "./canvas.js";
import type { SheetsView } from "./sheets.js";
import type { ControlsView } from "./controls.js";
import type { ViewerLocation } from "./router.js";
import { type FindingItem, type FindingsView, type SheetBadge, subjectsToSpecs, findingSpec, entitySpecs, focusStack } from "./findings.js";
import { sheetTiles, type OverviewView } from "./sheetoverview.js";
import { reconcile, expectationSpecs, expectationCaption, type RuleExpectationItem, type ExpectationRow, type ExpectationCaption } from "./expectations.js";
import { type RuleItem, type RulesView, defaultSelection } from "./rules.js";
import { withFocusShape, type FocusStyle, type HighlightSpec } from "./highlights.js";
import { type QueryView, LocateReason, emptyResult, errorResult, reasonMessage, resultFromResponse } from "./query.js";
import { type CoverageView, coverageFromResponse, emptyCoverage } from "./coverage.js";
import { type PartsView, partsFromResponse, emptyParts } from "./parts.js";
import { create } from "@bufbuild/protobuf";
import { type ConventionBarView } from "./conventions.js";
import {
  type ChecklistOption,
  type ReviewState,
  type ReviewView,
  checklistOptions,
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
  // setBusy shows/hides a loading indicator around a render (slow on large schematics, and
  // especially for the native shell-out). An optional label names the current phase (loading /
  // rendering / running checks) so the overlay says what it is doing; it is left unchanged when
  // omitted.
  setBusy(busy: boolean, label?: string): void;
  // getView / setView snapshot and restore the active renderer's pan/zoom for the given mode
  // (WebGL -> canvas, SVG/Native -> the SVG host). The value is opaque to the presenter — it
  // stores and replays whatever the renderer returns. getView returns null when there is
  // nothing to snapshot (no sheet drawn yet in that renderer).
  getView(mode: RenderMode): unknown;
  setView(mode: RenderMode, view: unknown): void;
  // setBoardLayers applies board layer visibility ("all" | "front" | "back") to the shown
  // document (WS7-034) — a pure view concern: BoardSVG stratifies into classed groups and
  // the host toggles CSS classes; no re-render. A schematic document has no such classes,
  // so the setting is harmless there.
  setBoardLayers(side: string): void;
}

// ViewSink bundles every view surface the presenter pushes state to (WS9-019): one typed
// object instead of a positional callback per panel, so adding a panel is one field here and
// one line in the composition root. Panel fields are the panels' framework-neutral view
// interfaces (the presenter calls setState, never a DOM node or a Solid signal — C3);
// summary, report, and location stay plain functions because their hosts are not islands.
export interface ViewSink {
  // sheetNavs are the sheet-navigation surfaces (today just the file tree) that stay in sync
  // from this one source; a second surface joins without presenter changes.
  sheetNavs: SheetsView[];
  summary: (text: string) => void;
  // controls receives the full control-bar state (active mode, native availability, layout
  // axis) whenever any of those change.
  controls: ControlsView;
  // findings receives the findings list + selection.
  findings: FindingsView;
  // expectationCaption receives the conformance sidecar's non-anchored verdict (WS9-045): the
  // set-equality counts and the fires:{} silent assertion, as a canvas strip. null hides it (no
  // sidecar). The anchored assertions render as the status-colored highlight overlay, not here.
  expectationCaption: (caption: ExpectationCaption | null) => void;
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
  // query receives the datalog query results (WS9-036) after each run. Optional: hosts without
  // the panel need not wire it, and the presenter no-ops runQuery when it is absent.
  query?: QueryView;
  // coverage receives the per-interface coverage matrix (WS9-041) on each design load. Optional
  // like query; the presenter no-ops refreshCoverage when it is absent.
  coverage?: CoverageView;
  // The datasheet-params panel (WS9-035), optional like coverage; refreshParts no-ops when absent.
  parts?: PartsView;
  // review receives the project's checklist verdict for the open design (WS9-052). Optional like
  // coverage; every review method no-ops when it is absent.
  review?: ReviewView;
  // conventionBar reports which naming vocabulary the answers were computed under (WS9-128).
  // Optional like the panels; setConvention no-ops when it is absent, so a host that offers no
  // convention picker simply always uses the server's default.
  conventionBar?: ConventionBarView;
  // projectBar states which project the open design resolved to and whether the built-in catalog is
  // in effect (agni issue 175). Optional like the rest: a host without it simply never says, which is
  // the pre-project behaviour.
  projectBar?: ProjectBarView;
}

// ViewerPresenter coordinates the viewer's semantic loop: a file selected in the tree is
// loaded (GetDesign), its sheets fill the navigator, and the chosen sheet is rendered in the
// active mode (GetSheet -> WebGL, or GetSheetSvg -> the SVG reference). It is framework-neutral
// — it calls the canvas, the navigator's view interface, and RenderView, never a DOM node or a
// Solid signal (C3). Its runtime is in-process TS because file/sheet/mode switches are
// low-frequency (C7).
export class ViewerPresenter {
  private mount = "";
  private path = "";
  private sheets: SheetRef[] = [];
  private currentSheet = "";
  // Default to SVG: it renders faithfully (with text) for every design, where WebGL is
  // currently lower-fidelity and native is per-format/opt-in.
  private mode: RenderMode = "svg";
  // currentLayout is the geometry axis (faithful / grid / layered / ...), empty until the first
  // design load fills it with the effective layout. It is orthogonal to mode.
  private currentLayout = "";
  // faithfulSymbols draws an auto-layout's nodes with the design's own symbols instead of the
  // synthetic glyphs. Only meaningful when the design ships symbols (availableLayouts has
  // "faithful") and the current layout is an auto-layout.
  private faithfulSymbols = false;
  private availableLayouts: string[] = [];
  private nativeAvailable = false;
  private summary = "";

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

  // rules is the catalog for the open design (ListRules), and selectedRules the active ruleset the
  // user has checked — the subset CheckDesign runs. It defaults to every available rule so a design
  // opens showing all runnable checks. Group-by/filter/search are the panel's own view state.
  private rules: RuleItem[] = [];
  private selectedRules: string[] = [];
  // rulesByName indexes the catalog for the finding->category lookup (findings carry only the rule
  // name on the wire). findingCache holds each rule's findings keyed by rule name, so toggling a
  // rule that has already run is instant (no round-trip) and a new rule fetches only itself; it is
  // cleared when the open design changes. A rule that ran and produced nothing is cached as [] so
  // it is not re-fetched.
  private rulesByName = new Map<string, RuleItem>();
  private readonly findingCache = new Map<string, FindingItem[]>();

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
    // Include the symbol source: faithful symbols scale the layout, so its pan/zoom is its own.
    return `${mode}|${this.currentLayout}|${this.faithfulSymbols ? "f" : "g"}|${this.mount}|${this.path}|${sheetId}`;
  }

  constructor(
    private readonly client: DesignClient,
    // checks is the CheckService client (rules, findings, expectations, the severity report) —
    // its own service since WS9-026, so its own client.
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
    // whose config scored it. Optional like the rest: without it a run stores unparented, which is
    // the correct answer for a design that belongs to no project anyway.
    private readonly projects?: ProjectClient,
  ) {}

  // runQuery evaluates an ad-hoc datalog query over the open design (WS9-036) and pushes the
  // result to the query panel. It needs a file open: with none it reports that as the panel's
  // error rather than calling the service. A parse error or unloadable design comes back as an
  // InvalidArgument, whose message the panel shows inline (search stays in the panel, never a
  // toast). The busy flag rides in the pushed result so the panel disables Run while a query runs.
  async runQuery(text: string): Promise<void> {
    if (!this.views.query || !this.query) return;
    if (!this.mount || !this.path) {
      this.views.query.setState(errorResult("Open a design first."));
      return;
    }
    this.views.query.setState(emptyResult(true));
    try {
      // The overlay goes here too, or the vocabulary bar lies. The bar names the vocabulary the
      // answers on screen were computed under, and a Query panel answering under the server's while
      // the bar said otherwise would be the exact over-claim the bar exists to prevent (WS3-113).
      const resp = await this.query.runQuery({ uri: artifactUri(this.mount, this.path), query: text, overlay: this.overlay() });
      this.views.query.setState(resultFromResponse(resp, (ids) => this.sheetBadges(ids)));
    } catch (e) {
      this.views.query.setState(errorResult(e instanceof Error ? e.message : String(e)));
    }
  }

  // locateEntity focuses a query result cell's entity (WS9-038): it navigates to the given sheet
  // (a badge click) then highlights the component/net, stacking the entity as a translucent focus
  // over any active findings highlight — the same two-layer emphasis selectFinding uses. Native
  // rendering shows the tool's own document with no overlay, so it hops to WebGL to reveal the
  // tint. A query entity is not a finding, so the highlight is built from the bare (kind, subject)
  // via entitySpecs rather than a findings lookup.
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
    // pin is carried because a picked PIN is a different target from its component: the spec builder
    // keys a pin highlight by (ref, pin), and an empty pin would silently widen the focus to the
    // whole part. Callers that locate a net or a component pass nothing and are unaffected.
    const focus = withFocusShape(entitySpecs(kind, subject, pin), this.highlightStyle);
    await this.setHighlights(focusStack(this.findings, kind, subject, focus));
    // Explain an entity the faithful view doesn't draw (WS9-039). The server sets a reason only for
    // an entity absent from the geometry, so a drawn rail (e.g. VBUS) reports none; the note shows
    // only on a faithful layout, since an auto-layout draws every entity and always resolves.
    const faithful = this.currentLayout === "" || this.currentLayout === "faithful";
    const note = faithful && reason !== LocateReason.UNSPECIFIED ? reasonMessage(reason, kind, subject) : "";
    this.views.query?.setLocateNote(note);
  }

  // sheetBadges denormalizes a finding's wire sheet ids into display badges (WS9-024): the id
  // for navigation, the SheetRef's name for the label (falling back to the id). A single-sheet
  // design gets no badges — there is nowhere to navigate — so panels render badges iff present.
  private sheetBadges(ids: string[]): SheetBadge[] {
    if (this.sheets.length <= 1) return [];
    return ids.map((id) => ({ id, name: this.sheets.find((s) => s.id === id)?.name || id }));
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
    const resp = await this.client.getLayoutReport({ uri: artifactUri(this.mount, this.path), symbols });
    this.views.report(resp.report ?? null);
  }

  // pushSheets fans the current sheet state out to every navigation surface.
  private pushSheets(activeId: string): void {
    const state = { mount: this.mount, path: this.path, sheets: this.sheets, activeId };
    for (const nav of this.views.sheetNavs) nav.setState(state);
    this.pushOverview(activeId);
  }

  // pushOverview reflects the per-sheet violation tiles (WS9-025): the design's sheets,
  // each counted from the current findings' sheet badges. Re-pushed when the sheet set,
  // the shown sheet, or the findings change.
  private pushOverview(activeId = this.currentSheet): void {
    this.views.overview?.setState({ tiles: sheetTiles(this.sheets, this.findings), activeId, ruleCount: this.selectedRules.length });
  }

  // boardLayers is the board layer-visibility setting, kept across sheet switches so
  // returning to the board restores the chosen view.
  private boardLayers = "all";

  // pushControls reflects the current renderer/native/layout state to the control bar.
  private pushControls(): void {
    this.views.controls.setState({
      mode: this.mode,
      nativeAvailable: this.nativeAvailable,
      layouts: this.availableLayouts,
      layout: this.currentLayout,
      providedSymbols: this.availableLayouts.includes("faithful"),
      faithfulSymbols: this.faithfulSymbols,
      board: this.currentSheet === "board",
      boardLayers: this.boardLayers,
    });
  }

  // setBoardLayers adopts a layer-visibility choice for the board sheet (WS7-034): a CSS
  // toggle on the shown document, no re-render.
  setBoardLayers(side: string): void {
    this.boardLayers = side;
    this.render.setBoardLayers(side);
    this.pushControls();
  }

  // openFile loads the design behind a tree selection, populates the sheet navigator, reports
  // whether NATIVE is available for it, and renders a sheet in the active mode. wantSheet names
  // the sheet to open (used when restoring a deep link); it defaults to the first sheet, and
  // falls back to the first sheet if the named one is absent from this design.
  // A DIFFERENT design starts from the server's default layout (faithful when the file carries
  // geometry) rather than inheriting the previous design's layout: a sticky auto-layout would
  // draw a geometry-bearing design as a graph instead of as authored. keepLayout opts out for
  // deep-link restores, where the URL's layout must win; a same-file re-open (setLayout) keeps
  // the chosen layout because mount/path are unchanged.
  async openFile(mount: string, path: string, wantSheet = "", keepLayout = false): Promise<void> {
    const newFile = mount !== this.mount || path !== this.path;
    if (!keepLayout && newFile) this.currentLayout = "";
    this.mount = mount;
    this.path = path;
    this.setBusy(true, "loading design…");
    try {
      const d = await this.client.getDesign({ uri: artifactUri(mount, path), layout: this.currentLayout });
      this.sheets = d.sheets;
      // Adopt the effective layout the server chose (the request may have been empty or
      // unavailable for this file), and report the options to the UI.
      this.currentLayout = d.layout;
      this.availableLayouts = d.availableLayouts ?? [];
      // Default the symbol source to faithful when a newly opened design provides its own
      // symbols (availableLayouts has "faithful"), so it draws with real artwork rather than
      // glyphs even after switching to an auto-layout. A restore keeps the URL's choice.
      if (!keepLayout && newFile) this.faithfulSymbols = this.availableLayouts.includes("faithful");
      this.summary = summaryLine(path, d.name, d.layout, d.sourceFormat, d.componentCount, d.netCount);
      this.views.summary(this.summary);
      // If the newly opened file can't render natively but we're in native mode, fall back to
      // SVG (the always-works renderer) so switching files doesn't error.
      if (!d.nativeAvailable && this.mode === "native") this.mode = "svg";
      this.nativeAvailable = d.nativeAvailable;
      this.pushControls();
      // Resolve the project as soon as the design is known, before the report and parts work. Whose
      // config is in effect is something a reader needs while the rest is still loading, and it is
      // what the pickers below are filtered by.
      await this.resolveProject();
      const target = (wantSheet && d.sheets.find((s) => s.id === wantSheet)) || d.sheets[0];
      this.pushSheets(target?.id ?? "");
      if (target) await this.showSheet(target.id);
      else this.syncLocation(); // no sheets to render, but the file selection still owns the URL
      this.findingCache.clear(); // findings are per-design; a new file starts a fresh cache
      this.skippedCache.clear(); // and so is which rules could not run: a new design has new tiers
      this.setBusyPhase("loading rules…");
      await this.loadRules(mount, path); // catalog + default selection; checks now run on demand (Run button)
      this.assembleFindings(); // empty until a run — pushes the "press Run" state + zero overview counts
      await this.setHighlights([]); // clear any highlight carried over from the previously open design
      this.clearExpectations(); // the sidecar reconcile is part of the on-demand run, not the load
      this.setBusyPhase("building report…");
      await this.refreshReport(); // the auto-layout conversion report is not a check — still eager
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
    }
  }

  // restore reopens the viewer at a URL-derived location: it adopts the mode/layout/symbol knobs
  // from the location (so the design loads and renders in that view) and opens the file at the
  // requested sheet. Used on initial load and on browser back/forward, so a refresh or a shared
  // link lands where you left off rather than on the empty shell. It sets the knobs directly
  // (not via setMode/setLayout) to avoid their re-render side effects: the single openFile below
  // does one load, and its native-unavailable guard still corrects an impossible mode.
  async restore(loc: ViewerLocation): Promise<void> {
    if (loc.mode) this.mode = loc.mode;
    this.currentLayout = loc.layout; // empty -> server picks the effective layout, as on a fresh open
    this.faithfulSymbols = loc.symbols;
    await this.openFile(loc.mount, loc.path, loc.sheet, true);
  }

  // currentLoc snapshots the URL-addressable state.
  private currentLoc(): ViewerLocation {
    return {
      mount: this.mount,
      path: this.path,
      isDir: false, // the presenter only ever addresses an open file
      sheet: this.currentSheet,
      mode: this.mode,
      layout: this.currentLayout,
      symbols: this.faithfulSymbols,
    };
  }

  // syncLocation reports the current URL-addressable state to the host (which reflects it into
  // the browser URL). Cheap and idempotent — the host dedupes against the address bar.
  private syncLocation(): void {
    this.views.location?.(this.currentLoc());
  }

  // loadRules fetches the rule catalog for the open design (ListRules), defaults the active ruleset
  // to every available rule, and pushes both to the rules panel. A failure leaves an empty catalog
  // (the panel shows "No rules.") rather than erroring the open.
  private async loadRules(mount: string, path: string): Promise<void> {
    try {
      const resp = await this.checks.listRules({ uri: artifactUri(mount, path), overlay: this.overlay() });
      this.rules = resp.rules.map((r) => ({
        name: r.name,
        severity: r.severity,
        summary: r.summary,
        impact: r.impact,
        detail: r.detail,
        reads: r.reads,
        tags: r.tags,
        available: r.available,
        unavailableReason: r.unavailableReason,
      }));
    } catch {
      this.rules = [];
    }
    this.rulesByName = new Map(this.rules.map((r) => [r.name, r]));
    this.selectedRules = defaultSelection(this.rules);
    this.pushRules();
  }

  // pushRules reflects the current catalog + active selection + per-rule fired counts to the rules
  // panel (the fired count comes from the finding cache, so a rule's badge shows how many findings
  // it produced this run).
  private pushRules(): void {
    // fired counts only the currently selected rules, so the rules-panel badges stay consistent
    // with the checks panel (which shows only the selection): a rule that ran but was then
    // deselected keeps its cached findings but drops out of the fired counts.
    const fired: Record<string, number> = {};
    for (const name of this.selectedRules) fired[name] = this.findingCache.get(name)?.length ?? 0;
    this.views.rules.setState({ rules: this.rules, selected: this.selectedRules, fired });
  }

  // setRuleSelection adopts a new active ruleset (from the rules panel's checkboxes) and re-renders
  // the findings over just that subset FROM THE CACHE — it fetches nothing (checks are on-demand), so
  // toggling a rule is instant. A selected rule with no cache entry shows as pending until the next
  // Run; a deselected rule's findings are simply hidden.
  async setRuleSelection(names: string[]): Promise<void> {
    this.selectedRules = names;
    this.assembleFindings();
    await this.setHighlights(subjectsToSpecs(this.findings));
  }

  // runChecks is the on-demand check trigger (the Run button): it fetches the selected rules not yet
  // cached, reconciles the expectation sidecar (a full run), and refreshes interface coverage. Checks
  // no longer fire on load or on a rule toggle, so this is the single place a design is evaluated and
  // a large design does not stall the open. Re-entrancy and no-open-file are guarded; running disables
  // the button meanwhile.
  async runChecks(): Promise<void> {
    if (this.checksRunning || !this.mount || !this.path) return;
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

  // runSelection fetches CheckDesign for the selected rules not yet cached (a rule that ran with no
  // findings is cached as [] so it is not re-fetched), assembles the visible list, and lights up every
  // current finding. A geometry-only file (no netlist) makes CheckDesign fail, leaving the missing
  // rules uncached (a later valid run retries) and the list empty rather than erroring.
  private async runSelection(): Promise<void> {
    const names = this.selectedRules;
    const missing = names.filter((n) => !this.findingCache.has(n));
    if (missing.length > 0 && this.mount && this.path) {
      try {
        const resp = await this.checks.checkDesign({ uri: artifactUri(this.mount, this.path), rules: missing, overlay: this.overlay() });
        for (const n of missing) {
          this.findingCache.set(n, []); // mark computed (even if it fired nothing)
          this.skippedCache.delete(n); // a rerun may have made it runnable (a board was attached)
        }
        // ?? [] because a hand-built response (a test stub, an older server) may omit it, and a
        // missing field must degrade to "nothing was skipped" rather than throwing mid-run.
        for (const sk of resp.skipped ?? []) this.skippedCache.set(sk.name, sk.reason);
        for (const f of resp.findings) {
          this.findingCache.get(f.rule)?.push({
            rule: f.rule,
            category: this.rulesByName.get(f.rule)?.tags.category ?? "",
            profile: this.rulesByName.get(f.rule)?.tags.profile ?? "",
            severity: f.severity,
            kind: f.subject?.kind ?? "",
            subject: f.subject?.ref ?? "",
            pin: f.subject?.pin ?? "",
            netId: f.subject?.netId ?? "",
            busId: f.subject?.busId ?? "",
            message: f.message,
            sheets: this.sheetBadges(f.sheets ?? []),
            locateReason: f.locateReason ?? 0,
          });
        }
      } catch {
        // no netlist etc.: leave missing rules uncached so a later valid selection retries.
      }
    }
    this.assembleFindings();
    await this.setHighlights(subjectsToSpecs(this.findings)); // light up every current finding at once
  }

  // assembleFindings rebuilds the visible list from the per-rule cache for the active ruleset (a
  // stable rule-then-subject base order; the panel re-sorts), clears the focus, and pushes the checks,
  // rules (fired counts), and overview (per-sheet counts) panels. It fetches nothing.
  private assembleFindings(): void {
    this.findings = this.selectedRules
      .flatMap((n) => this.findingCache.get(n) ?? [])
      .sort((a, b) => (a.rule !== b.rule ? (a.rule < b.rule ? -1 : 1) : a.subject < b.subject ? -1 : a.subject > b.subject ? 1 : 0));
    this.selectedSubject = "";
    this.pushFindings();
    this.pushRules(); // fired counts changed
    this.pushOverview(); // per-sheet counts follow the same findings
  }

  // pushFindings reflects the current findings + focus + selection to the checks panel, including the
  // pending count (selected rules not yet run — badges the Run button and tells "press Run" from "ran
  // clean") and the rule catalog summaries (the per-rule one-liners the merged panel shows).
  private pushFindings(): void {
    const pending = this.selectedRules.filter((n) => !this.findingCache.has(n)).length;
    const ruleSummaries: Record<string, string> = {};
    for (const r of this.rules) if (r.summary) ruleSummaries[r.name] = r.summary;
    this.views.findings.setState({
      findings: this.findings,
      selected: this.selectedSubject,
      ruleCount: this.selectedRules.length,
      pending,
      running: this.checksRunning,
      // Only the SELECTED rules, since a rule nobody ticked being unavailable is not news. Sorted so
      // the list is stable across runs rather than following response order.
      skipped: this.selectedRules
        .filter((n) => this.skippedCache.has(n))
        .map((n) => ({ rule: n, reason: this.skippedCache.get(n) ?? "" }))
        .sort((a, b) => a.rule.localeCompare(b.rule)),
      ruleSummaries,
    });
  }

  // skippedCache is rule name -> why it could not run on this design, alongside findingCache. It is a
  // cache for the same reason: checks are on-demand and per rule, so a rule's gated-ness is learned
  // when it is first run and has to survive until the design changes.
  private skippedCache = new Map<string, string>();

  private expectations: RuleExpectationItem[] = [];
  private expectationFindings: FindingItem[] = [];
  private hasSidecar = false;

  // ---- Naming convention (WS9-128) -----------------------------------------------------------
  //
  // A request may carry its own naming convention, which REPLACES the server's startup default for
  // that request (WS3-124). The presenter holds the chosen one and stamps it onto every rule-running
  // call, so a user asking "what does this board look like under my vocabulary" gets a consistent
  // answer across the checks panel, the report, and a review run.

  // convention is the resolved value sent on each request, null while the server's default applies.
  // conventionRef is the ref it was resolved from, so the UI can name it.
  private convention: NamingConvention | null = null;
  private conventionRef = "";

  // overlay is what every rule-running call carries. It is a method rather than a field so a caller
  // cannot forget: the three call sites that run rules all go through it, and a fourth added later
  // that did not would be visibly different from its neighbours.
  private overlay(): OverlayConfig | undefined {
    if (!this.convention && !this.projectState.plain) return undefined;
    return create(OverlayConfigSchema, {
      // The convention rides inside the shared AnalysisConfig, the same message a Project declares, so
      // a config tier added there is one a request can carry without a second edit here.
      config: { conventions: this.convention ?? undefined },
      // ignore_project is the "show me the built-in catalog" choice. It rides on every rule-running
      // request rather than being applied once, because each surface composes its own overlay: a
      // toggle that reached the check panel but not the rules list would show findings from one
      // catalog beside the rule set of another.
      ignoreProject: this.projectState.plain,
    });
  }

  // resolveProject asks which project the open design belongs to and pushes the answer to the bar.
  //
  // The viewer resolves and SHOWS, rather than the server's loader silently swapping in the design's
  // entry. That was the open question from the resource work, and the browser is what settles it: the
  // CLI can print a line saying which file it actually read, and a silent swap in a viewer has no
  // equivalent — the user picked a file in a tree and would be looking at a different one.
  private async resolveProject(): Promise<void> {
    if (!this.views.projectBar && !this.views.review && !this.views.conventionBar) return;
    const plain = this.projectState.plain;
    this.projectState = { ...emptyProject(), plain, busy: true };
    this.pushProject();
    if (!this.projects || !this.mount || !this.path) {
      this.projectState = { ...emptyProject(), plain };
      this.pushProject();
      return;
    }
    const uri = artifactUri(this.mount, this.path);
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
  // re-runs whatever is on screen. Re-running is the point: the toggle answers "whose rules are
  // these" by subtraction, and subtraction needs the second run.
  async setPlainCatalog(plain: boolean): Promise<void> {
    if (this.projectState.plain === plain) return;
    this.projectState = { ...this.projectState, plain };
    this.pushProject();
    // The rule LIST is recomposed too, not just the findings. A toggle that changed which rules ran
    // but left the panel listing the other catalog's rules would show findings from one and the rule
    // set of another, which is the drift the toggle exists to make visible.
    await this.loadRules(this.mount, this.path);
    await this.runChecks();
  }

  // setConvention resolves a stored convention config and applies it to subsequent runs, or clears
  // back to the server's default when ref is empty.
  //
  // It resolves through the SERVER (GetNamingConvention) rather than parsing YAML here, because the
  // browser holds a ref and no filesystem and because the config's validity is the engine's call: a
  // pattern that will not compile is rejected once, here, instead of on every run that carries it.
  //
  // Cached findings are dropped, and that is the point rather than housekeeping. A convention changes
  // which rules exist and what the engine believes a rail IS, so every cached finding was computed
  // under a different question. Keeping them would mix two vocabularies in one list.
  async setConvention(ref: string): Promise<void> {
    if (!this.views.conventionBar) return;
    if (ref === "") {
      this.convention = null;
      this.conventionRef = "";
      await this.reloadForConvention();
      this.pushConvention();
      return;
    }
    this.pushConvention(true);
    try {
      const resp = await this.checks.getNamingConvention({ uri: artifactUri(this.mount, ref) });
      this.convention = resp.convention ?? null;
      this.conventionRef = ref;
      await this.reloadForConvention();
      this.pushConvention();
    } catch (e) {
      this.pushConvention(false, messageOf(e));
    }
  }

  // reloadForConvention re-reads the rule CATALOG and drops cached findings after the vocabulary
  // changes.
  //
  // Reloading the catalog is the part that is easy to miss and expensive to get wrong. A request
  // convention replaces the server's, so the set of rules that EXIST changes: the server's naming
  // rules disappear and the request's appear under a different namespace. A client that kept the old
  // catalog would hold a selection naming rules that no longer exist, run none of them, and show no
  // naming findings at all — which reads as a design with no naming problems rather than as a client
  // asking the wrong question.
  private async reloadForConvention(): Promise<void> {
    this.findingCache.clear();
    this.skippedCache.clear();
    // Query results were computed under the previous vocabulary too, and `rail` answering differently
    // is the whole point of the feature, so leaving them on screen would show two vocabularies at once.
    this.views.query?.setState(emptyResult(false));
    if (this.mount && this.path) await this.loadRules(this.mount, this.path);
    this.assembleFindings();
  }

  // pushConvention reports which vocabulary is in effect. It is a first-class piece of state rather
  // than a label, because a finding that changed because the vocabulary changed and one that changed
  // because the design changed are not the same claim, and nothing in a findings list distinguishes
  // them (WS9-128).
  private pushConvention(busy = false, error = ""): void {
    this.views.conventionBar?.setState({
      choices: this.conventionChoices,
      active: this.conventionRef,
      name: this.convention?.name ?? "",
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
  // A review RUN is a resource (WS9-053), so the panel is not a "run and render" surface: it shows
  // the runs already stored for this design, and creating one is an explicit act. That shape is the
  // point rather than an accident of the API. A checklist verdict is compared between revisions, so
  // a panel that could only ever show the latest run would rebuild the gap the resource model closed.

  // reviewState is the panel's pushed state, held here because the presenter owns the interaction
  // loop (which run is selected, which checklist is chosen) and the panel is a humble view (C3).
  private reviewState: ReviewState = emptyReview();

  // pushReview sends the current state to the panel, a no-op when the panel is unwired.
  private pushReview(): void {
    this.views.review?.setState({ ...this.reviewState });
  }

  // refreshReviews loads the runs stored for the open design plus the checklists sitting beside it.
  // It runs on each design load, so opening a reviewed board shows its latest verdict rather than an
  // empty panel.
  //
  // A server started without --review-store answers with a failed precondition, which is recorded as
  // storeConfigured=false rather than as an error. That distinction is the panel's, and it matters:
  // "this deployment keeps no runs" and "nobody has reviewed this board" look identical in an empty
  // list, and a user told the wrong one goes hunting for a button that was never going to appear.
  private async refreshReviews(): Promise<void> {
    if (!this.views.review || !this.reviews) return;
    this.reviewState = emptyReview();
    if (!this.mount || !this.path) {
      this.pushReview();
      return;
    }
    this.reviewState.checklists = this.checklistChoices;
    this.reviewState.checklist = this.reviewState.checklists[0]?.ref ?? "";
    try {
      const resp = await this.reviews.listReviews({ filter: `design="${artifactUri(this.mount, this.path)}"` });
      this.reviewState.runs = resp.reviews.map(reviewFromWire);
      // Newest first comes from the server, so the head is the latest run.
      this.reviewState.selected = this.reviewState.runs[0]?.name ?? "";
    } catch (e) {
      if (isStoreUnconfigured(e)) this.reviewState.storeConfigured = false;
      else this.reviewState.error = messageOf(e);
    }
    this.pushReview();
  }

  // pickerChoices are the config files the convention and checklist pickers offer, ONE LIST PER KIND.
  //
  // When the design resolves to a project, they are what the PROJECT DECLARES — because it does
  // declare them, and a declaration beats a guess. Before this the pickers listed every YAML sitting
  // beside the design and could not tell which were really their own kind, so they offered
  // design-intent files and descriptors that could never resolve; conventionbar's own doc called
  // choosing one "an EXPECTED path, not an edge case".
  //
  // The kinds are kept APART. A checklist and a naming convention are different file formats parsed
  // by different rpcs, so a single shared list only moves the problem: offering `review.yaml` in the
  // vocabulary picker fails exactly the way offering `intent.yaml` did, with a YAML error naming a
  // field the other schema has never heard of. Profiles appear in NEITHER, because a profile is
  // composed into the catalog rather than selected as a vocabulary or run as a checklist; there is no
  // picker it is an answer to.
  //
  // A design with no project falls back to listing the siblings for both, which is the honest answer:
  // nothing declared anything, so the picker cannot know, and offering a file that turns out not to be
  // a checklist costs one clear error where hiding a real one costs a user their own file.
  private async pickerChoices(): Promise<{ conventions: ChecklistOption[]; checklists: ChecklistOption[] }> {
    const cfg = this.projectResolved?.project?.config;
    if (cfg && (cfg.conventionsUri || cfg.checklistUri)) {
      return {
        conventions: optionsFor(cfg.conventionsUri),
        checklists: optionsFor(cfg.checklistUri),
      };
    }
    const siblings = await this.yamlSiblings();
    return { conventions: siblings, checklists: siblings };
  }

  // yamlSiblings lists the design's own directory and keeps the YAML files. It deliberately does not
  // read them: whether a YAML file is a checklist or a naming convention is decided by the server
  // (GetReviewManifest / GetNamingConvention), which parse and validate. Guessing here would mean
  // either hiding a user's real file or duplicating two parsers in the browser.
  //
  // One listing feeds BOTH pickers. A project keeps its review.yaml and its conventions.yaml side by
  // side, and two calls asking the same question of the same directory would be two chances to
  // disagree about what is there.
  private async yamlSiblings(): Promise<ChecklistOption[]> {
    if (!this.workspace) return [];
    const slash = this.path.lastIndexOf("/");
    const dir = slash < 0 ? "" : this.path.slice(0, slash);
    try {
      const resp = await this.workspace.listDir({ uri: artifactUri(this.mount, dir) });
      return checklistOptions(resp.entries.map((e) => ({ name: e.name, path: uriPath(e.uri), isDir: e.isDir })));
    } catch {
      return [];
    }
  }

  // designParent is the project resource name the open design belongs to, "" when it belongs to none
  // or when resolution fails. A failure is not worth surfacing here: the run still stores, just
  // unparented, which is strictly better than refusing to record a review that ran.
  private async designParent(): Promise<string> {
    if (!this.projects || !this.mount || !this.path) return "";
    try {
      const resp = await this.projects.resolveDesign({ uri: artifactUri(this.mount, this.path) });
      return resp.project?.name ?? "";
    } catch {
      return "";
    }
  }

  // createReview runs the chosen checklist against the open design and stores the result, then shows
  // it. The manifest is resolved server-side first (GetReviewManifest) and sent back as a VALUE, which
  // is the C22 seam: the browser holds a ref and no filesystem, so the one read is a named rpc rather
  // than something the run does behind its back.
  async createReview(): Promise<void> {
    if (!this.views.review || !this.reviews) return;
    if (this.reviewState.running) return;
    if (!this.mount || !this.path || !this.reviewState.checklist) return;
    this.reviewState = { ...this.reviewState, running: true, error: "" };
    this.pushReview();
    this.setBusy(true, "running review…");
    try {
      const man = await this.reviews.getReviewManifest({ uri: artifactUri(this.mount, this.reviewState.checklist) });
      const created = await this.reviews.createReview({
        // A run is stored under the design's project when it has one. Resolving it here rather than
        // letting the server re-derive it keeps the run filed under the same project whose config
        // scored it, and an empty parent is a real answer: a design on a mounted folder often
        // belongs to no project, and its runs belong to none either.
        parent: await this.designParent(),
        designUri: artifactUri(this.mount, this.path),
        manifest: man.manifest,
        overlay: this.overlay(),
      });
      const run = reviewFromWire(created);
      // Prepend rather than refetch: the new run is the newest, and the server hands the whole
      // document back, so a second round-trip would only re-fetch what is already in hand.
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

  // refreshCoverage fetches the interface-coverage matrix for the open design (WS9-041) and pushes
  // it to the coverage panel. Optional panel: a no-op when unwired. A design with no detected
  // interface (or a fetch failure) pushes an empty state, so the panel shows its empty message
  // rather than stale coverage from the previous design.
  private async refreshCoverage(): Promise<void> {
    if (!this.views.coverage) return;
    if (!this.mount || !this.path) {
      this.views.coverage.setState(emptyCoverage());
      return;
    }
    try {
      const resp = await this.checks.getInterfaceCoverage({ uri: artifactUri(this.mount, this.path) });
      this.views.coverage.setState(coverageFromResponse(resp));
    } catch {
      this.views.coverage.setState(emptyCoverage());
    }
  }

  // refreshParts fetches the datasheet-params join for the open design (WS9-035) and pushes it to
  // the parts panel. Optional panel: a no-op when unwired. A design with no joined specs (or a serve
  // started without --params, or a fetch failure) pushes an empty state, so the panel shows its
  // empty hint rather than stale parts from the previous design.
  private async refreshParts(): Promise<void> {
    if (!this.views.parts) return;
    if (!this.mount || !this.path) {
      this.views.parts.setState(emptyParts());
      return;
    }
    try {
      const resp = await this.checks.getComponentParams({ uri: artifactUri(this.mount, this.path) });
      this.views.parts.setState(partsFromResponse(resp));
    } catch {
      this.views.parts.setState(emptyParts());
    }
  }

  // clearExpectations resets the expectations panel to empty and pushes it. Called on design load,
  // because the sidecar reconcile is now part of the on-demand Run (refreshExpectations) rather than
  // the load — before the first Run the panel shows its neutral empty state instead of reconciling
  // against zero findings (which would mislabel every non-pending expectation "missing").
  private clearExpectations(): void {
    this.expectations = [];
    this.expectationFindings = [];
    this.hasSidecar = false;
    this.pushExpectations();
  }

  // refreshExpectations loads the design's expectation sidecar and reconciles it for the panel. It
  // reconciles against a FULL check run (every rule), not the checks-panel's selection, so a
  // "missing" row means the rule ran and did not fire, not that it was deselected. Runs as part of the
  // on-demand check run (runChecks); a design with no sidecar leaves the panel empty.
  private async refreshExpectations(): Promise<void> {
    this.expectations = [];
    this.expectationFindings = [];
    this.hasSidecar = false;
    if (this.mount && this.path) {
      try {
        const exp = await this.checks.getExpectations({ uri: artifactUri(this.mount, this.path) });
        this.hasSidecar = exp.hasSidecar;
        this.expectations = exp.expectations.map((e) => ({ rule: e.rule, subjects: e.subjects, pending: e.pending, why: e.why }));
        if (this.hasSidecar) {
          const resp = await this.checks.checkDesign({ uri: artifactUri(this.mount, this.path), rules: [], overlay: this.overlay() });
          this.expectationFindings = resp.findings.map((f) => ({
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
            sheets: this.sheetBadges(f.sheets ?? []),
            locateReason: f.locateReason ?? 0,
          }));
        }
      } catch {
        this.hasSidecar = false; // no netlist / bad path: no overlay/caption rather than erroring
      }
    }
    // A conformance fixture's assertions become the status-colored highlight overlay (the anchored
    // half) — replacing the plain "all findings lit" base for this design — plus the caption (the
    // non-anchored verdict). A real design (no sidecar) keeps the findings base and hides the caption.
    if (this.hasSidecar) {
      await this.setHighlights(expectationSpecs(this.reconcileExpectations(), this.expectationFindings));
    }
    this.pushExpectations();
  }

  // reconcileExpectations joins the sidecar against the full-run findings, stamping each row with its
  // rule's catalog summary (WS9-020). Empty when the design has no sidecar.
  private reconcileExpectations(): ExpectationRow[] {
    return this.hasSidecar
      ? reconcile(this.expectations, this.expectationFindings, (rule) => this.rulesByName.get(rule)?.summary ?? "")
      : [];
  }

  // pushExpectations reflects the conformance caption (WS9-045): the set-equality verdict + counts for
  // a design with a sidecar, null (hidden) otherwise. It carries no highlight — the anchored overlay is
  // applied once by refreshExpectations, so a later focus change does not disturb it.
  private pushExpectations(): void {
    this.views.expectationCaption(this.hasSidecar ? expectationCaption(this.reconcileExpectations()) : null);
  }

  // setHighlights replaces the active highlight layers — the selection/unselection API. Each
  // spec selects components/nets/pins with a color/alpha; an empty array clears everything.
  // The WebGL canvas applies specs locally (it holds the packed keys, so no round-trip); SVG
  // mode fetches the matching overlay document and composites it above the sheet. Native mode
  // shows the format's own golden document, which the overlay cannot frame-match, so
  // highlights are simply not drawn there (they appear on the next switch to WebGL/SVG).
  async setHighlights(specs: HighlightSpec[]): Promise<void> {
    this.highlights = specs;
    this.canvas.setHighlights(specs);
    await this.refreshSvgOverlay();
  }

  // refreshSvgOverlay fetches the highlight overlay for the current sheet and stacks it above
  // the SVG document; no highlights (or a failed fetch) clears it instead. Called whenever the
  // specs change or an SVG sheet (re)renders — the overlay is framed per sheet, so it must be
  // re-fetched with the base document.
  private async refreshSvgOverlay(): Promise<void> {
    if (this.mode !== "svg" || !this.currentSheet) return;
    if (this.highlights.length === 0) {
      this.render.setSvgOverlay("");
      return;
    }
    try {
      const resp = await this.client.highlightSheet({
        uri: artifactUri(this.mount, this.path),
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

  // selectFinding focuses one finding: it highlights just that finding's subject (exactly by its
  // kind — net, component, or pin), in whichever renderer is showing. Clicking the focused finding
  // again clears the focus and returns to the whole-selection highlight (every current finding).
  // sheet names an explicit sheet to show (a badge click); without it, a subject that does not
  // appear on the current sheet navigates to the first sheet it lives on before highlighting
  // (WS9-024) — previously the highlight join silently missed. An explicit sheet never toggles
  // the focus off, so clicking a second badge of a focused spanning net just switches sheets.
  async selectFinding(subject: string, sheet?: string, netId = ""): Promise<void> {
    const toggleOff = this.selectedSubject === subject && this.selectedNetId === netId && !sheet;
    this.selectedSubject = toggleOff ? "" : subject;
    this.selectedNetId = toggleOff ? "" : netId;
    this.pushFindings();
    // Native shows the tool's own golden document (no overlay); hop to WebGL to see the tint.
    if (!toggleOff && this.mode === "native") await this.setMode("webgl");
    if (toggleOff) {
      this.views.findings.setFindingLocateNote("");
      await this.setHighlights(subjectsToSpecs(this.findings)); // back to all subjects
      return;
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
    // Focus stacks two layers (WS9-017): the other findings keep their outline, and the focused
    // subject frames on top — a component/pin as a translucent bounding rect, a net as a
    // translucent PATH highlighter along its wire (WS9-040) — so the selection reads as emphasis
    // without losing the surrounding finding context. focusStack drops the focused NET from the
    // base layer so the opaque underlay does not show through its translucent highlighter.
    await this.reapplyFocus(subject, netId);
  }

  // findFinding locates the focused finding by its per-instance id when given (a net id, or a bus
  // id for a bus finding — WS7-042b — the two namespaces are disjoint), so one of two same-named
  // nets or anonymous buses is picked; else by subject name. It falls back to the full-run
  // expectation findings, since a subject can be clicked from the expectations panel with its rule
  // deselected.
  private findFinding(subject: string, id: string): FindingItem | undefined {
    const match = (f: FindingItem) => (id !== "" ? f.netId === id || f.busId === id : f.subject === subject);
    return this.findings.find(match) ?? this.expectationFindings.find(match);
  }

  // reapplyFocus (re)builds the focus highlight for a subject/instance on the current sheet, stamping
  // the active user style (WS9-044). Shared by selectFinding's non-toggle tail and setHighlightStyle,
  // so changing the style live-updates the currently focused subject.
  private async reapplyFocus(subject: string, netId = ""): Promise<void> {
    const focused = this.findFinding(subject, netId);
    const focusLayers = focused ? withFocusShape(findingSpec(focused), this.highlightStyle) : [];
    await this.setHighlights(focusStack(this.findings, focused?.kind ?? "", focused?.subject ?? "", focusLayers, focused?.netId ?? ""));
  }

  // setHighlightStyle applies a user highlight style (WS9-044) to the focus marker: subsequent
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
        this.views.summary(this.summary); // clear any prior transient error
      } catch (e) {
        if (this.mode !== "native") throw e;
        await this.renderSheet(sheetId, SheetFormat.SVG);
        this.views.summary(`${this.summary} · native unavailable, showing SVG`);
      }
    } catch (e) {
      this.views.summary(`error: ${String(e)}`);
    } finally {
      this.setBusy(false);
      this.syncLocation(); // the selected file/sheet/view is the URL, error or not
    }
  }

  // renderSheet fetches one sheet in the given format, draws it in the matching renderer, then
  // restores a remembered pan/zoom for the current (mode, file, sheet) — else the fresh fit
  // stands. It keys the view on the active mode, so a native-mode SVG fallback still restores
  // the native slot's view (native and SVG share the SVG host). Throws on RPC failure.
  private async renderSheet(sheetId: string, format: SheetFormat): Promise<void> {
    const symbols = this.faithfulSymbols ? SymbolSource.FAITHFUL : SymbolSource.GLYPH;
    const resp = await this.client.getSheet({ uri: artifactUri(this.mount, this.path), sheet: sheetId, layout: this.currentLayout, format, symbols });
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
    if (mode === "native" && this.currentLayout !== "faithful" && this.availableLayouts.includes("faithful")) {
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
    await this.openFile(this.mount, this.path);
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

// isStoreUnconfigured reports the one server condition the review panel must not show as an error:
// the deployment was started without --review-store, so it keeps no runs at all. Connect maps the
// service's sentinel to FailedPrecondition, and matching on the CODE rather than the message keeps
// this from breaking when the wording changes.
function isStoreUnconfigured(e: unknown): boolean {
  return ConnectError.from(e).code === Code.FailedPrecondition;
}

// messageOf pulls a human-readable message off a thrown value for inline display.
function messageOf(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

// baseOf is the final path element of an artifact URI, for a picker label.
function baseOf(uri: string): string {
  const p = uriPath(uri);
  const i = p.lastIndexOf("/");
  return i < 0 ? p : p.slice(i + 1);
}

// optionsFor turns one declared URI into a picker's option list, empty when the project declared
// none. A project that declares no convention offers only the server's, which is the truthful state:
// there is no project convention to go back to.
function optionsFor(uri: string | undefined): ChecklistOption[] {
  return uri ? [{ ref: uriPath(uri), label: baseOf(uri) }] : [];
}

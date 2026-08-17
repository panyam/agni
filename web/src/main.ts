// Entry point for the web viewer shell. The page is server-rendered by goapplib/templar
// (the border-layout shell with a file-tree sidebar, canvas region, and detail panel).
// This boots the tsappkit lifecycle over that shell: AppRoot discovers the interactive
// regions and returns them as child components, and the LifecycleController initializes
// each one. Today the only region is the WebGL canvas; the file-tree and detail-panel
// islands join here in later tickets (WS9-002, WS9-005).

import { BaseComponent, EventBus, LifecycleController, type LCMComponent } from "@panyam/tsappkit";
import { CanvasComponent } from "./canvas.js";
import { comparePickerIsland } from "./comparepicker.js";
import { controlBarIsland } from "./controlbar.js";
import { sheetTabsIsland } from "./sheettabs.js";
import { findingsPanelIsland } from "./findingspanel.js";
import { rulesPanelIsland } from "./rulespanel.js";
import { diffPanelIsland } from "./diffpanel.js";
import { diffChangesPanelIsland } from "./diffchangespanel.js";
import { sheetOverviewPanelIsland } from "./sheetoverviewpanel.js";
import { queryPanelIsland } from "./querypanel.js";
import { coveragePanelIsland } from "./coveragepanel.js";
import { reviewPanelIsland } from "./reviewpanel.js";
import { conventionBarIsland } from "./conventionbar.js";
import { projectBarIsland } from "./projectbar.js";
import { partsPanelIsland } from "./partspanel.js";
import { ViewerPresenter, type RenderView } from "./viewer.js";
import { DiffPresenter, type DiffRenderView, type DiffSideView } from "./diffpresenter.js";
import { SvgView } from "./svgview.js";
import { compareButton } from "./compare.js";
import { designClient, checksClient, diffClient, queryClient, reviewClient, workspaceClient,
  projectClient,
} from "./api.js";
import { createViewerDock, openDiffPanel, closeDiffPanel } from "./dock.js";
import { highlightMenu, loadHighlightStyle } from "./highlightstyle.js";
import { currentLocation, hasFile, locationToUrl, type ViewerLocation } from "./router.js";
import { GROUP_BOARD_COPPER_BACK, GROUP_BOARD_COPPER_FRONT } from "./packed.js";
import { delayedBusy } from "./busy.js";
import { expectationCaptionStrip } from "./expectcaption.js";
import { queryFor } from "./selection.js";
import { baseName, noteOpen } from "./recents.js";

// restoring guards the URL feedback loop: while we apply a URL to the presenter (initial load or
// back/forward), the presenter's onLocation still fires, but we must not push a new history entry
// for state we are merely replaying. It is a module-level flag because both the presenter callback
// and the restore driver below need it.
let restoring = false;

// syncUrl reflects a location (a file the presenter opened, or a folder the tree selected) into
// the address bar. It sets the tab title always, but only pushes history when the URL actually
// changed and we are not mid-restore, so normal navigation builds a back-stack while a
// refresh/back-forward replay does not.
function syncUrl(loc: ViewerLocation): void {
  document.title = hasFile(loc) ? `${loc.path || loc.mount} — Agni` : "Agni viewer";
  if (restoring) return;
  const url = locationToUrl(loc);
  if (url !== window.location.pathname + window.location.search) window.history.pushState(null, "", url);
}

class AppRoot extends BaseComponent {
  // presenter is exposed so the boot code can drive a deep-link restore once the islands are
  // initialized (it is created in performLocalInit, below).
  presenter: ViewerPresenter | null = null;

  override performLocalInit(): LCMComponent[] {
    const children: LCMComponent[] = [];

    const canvasEl = document.getElementById("view");
    const pickerEl = document.getElementById("compare-picker");
    const compareTreeEl = document.getElementById("compare-tree");
    const svgEl = document.getElementById("svg-view");
    const controlsEl = document.getElementById("controls");
    const findingsEl = document.getElementById("findings");
    const rulesEl = document.getElementById("rules");
    const compareEl = document.getElementById("compare");
    const diffBarEl = document.getElementById("diff-bar");
    const diffSvgA = document.getElementById("diff-svg-a");
    const diffSvgB = document.getElementById("diff-svg-b");
    const diffPhA = document.getElementById("diff-ph-a");
    const diffPhB = document.getElementById("diff-ph-b");
    const diffChangesEl = document.getElementById("diff-changes");
    const sheetOverviewEl = document.getElementById("sheet-overview");
    const queryEl = document.getElementById("query-panel");
    const coverageEl = document.getElementById("coverage-panel");
    const reviewEl = document.getElementById("review-panel");
    const conventionEl = document.getElementById("convention-bar");
    const projectEl = document.getElementById("project-bar");
    const partsEl = document.getElementById("parts-panel");
    const sheetTabsEl = document.getElementById("sheet-tabs");
    if (!canvasEl || !pickerEl || !compareTreeEl || !svgEl || !controlsEl || !findingsEl || !rulesEl || !sheetTabsEl)
      return children;
    if (!compareEl || !diffBarEl || !diffSvgA || !diffSvgB || !diffPhA || !diffPhB || !diffChangesEl)
      return children;
    if (!sheetOverviewEl || !queryEl || !coverageEl || !partsEl || !reviewEl || !conventionEl) return children;
    if (!projectEl) return children;

    // RenderView reveals whichever renderer drew the sheet: the SVG host overlays the canvas,
    // so showWebgl just hides it and showSvg fills + shows it.
    const busyEl = document.getElementById("render-busy");
    // One app-level loader overlay drives both the viewer and the diff (they never run at once);
    // the shared delayedBusy keeps a single show-timer for it (WS7-043/044).
    const setBusyOverlay = delayedBusy(busyEl);
    // WS9-045: the conformance expectation verdict strip (a plain-DOM sink, like the busy overlay;
    // hidden on any design without a sidecar).
    const setExpectCaption = expectationCaptionStrip(document.getElementById("expect-caption"));
    const readoutEl = document.getElementById("readout");
    const svgView = new SvgView(svgEl);
    // Clicking the drawing selects what is under the cursor and asks the query engine about it. The
    // wiring is deferred to the end of this method because it needs the query panel, which is built
    // below; see the assignment after the panels.
    const canvas = new CanvasComponent("canvas", canvasEl, this._eventBus);
    const renderView: RenderView = {
      showWebgl: () => {
        svgView.hide();
        canvas.showText(); // the text layer belongs to the WebGL view
        if (readoutEl) readoutEl.style.display = ""; // canvas owns the readout in WebGL mode
      },
      showSvg: (markup) => {
        // Show before setSvg so fit() measures a laid-out (non-zero) host.
        canvas.hideText(); // the SVG host renders its own text
        svgView.show();
        svgView.setSvg(markup);
        // Keep the readout visible in SVG mode too; SVG has no vertex buffer, so it reports the
        // drawn-element count rather than primitives/vertices.
        if (readoutEl) {
          readoutEl.textContent = `SVG — ${svgView.stats().elements} elements`;
          readoutEl.style.display = "";
        }
      },
      setSvgOverlay: (markup) => svgView.setOverlay(markup),
      setBusy: setBusyOverlay,
      // WebGL views live on the canvas; SVG and Native both use the SVG host.
      getView: (mode) => (mode === "webgl" ? canvas.getView() : svgView.getView()),
      setView: (mode, view) => {
        if (mode === "webgl") canvas.setView(view as ReturnType<typeof canvas.getView> & object);
        else svgView.setView(view as ReturnType<typeof svgView.getView>);
      },
      // Board layer visibility (WS7-034/035): CSS classes over BoardSVG's classed strata,
      // and hidden packed groups on the WebGL canvas (the packed board's back/front strata).
      setBoardLayers: (side) => {
        svgEl.classList.remove("board-front", "board-back");
        if (side === "front" || side === "back") svgEl.classList.add(`board-${side}`);
        const hidden = side === "front" ? [GROUP_BOARD_COPPER_BACK] : side === "back" ? [GROUP_BOARD_COPPER_FRONT] : [];
        canvas.setHiddenGroups(hidden);
      },
    };
    // The visual diff (WS9-005): two SvgViews in the diff panel, mutually synced (a user
    // pan/zoom on one side is mirrored onto the other; setView fires no onViewChange, so the
    // mirroring cannot feed back). Each side pairs its canvas with a placeholder element for
    // "no sheet on this side" / render errors. reveal (WS9-006 click-to-locate) centers on
    // the side's overlay content and mirrors the resulting camera to the sibling, so both
    // panes land on the focused item.
    const diffSvgViewA = new SvgView(diffSvgA);
    const diffSvgViewB = new SvgView(diffSvgB);
    diffSvgViewA.show();
    diffSvgViewB.show();
    diffSvgViewA.onViewChange = (v) => diffSvgViewB.setView(v);
    diffSvgViewB.onViewChange = (v) => diffSvgViewA.setView(v);
    const diffSide = (view: SvgView, other: SvgView, ph: HTMLElement): DiffSideView => ({
      showSvg: (markup) => {
        ph.classList.remove("on");
        view.setSvg(markup);
      },
      setOverlay: (markup) => view.setOverlay(markup),
      setOverlays: (markups) => view.setOverlays(markups),
      showPlaceholder: (text) => {
        ph.textContent = text;
        ph.classList.add("on");
      },
      reveal: () => {
        const v = view.revealOverlay();
        if (v) other.setView(v);
      },
    });
    const diffSidesEl = document.getElementById("diff-sides");
    const diffLabelA = document.getElementById("diff-label-a");
    const diffView: DiffRenderView = {
      a: diffSide(diffSvgViewA, diffSvgViewB, diffPhA),
      b: diffSide(diffSvgViewB, diffSvgViewA, diffPhB),
      setBusy: (busy) => setBusyOverlay(busy, busy ? "comparing…" : undefined),
      // Overlay mode (WS9-007): hide the b pane (the a pane flexes full width and hosts the
      // union) and relabel — arrangement is view chrome, so it lives here, not the presenter.
      setOverlayMode: (on) => {
        diffSidesEl?.classList.toggle("overlay", on);
        if (diffLabelA) diffLabelA.textContent = on ? "A ∪ B — union" : "A — old";
      },
    };
    const diffPanel = diffPanelIsland(diffBarEl, this._eventBus, {
      onPair: (i) => void diffPresenter.selectPair(i),
      onMode: (m) => void diffPresenter.setMode(m),
      onClose: () => {
        diffPresenter.close();
        if (dockApi) closeDiffPanel(dockApi);
      },
    });
    // The changes panel (WS9-006) renders from the same DiffState push; clicking an item
    // focuses it (emphasis + locate) through the presenter.
    const diffChanges = diffChangesPanelIsland(diffChangesEl, this._eventBus, {
      onSelect: (id, pair) => void diffPresenter.selectItem(id, pair),
    });
    const diffPresenter = new DiffPresenter(diffClient(), designClient(), diffView, (s) => {
      diffPanel.view.setState(s);
      diffChanges.view.setState(s);
    });

    // Compare chrome (WS9-049 phase 3): the button opens a picker, and the picker reports a design
    // to compare against. openFile is the currently open design, kept here because it is side A of
    // whatever comparison the user starts.
    let openFile: { mount: string; path: string } | null = null;
    // comparePicker.onPick means "compare against this", not "set side B" — so when the presenter
    // layer grows to hold several comparisons at once, this callback is what changes, not the picker.
    const comparePick = comparePickerIsland(pickerEl, compareTreeEl, this._eventBus, (target) => {
      if (!openFile) return;
      if (dockApi) openDiffPanel(dockApi);
      void diffPresenter.open(openFile, target);
    });
    const compare = compareButton(compareEl, () => comparePick.picker.open(openFile));
    // WS9-049: the visited-sheet tab strip. It is a second SheetsView beside the tree, so it needs
    // no presenter change; selecting a tab is the same showSheet intent a tree sheet-click emits.
    const sheetTabs = sheetTabsIsland(sheetTabsEl, this._eventBus, {
      onSelect: (id) => void presenter.showSheet(id),
    });
    // The control bar (render-mode buttons + layout selector) is a Solid island: it renders from
    // the ControlsState the presenter pushes and emits mode/layout intents back up.
    const controls = controlBarIsland(controlsEl, this._eventBus, {
      onMode: (mode) => void presenter.setMode(mode),
      onLayout: (layout) => void presenter.setLayout(layout),
      onSymbols: (faithful) => void presenter.setSymbols(faithful),
      onBoardLayers: (side) => presenter.setBoardLayers(side),
    });
    // The merged checks panel lists rule findings for the loaded design (grouped/sorted client-side)
    // and hosts the on-demand Run button; clicking a finding highlights its subject (net/ref_des),
    // pressing Run evaluates the current rule selection (WS9).
    const findings = findingsPanelIsland(findingsEl, this._eventBus, {
      onSelect: (subject, sheet, netId) => void presenter.selectFinding(subject, sheet, netId),
      onRun: () => void presenter.runChecks(),
    });
    // The rules panel is the catalog of what the engine can assert; ticking rules sets the active
    // ruleset, which the presenter re-runs the checks over.
    const rules = rulesPanelIsland(rulesEl, this._eventBus, {
      onSelectionChange: (names) => void presenter.setRuleSelection(names),
    });
    // The sheet overview (WS9-025) is a birds-eye navigation surface: per-sheet violation
    // tiles, click to show that sheet.
    const sheetOverview = sheetOverviewPanelIsland(sheetOverviewEl, this._eventBus, {
      onSelect: (sheetId) => void presenter.showSheet(sheetId),
    });
    // The datalog query panel (WS9-036): the user runs an ad-hoc query, the presenter evaluates
    // it over the open design and pushes results back through query.view.
    const query = queryPanelIsland(queryEl, this._eventBus, {
      onRun: (text) => void presenter.runQuery(text),
      onLocate: (kind, subject, sheet, reason) => void presenter.locateEntity(kind, subject, sheet, reason),
    });
    // A click on the drawing is a question about what was clicked: highlight it, write the query that
    // asks what is known about it, and bring the Query panel forward if it is a background tab. The
    // generated query is left editable on purpose — using the viewer is how a reader learns the
    // language, rather than the language being a wall in front of the answers.
    svgView.onPick = (sel) => {
      void presenter.locateEntity(sel.kind, sel.ref ?? sel.net ?? sel.busId ?? "", undefined, undefined, sel.pin);
      query.view.setQuery(queryFor(sel));
      if (dockApi) dockApi.getPanel("query")?.api.setActive();
    };

    // The interface-coverage panel (WS9-041): clicking a signal locates its net, the same locate
    // path the query panel uses.
    const coverage = coveragePanelIsland(coverageEl, this._eventBus, {
      onLocate: (net) => void presenter.locateEntity("net", net),
    });
    // The datasheet-params panel (WS9-035): clicking a component locates it on the canvas, the same
    // component-highlight path a finding uses (zero new highlight code).
    const parts = partsPanelIsland(partsEl, this._eventBus, {
      onLocate: (refDes) => void presenter.locateEntity("component", refDes),
    });
    // The naming-vocabulary bar (WS9-128): choosing a convention re-runs everything under it, since
    // a request convention replaces the server's rather than adding to it.
    const conventionBar = conventionBarIsland(conventionEl, this._eventBus, {
      onSelect: (ref) => void presenter.setConvention(ref),
    });
    // The project bar (agni issue 175): which project's config produced what is on screen, and the
    // opt-out that re-runs the design under the built-in catalog so the difference is visible.
    const projectBar = projectBarIsland(projectEl, this._eventBus, {
      onPlain: (plain) => void presenter.setPlainCatalog(plain),
    });
    // The review panel (WS9-052): the project's checklist verdict over the stored runs. Locating a
    // finding under an item reuses the same locateEntity path every other panel uses, so a review
    // finding highlights exactly the way a check finding does.
    const review = reviewPanelIsland(reviewEl, this._eventBus, {
      onSelectRun: (name) => presenter.showReview(name),
      onSelectChecklist: (ref) => presenter.setChecklist(ref),
      onCreate: () => void presenter.createReview(),
      onLocate: (kind, subject) => void presenter.locateEntity(kind, subject),
    });
    // The relation catalog (WS9-037) is static per build and design-independent, so fetch it once
    // at startup and push it to the panel's picker; a failure just leaves the picker empty (the
    // panel falls back to the syntax hint).
    void queryClient()
      .listRelations({})
      .then((r) => {
        query.view.setRelations(r.relations);
        query.view.setExamples(r.examples); // WS14-002: starter queries beside the relation picker
      })
      .catch(() => {});
    // The presenter fans sheet state to every surface in sheetNavs. The file tree used to be one of
    // them (sheets nested under their file); with the tree gone from this page the fan-out feeds the
    // top tab strip, and the Sheets overview panel takes its own `overview` channel. The array stays
    // a fan-out because that is what let the strip join in phase 1 with no presenter change.
    const presenter = new ViewerPresenter(
      designClient(),
      checksClient(),
      canvas,
      renderView,
      {
        sheetNavs: [sheetTabs.view],
        summary: setSummary,
        controls: controls.view,
        findings: findings.view,
        expectationCaption: setExpectCaption,
        rules: rules.view,
        report: setReport,
        // Every location report also feeds the Compare chrome: the open design is side A of any
        // comparison the user starts, and until one is open there is nothing to compare against.
        location: (loc) => {
          if (hasFile(loc)) {
            openFile = { mount: loc.mount, path: loc.path };
            compare.setEnabled(true);
            // The landing page's Recent list is written HERE rather than at the click that opened
            // the design, so a deep link and a back/forward restore count as openings too: what the
            // list is for is "where was I", and arriving by URL is arriving.
            noteOpen({ kind: "design", mount: loc.mount, path: loc.path, label: baseName(loc.path) });
          }
          syncUrl(loc);
        },
        overview: sheetOverview.view,
        query: query.view,
        coverage: coverage.view,
        review: review.view,
        conventionBar: conventionBar.view,
        projectBar: projectBar.view,
        parts: parts.view,
      },
      queryClient(),
      reviewClient(),
      workspaceClient(),
      projectClient(),
    );
    this.presenter = presenter;

    // WS9-044: the highlight-style dropdown. Apply any saved style at boot, then update the
    // presenter (and persist) whenever the user edits color / opacity / width. It is top-bar
    // chrome, so it lives here, not in an island.
    const highlightMenuEl = document.getElementById("highlight-menu");
    if (highlightMenuEl) {
      const saved = loadHighlightStyle(window.localStorage);
      if (saved) presenter.setHighlightStyle(saved);
      highlightMenu(highlightMenuEl, window.localStorage, (style) => presenter.setHighlightStyle(style));
    }

    children.push(
      canvas,
      comparePick.island,
      sheetTabs.island,
      controls.island,
      findings.island,
      rules.island,
      diffPanel.island,
      diffChanges.island,
      sheetOverview.island,
      query.island,
      coverage.island,
      review.island,
      conventionBar.island,
      projectBar.island,
      parts.island,
    );
    return children;
  }
}

function setSummary(text: string): void {
  const el = document.getElementById("design-summary");
  if (el) el.textContent = text;
}

// setReport renders the auto-layout conversion report into the detail panel: a per-label count
// line, then call-outs for the unmapped (box) and unresolved components, the latter pointing at
// --symbol-path. A null/empty report hides the panel (e.g. the faithful layout).
function setReport(report: { components: { refDes: string; deviceClass: string; kind: string }[] } | null): void {
  const el = document.getElementById("conversion-report");
  if (!el) return;
  el.replaceChildren();
  if (!report || report.components.length === 0) {
    el.style.display = "none";
    return;
  }
  el.style.display = "";

  // Group ref-des by label: the device class for a glyph, else the kind (provided/box/unresolved).
  const byLabel = new Map<string, string[]>();
  for (const c of report.components) {
    const label = c.kind === "glyph" ? c.deviceClass : c.kind;
    (byLabel.get(label) ?? byLabel.set(label, []).get(label)!).push(c.refDes);
  }

  const heading = document.createElement("h4");
  heading.textContent = "Conversion";
  el.appendChild(heading);

  const counts = document.createElement("div");
  counts.className = "report-counts";
  counts.textContent = [...byLabel.entries()]
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([label, refs]) => `${label} ${refs.length}`)
    .join(" · ");
  el.appendChild(counts);

  const callout = (cls: string, text: string) => {
    const d = document.createElement("div");
    d.className = cls;
    d.textContent = text;
    el.appendChild(d);
  };
  const box = byLabel.get("box");
  if (box) callout("report-box", `${box.length} unmapped (no device glyph): ${box.join(" ")}`);
  const un = byLabel.get("unresolved");
  if (un) callout("report-unresolved", `${un.length} unresolved — pass --symbol-path: ${un.join(" ")}`);
}

// Boot the dock shell before the island lifecycle: the dock adopts the server-rendered
// holes into its panels first, so islands initialize inside laid-out (measurable) panels.
// Islands in a closed panel still mount — their hole just stays parked and hidden.
const dockEl = document.getElementById("dock");
const parkEl = document.getElementById("panel-park");
const menuEl = document.getElementById("panels-menu");
// dockApi is kept so entering a comparison can open/focus the Diff panel (WS9-005).
const dockApi = dockEl && parkEl && menuEl ? createViewerDock(dockEl, parkEl, menuEl, window.localStorage) : null;

const bus = new EventBus();
const controller = new LifecycleController(bus);
const root = new AppRoot("app", document.body, bus);
void controller
  .initializeFromRoot(root)
  .then(async () => {
    const presenter = root.presenter;
    if (!presenter) return;
    // applyUrl opens whatever the current URL addresses. The restoring flag keeps this replay from
    // pushing a duplicate history entry (see syncUrl); it runs once at boot (deep-link refresh) and
    // again on every popstate (browser back/forward).
    const applyUrl = async (): Promise<void> => {
      const loc = currentLocation();
      // A folder location cannot reach this page: the server routes a folder URL to the browse page
      // and only a /view URL here (WS9-049 phase 2), and syncUrl only ever pushes file locations. So
      // there is no dir branch to handle — the tree that used to expand to one is gone.
      if (!hasFile(loc)) return; // not a design URL — leave the empty shell as-is
      restoring = true;
      try {
        await presenter.restore(loc);
      } finally {
        restoring = false;
      }
    };
    window.addEventListener("popstate", () => void applyUrl());
    await applyUrl();
  })
  .catch((err) => {
    console.error(err);
    const readout = document.getElementById("readout");
    if (readout) readout.textContent = `error: ${String(err)}`;
  });

// Entry point for the viewer page (/designs/<mount>/<path>/view). The page is server-rendered by
// goapplib/templar. This boots the dockview shell first, then the tsappkit lifecycle. AppRoot builds
// the canvas, the panel islands and the ViewerPresenter that feeds them, and the LifecycleController
// initializes each island. The page has no file tree (WS9-049).

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
import { tracePanelIsland } from "./tracepanel.js";
import { reviewPanelIsland } from "./reviewpanel.js";
import { conventionBarIsland } from "./conventionbar.js";
import { projectBarIsland } from "./projectbar.js";
import { partsPanelIsland } from "./partspanel.js";
import { ViewerPresenter, type RenderView, type SavedFile } from "./viewer.js";
import { DiffPresenter, type DiffRenderView, type DiffSideView } from "./diffpresenter.js";
import { SvgView } from "./svgview.js";
import { compareButton } from "./compare.js";
import { designClient, checksClient, diffClient, queryClient, reviewClient, workspaceClient,
  projectClient, useEngineFetch,
} from "./api.js";
import { startEngine, mountingFetch, mountOf, type DesignFiles, type WasmEngine } from "./wasm/client.js";
import { BROWSER_MOUNT, installDrop } from "./wasm/drop.js";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { createViewerDock, openDiffPanel, closeDiffPanel } from "./dock.js";
import { highlightMenu, loadHighlightStyle } from "./highlightstyle.js";
import { appBase, currentLocation, hasFile, locationToUrl, setBase, setStaticHost, type ViewerLocation } from "./router.js";
import { GROUP_BOARD_COPPER_BACK, GROUP_BOARD_COPPER_FRONT } from "./packed.js";
import { LocateReason } from "./query.js";
import { delayedBusy } from "./busy.js";
import { expectationCaptionStrip } from "./expectcaption.js";
import { undrawnStrip } from "./undrawn.js";
import { staleLinkStrip } from "./stalelink.js";
import { entityBindings } from "./selection.js";
import { baseName, noteOpen } from "@agni/web-shared/recents.js";

// restoring is true while a URL is being replayed into the presenter (initial load or back/forward).
// The presenter's location callback still fires then, and must not push a history entry for it.
// Module-level because both the presenter callback and the restore driver below read it.
let restoring = false;

// isTextEntry reports whether an event landed in somewhere the reader is typing, so a page-level key
// binding can decline it. The query box and the review notes both take free text, and Escape inside
// them belongs to the field.
function isTextEntry(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el || !el.tagName) return false;
  const tag = el.tagName.toLowerCase();
  return tag === "input" || tag === "textarea" || tag === "select" || el.isContentEditable === true;
}

// syncUrl reflects the open design's location into the address bar. It always sets the tab title,
// and pushes history only when the URL changed and no restore is running, so navigation builds a
// back stack and a refresh or back/forward replay does not.
function syncUrl(loc: ViewerLocation): void {
  document.title = hasFile(loc) ? `${loc.path || loc.mount} — Agni` : "Agni viewer";
  if (restoring) return;
  const url = withEngineParams(locationToUrl(loc));
  if (url !== window.location.pathname + window.location.search) window.history.pushState(null, "", url);
}

// engineParams are the query parameters that choose this page's engine. They ride along on every
// URL the page pushes, so a refresh or a back/forward keeps the engine the visitor asked for.
const engineParams = (() => {
  const q = new URLSearchParams(window.location.search);
  const out = new URLSearchParams();
  for (const k of ["engine", "wasm-max-bytes"]) {
    const v = q.get(k);
    if (v !== null) out.set(k, v);
  }
  return out;
})();

function withEngineParams(url: string): string {
  if (!engineParams.size) return url;
  const u = new URL(url, window.location.origin);
  engineParams.forEach((v, k) => u.searchParams.set(k, v));
  return u.pathname + u.search;
}

// pageEngine is the engine this page analyses its design on: the URL's `engine=`, else the one the
// server wrote into the shell (`agni serve --engine`), else the server.
function pageEngine(): string {
  // A page served as plain files has no server to analyse anything, so it runs in the browser.
  if (onStaticHost) return "wasm";
  return engineParams.get("engine") ?? document.querySelector<HTMLElement>(".app")?.dataset.engine ?? "server";
}

// The shell says where the app is served and whether an agni server is behind it (agni issue 856).
// Both are read before anything builds or reads a URL.
const shell = document.querySelector<HTMLElement>(".app")?.dataset ?? {};
const onStaticHost = shell.host === "static";
setBase(shell.base || "/");
setStaticHost(onStaticHost);

// fileURL is where one of a mount's files is fetched from: the server's read-only /raw/ route, or
// the same path under the static site's raw/ folder, which `agni site` writes with that layout.
function fileURL(mount: string, path: string): URL {
  return new URL(`${appBase()}raw/${encodeURIComponent(mount)}/${path.split("/").map(encodeURIComponent).join("/")}`, location.href);
}

// wasmMaxBytes is the largest design the page analyses in the browser: the URL's `wasm-max-bytes=`,
// else the server's `--wasm-max-bytes` from the shell (agni issue 852).
function wasmMaxBytes(): number {
  const v = engineParams.get("wasm-max-bytes") ?? document.querySelector<HTMLElement>(".app")?.dataset.wasmMaxBytes;
  const n = Number(v);
  return v && Number.isFinite(n) && n > 0 ? n : Number.POSITIVE_INFINITY;
}

// showEngineNote says, in the top bar, that a design left the engine the page asked for.
function showEngineNote(text: string): void {
  const el = document.getElementById("engine-note");
  if (!el) return;
  el.textContent = text;
  el.hidden = false;
}

// refuseBrowserMounts answers, on the server engine, every request naming a dropped design with a
// refusal and a note rather than sending it, since the server has never seen those files (agni issue
// 854). Requests about the server's own mounts pass.
function refuseBrowserMounts(): void {
  const reason = "A dropped design is analysed in the browser and never sent to the server, so it needs the browser engine";
  const network = globalThis.fetch.bind(globalThis);
  if (currentLocation().mount === BROWSER_MOUNT) showEngineNote(`${reason}.`);
  useEngineFetch(async (input, init) => {
    const req = new Request(input, init);
    const body = await req.clone().text();
    if (body.includes(`"mount://${BROWSER_MOUNT}/`) || body.includes(`"mount://${BROWSER_MOUNT}"`)) {
      showEngineNote(`${reason}.`);
      return new Response(JSON.stringify({ code: "failed_precondition", message: reason }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      });
    }
    return network(req);
  });
}

// installPageDrop wires dropping files on the page (agni issue 854). On the server engine a drop is
// refused with the reason, so a visitor's files never leave the browser by accident.
function installPageDrop(engine: WasmEngine | undefined): void {
  const dialog = document.getElementById("drop-dialog") as HTMLDialogElement | null;
  const button = document.getElementById("drop-open");
  const input = document.getElementById("drop-input") as HTMLInputElement | null;
  if (!dialog || !button || !input) return;
  const folderButton = document.getElementById("drop-folder") ?? undefined;
  const folderInput = (document.getElementById("drop-folder-input") as HTMLInputElement | null) ?? undefined;
  const files = workspaceClient();
  installDrop({
    engine,
    propose: (uri) => files.proposeDesigns({ uri }),
    open: (path) => {
      const segs = [BROWSER_MOUNT, ...path.split("/")].map(encodeURIComponent).join("/");
      window.history.pushState(null, "", withEngineParams(`${appBase()}designs/${segs}/view`));
      window.dispatchEvent(new PopStateEvent("popstate"));
    },
    refuse: (reason) => showEngineNote(`${reason}.`),
    target: document.body,
    dialog,
    button,
    input,
    folderButton,
    folderInput,
  });
}

// SERVE_LANE_MAX_BYTES is the serve worker's memory limit (agni issue 911). A restored Jetson AGX Thor
// baseboard holds about 320 MB and a fresh read of it about 390 MB, so one open of the largest seeded
// board never trips it, and a worker that has crept past it across several designs is replaced by one
// that restores the open design from the browser's cache. A tighter limit would restart a worker
// whose restored design alone is over it after every request.
const SERVE_LANE_MAX_BYTES = 512 << 20;

// startPageEngine puts the page on the in-browser engine when it runs on wasm (agni issue 853): the engine
// loads in a worker, and every client the page builds afterwards talks to it, bringing in each
// design's files from the server the first time a request names it. The design keeps the URL it has
// under the server engine. Listings and file reads go to the server, which owns the mount namespace.
// It must finish before the app root builds its clients. The assets sit beside this bundle, so the
// page works under any path prefix.
async function startPageEngine(): Promise<WasmEngine | undefined> {
  if (pageEngine() !== "wasm") {
    refuseBrowserMounts();
    return undefined;
  }
  const asset = (name: string) => new URL(name, import.meta.url).href;
  const engine = await startEngine(
    { worker: asset("agni-worker.js"), wasm: asset("agni.wasm"), exec: asset("wasm_exec.js") },
    {
      serveMaxBytes: SERVE_LANE_MAX_BYTES,
      onRestart: (lane, bytes) => console.info(`agni: the ${lane} worker held ${Math.round(bytes / (1 << 20))} MB and was replaced`),
    },
  );
  const network = globalThis.fetch.bind(globalThis);
  const files = workspaceClient();
  const workspacePath = `/${WorkspaceService.typeName}/`;
  useEngineFetch(
    mountingFetch({
      engine,
      source: {
        // A static site lists each mount whole, in files/<mount>.json beside its raw/ folder, since a
        // seeded mount is one design and there is no server to ask.
        list: onStaticHost
          ? async (uri) => {
              const res = await network(new URL(`${appBase()}files/${encodeURIComponent(mountOf(uri))}.json`, location.href));
              if (!res.ok) throw new Error(`${mountOf(uri)}: ${res.status}`);
              return (await res.json()) as DesignFiles;
            }
          : (uri) => files.listDesignFiles({ uri }),
        fetchFile: async (mount, path) => {
          const res = await network(fileURL(mount, path));
          if (!res.ok) throw new Error(`${mount}/${path}: ${res.status}`);
          return new Uint8Array(await res.arrayBuffer());
        },
      },
      network,
      // With no server, workspace calls are the worker's too.
      passthrough: (url) => !onStaticHost && url.pathname.startsWith(workspacePath),
      browserMounts: new Set([BROWSER_MOUNT]),
      maxBytes: wasmMaxBytes(),
      onServer: (f) => showEngineNote(`Analysed on the server: ${f.reason}.`),
    }),
  );
  return engine;
}

class AppRoot extends BaseComponent {
  // presenter is exposed so the boot code can drive a deep-link restore once the islands are
  // initialized. performLocalInit creates it.
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
    const traceEl = document.getElementById("trace-panel");
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
    if (!traceEl) return children;
    if (!projectEl) return children;

    const busyEl = document.getElementById("render-busy");
    // One app-level loader overlay drives both the viewer and the diff (they never run at once);
    // the shared delayedBusy keeps a single show-timer for it (WS7-043/044).
    const setBusyOverlay = delayedBusy(busyEl);
    // WS9-045: the conformance expectation verdict strip (a plain-DOM sink, like the busy overlay;
    // hidden on any design without a sidecar).
    const setExpectCaption = expectationCaptionStrip(document.getElementById("expect-caption"));
    const setUndrawnNote = undrawnStrip(document.getElementById("undrawn-note"));
    const setStaleLinkNote = staleLinkStrip(document.getElementById("stale-link-note"));
    const readoutEl = document.getElementById("readout");
    const svgView = new SvgView(svgEl);
    // Clicking the drawing selects what is under the cursor and asks the query engine about it. The
    // wiring is deferred to the end of this method because it needs the query panel, which is built
    // below; see the assignment after the panels.
    const canvas = new CanvasComponent("canvas", canvasEl, this._eventBus);
    // RenderView reveals whichever renderer drew the sheet. The SVG host overlays the canvas, so
    // showWebgl just hides it and showSvg fills and shows it.
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
        // The readout stays visible in SVG mode, with the element count (see SvgView.stats).
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
      // Board layer visibility (WS7-034/035) toggles CSS classes over BoardSVG's classed strata
      // and hides packed groups on the WebGL canvas (the packed board's back/front strata).
      setBoardLayers: (side) => {
        svgEl.classList.remove("board-front", "board-back");
        if (side === "front" || side === "back") svgEl.classList.add(`board-${side}`);
        const hidden = side === "front" ? [GROUP_BOARD_COPPER_BACK] : side === "back" ? [GROUP_BOARD_COPPER_FRONT] : [];
        canvas.setHiddenGroups(hidden);
      },
    };
    // The visual diff (WS9-005) is two SvgViews that mirror each other's pan and zoom. setView fires
    // no onViewChange, so the mirroring cannot loop. Each side pairs its view with a placeholder for
    // "no sheet on this side" and render errors. reveal (WS9-006) centers on the side's overlay and
    // copies the camera to the sibling, so both panes land on the focused item.
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
      // Overlay mode (WS9-007) hides the b pane, so the a pane flexes to full width and hosts the
      // union. Arrangement is view chrome, so it lives here and not in the presenter.
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

    // Compare chrome (WS9-049 phase 3). The button opens a picker, and the picker reports a design to
    // compare against. openFile is the open design, side A of any comparison the user starts.
    let openFile: { mount: string; path: string } | null = null;
    // The pick callback means "compare against this", not "set side B".
    const comparePick = comparePickerIsland(pickerEl, compareTreeEl, this._eventBus, (target) => {
      if (!openFile) return;
      if (dockApi) openDiffPanel(dockApi);
      void diffPresenter.open(openFile, target);
    });
    const compare = compareButton(compareEl, () => comparePick.picker.open(openFile));
    // WS9-049: the visited-sheet tab strip, the only SheetsView in sheetNavs. Selecting a tab emits
    // the same showSheet intent the sheet overview does.
    const sheetTabs = sheetTabsIsland(sheetTabsEl, this._eventBus, {
      onSelect: (id) => void presenter.showSheet(id),
    });
    // The control bar (render-mode buttons + layout selector) is a Solid island that renders from
    // the ControlsState the presenter pushes and emits mode/layout intents back up.
    const controls = controlBarIsland(controlsEl, this._eventBus, {
      onMode: (mode) => void presenter.setMode(mode),
      onLayout: (layout) => void presenter.setLayout(layout),
      onSymbols: (faithful) => void presenter.setSymbols(faithful),
      onBoardLayers: (side) => presenter.setBoardLayers(side),
      onClearHighlights: () => void presenter.clearHighlights(),
    }, { staticHost: onStaticHost });
    // Escape clears the highlight. It lives here and not in pagegestures, which routes only the
    // datasheet workbench's keys and never sees this canvas (agni issue 348). An open picker owns
    // Escape, and so does a text field the reader is typing in.
    document.addEventListener("keydown", (e) => {
      if (e.key !== "Escape") return;
      if (comparePick.picker.isOpen()) return;
      if (isTextEntry(e.target)) return;
      void presenter.clearHighlights();
    });
    // The merged checks panel lists rule findings for the loaded design (grouped/sorted client-side)
    // and hosts the on-demand Run button; clicking a finding highlights its subject (net/ref_des),
    // pressing Run evaluates the current rule selection (WS9).
    const findings = findingsPanelIsland(findingsEl, this._eventBus, {
      onSelect: (subject, sheet, netId) => void presenter.selectFinding(subject, sheet, netId),
      // A context entity is not a finding, so it locates through the same path a query result cell
      // does (locateEntity) rather than through selectFinding, whose lookup is by finding subject
      // and would find nothing (agni issue 349).
      onLocateContext: (kind, subject, pin) => void presenter.locateEntity(kind, subject, undefined, LocateReason.UNSPECIFIED, pin),
      onRun: () => void presenter.runChecks(),
      // A verdict is addressed by its derived id, not by subject, because a passing verdict has no
      // finding for selectFinding to look up and two rules can hold verdicts about one subject.
      onSelectVerdict: (id) => void presenter.locateVerdict(id),
      onSave: (format) => void presenter.saveReport(format),
    });
    // The rules panel is the catalog of what the engine can assert; ticking rules sets the active
    // ruleset, which the presenter re-runs the checks over.
    const rules = rulesPanelIsland(rulesEl, this._eventBus, {
      onSelectionChange: (names) => void presenter.setRuleSelection(names),
    });
    // The sheet overview (WS9-025) shows per-sheet violation tiles, and clicking one shows that
    // sheet.
    const sheetOverview = sheetOverviewPanelIsland(sheetOverviewEl, this._eventBus, {
      onSelect: (sheetId) => void presenter.showSheet(sheetId),
    });
    // The datalog query panel (WS9-036) runs an ad-hoc query. The presenter evaluates
    // it over the open design and pushes results back through query.view.
    const query = queryPanelIsland(queryEl, this._eventBus, {
      onRun: (text, bindings) => void presenter.runQuery(text, bindings),
      onLocate: (kind, subject, sheet, reason, pin) => void presenter.locateEntity(kind, subject, sheet, reason, pin ?? ""),
      // onInspect opens the check results for the selected entity through the EXISTING finding focus
      // (agni issue 259). Nothing re-evaluates, since selectFinding focuses a subject within the
      // results already computed. The identity argument mirrors selectionFromFinding, since the two
      // have to agree on which instance of a repeated net name is meant.
      onInspect: (sel) =>
        void presenter.selectFinding(
          sel.kind === "bus" ? (sel.busId ?? "") : sel.kind === "net" ? (sel.net ?? "") : (sel.ref ?? ""),
          undefined,
          sel.kind === "bus" ? (sel.busId ?? "") : (sel.netId ?? ""),
        ),
    });
    // A click on the drawing highlights what was clicked, writes the query that asks what is known
    // about it, and brings the Query panel forward if it is a background tab. The generated query
    // stays editable, so a reader learns the query language by using the viewer.
    svgView.onPick = (sel) => {
      void presenter.locateEntity(sel.kind, sel.ref ?? sel.net ?? sel.busId ?? "", undefined, undefined, sel.pin);
      // The panel names the pick, so the same bar carries the next question. A click on a result
      // cell replaces the name with whatever it landed on.
      query.view.setSelection(sel);
      // The preset comes from the server (query.EntityQueries), so the query text is checked where
      // the relations it names are defined. Before the catalog arrives there is no preset, and a
      // click then highlights and asks nothing rather than running a guess.
      const preset = query.view.entityQuery(sel.kind);
      if (preset) query.view.setQuery(preset.query, entityBindings(preset.binds, sel));
      if (dockApi) dockApi.getPanel("query")?.api.setActive();
    };

    // In the interface-coverage panel (WS9-041), clicking a signal locates its net through the same
    // path the query panel uses.
    const coverage = coveragePanelIsland(coverageEl, this._eventBus, {
      onLocate: (net) => void presenter.locateEntity("net", net),
    });
    // In the trace panel (agni issue 600) the reader names two pins, and the presenter walks between them
    // and lights the route through the same highlight stack a query cell and a verdict use.
    const trace = tracePanelIsland(traceEl, this._eventBus, {
      onTrace: (from, to) => void presenter.runTrace(from, to),
    });
    // In the datasheet-params panel (WS9-035), clicking a component locates it on the canvas through
    // the same component-highlight path a finding uses.
    const parts = partsPanelIsland(partsEl, this._eventBus, {
      onLocate: (refDes) => void presenter.locateEntity("component", refDes),
    }, { staticHost: onStaticHost });
    // In the naming-vocabulary bar (WS9-128), choosing a convention re-runs everything under it, since
    // a request convention replaces the server's rather than adding to it.
    const conventionBar = conventionBarIsland(conventionEl, this._eventBus, {
      onSelect: (ref) => void presenter.setConvention(ref),
    }, { staticHost: onStaticHost });
    // The project bar (agni issue 175) names the project whose config produced what is on screen, and
    // offers an opt-out that re-runs the design under the built-in catalog so the difference shows.
    const projectBar = projectBarIsland(projectEl, this._eventBus, {
      onPlain: (plain) => void presenter.setPlainCatalog(plain),
    });
    // The review panel (WS9-052) shows the project's checklist verdict over the stored runs. A
    // finding under an item locates through the same locateEntity path as every other panel.
    const review = reviewPanelIsland(reviewEl, this._eventBus, {
      onSelectRun: (name) => presenter.showReview(name),
      onSelectChecklist: (ref) => presenter.setChecklist(ref),
      onCreate: () => void presenter.createReview(),
      onLocate: (kind, subject) => void presenter.locateEntity(kind, subject),
    }, { staticHost: onStaticHost });
    // The relation catalog (WS9-037) is static per build and design-independent, so fetch it once
    // at startup and push it to the panel's picker; a failure just leaves the picker empty (the
    // panel falls back to the syntax hint).
    void queryClient()
      .listRelations({})
      .then((r) => {
        query.view.setRelations(r.relations);
        query.view.setExamples(r.examples); // WS14-002: starter queries beside the relation picker
        query.view.setEntityQueries(r.entityQueries); // the click-to-ask presets
        // The find-by-name template (agni issue 338). A server that sends none leaves the panel
        // with no search mode, since the template names relations only the server defines.
        const sq = r.searchQuery;
        query.view.setSearch(sq ? { query: sq.query, teaches: sq.teaches, bind: sq.bind, pattern: sq.pattern } : null);
      })
      .catch(() => {});
    // The presenter fans sheet state to every surface in sheetNavs, here only the top tab strip
    // (the file tree left this page in WS9-049). The Sheets overview panel takes its own `overview`
    // channel.
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
        undrawnNote: setUndrawnNote,
        staleLinkNote: setStaleLinkNote,
        rules: rules.view,
        report: setReport,
        // A saved report goes to the visitor's machine as a download, and its rows link back to this
        // viewer under the page's own base (agni issue 127).
        download: saveFile,
        linkBase: new URL(appBase(), location.href).href,
        // Every location report also feeds the Compare chrome, since the open design is side A of any
        // comparison and Compare stays disabled until one is open.
        location: (loc) => {
          if (hasFile(loc)) {
            openFile = { mount: loc.mount, path: loc.path };
            compare.setEnabled(true);
            // The landing page's Recent list is written HERE rather than at the click that opened
            // the design, so a deep link and a back/forward restore count as openings too.
            noteOpen({ kind: "design", mount: loc.mount, path: loc.path, label: baseName(loc.path) });
          }
          syncUrl(loc);
        },
        overview: sheetOverview.view,
        query: query.view,
        coverage: coverage.view,
        trace: trace.view,
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
      trace.island,
      review.island,
      conventionBar.island,
      projectBar.island,
      parts.island,
    );
    return children;
  }
}

// saveFile hands a file the presenter wrote to the browser as a download (agni issue 127). It goes
// through an object URL rather than a data: URL, because a report on a large board runs to megabytes.
function saveFile(file: SavedFile): void {
  const url = URL.createObjectURL(new Blob([file.content as BlobPart], { type: file.contentType }));
  const a = document.createElement("a");
  a.href = url;
  a.download = file.filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Revoked on the next turn, since a click starts the download asynchronously in some browsers.
  setTimeout(() => URL.revokeObjectURL(url), 0);
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

// Boot the dock shell before the island lifecycle. The dock adopts the server-rendered holes
// into its panels first, so islands initialize inside laid-out (measurable) panels.
// Islands in a closed panel still mount, and their hole stays parked and hidden.
const dockEl = document.getElementById("dock");
const parkEl = document.getElementById("panel-park");
const menuEl = document.getElementById("panels-menu");
// dockApi is kept so entering a comparison can open/focus the Diff panel (WS9-005).
const dockApi = dockEl && parkEl && menuEl ? createViewerDock(dockEl, parkEl, menuEl, window.localStorage) : null;

const bus = new EventBus();
const controller = new LifecycleController(bus);
void startPageEngine()
  .then(async (engine) => {
    const root = new AppRoot("app", document.body, bus);
    await controller.initializeFromRoot(root);
    return { root, engine };
  })
  .then(async ({ root, engine }) => {
    const presenter = root.presenter;
    if (!presenter) return;
    installPageDrop(engine);
    const here = currentLocation();
    if (engine && here.mount === BROWSER_MOUNT && hasFile(here)) {
      // A reload empties the worker, and a dropped design existed nowhere else.
      showEngineNote("Dropped files live only in the tab they were dropped into, and a reload empties it: drop them again.");
    }
    // applyUrl opens whatever the current URL addresses. The restoring flag keeps this replay from
    // pushing a duplicate history entry (see syncUrl); it runs once at boot (deep-link refresh) and
    // again on every popstate (browser back/forward).
    const applyUrl = async (): Promise<void> => {
      const loc = currentLocation();
      // A folder location cannot reach this page. The server routes a folder URL to the browse page
      // and only a /view URL here (WS9-049 phase 2), and syncUrl only pushes file locations.
      if (!hasFile(loc)) return; // not a design URL, so leave the empty shell as-is
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

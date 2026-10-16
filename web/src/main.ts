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
import { tablesPanelIsland } from "./tablespanel.js";
import { TablesPresenter, type TablesSourceKey } from "./tables.js";
import { reviewPanelIsland } from "./reviewpanel.js";
import { conventionBarIsland } from "./conventionbar.js";
import { projectBarIsland } from "./projectbar.js";
import { partsPanelIsland } from "./partspanel.js";
import { ViewerPresenter, type RenderView, type SavedFile } from "./viewer.js";
import { DiffPresenter, type DiffRenderView, type DiffSideView } from "./diffpresenter.js";
import { SvgView, drawnOnSheet } from "./svgview.js";
import { compareButton } from "./compare.js";
import { designClient, checksClient, diffClient, queryClient, reviewClient, workspaceClient,
  projectClient, useEngineFetch,
  tableClient, timingMode,
} from "./api.js";
import { artifactUri } from "@agni/web-shared/uri.js";
import { startEngine, mountingFetch, mountOf, type DesignFiles, type WasmEngine } from "./wasm/client.js";
import { fetchFile } from "./wasm/fetchfile.js";
import { BROWSER_MOUNT, installDrop } from "./wasm/drop.js";
import { WorkspaceService } from "./gen/agni/v1/webapi/workspace_pb.js";
import { createViewerDock, openDiffPanel, closeDiffPanel } from "./dock.js";
import { highlightMenu, loadHighlightStyle } from "./highlightstyle.js";
import { appBase, currentLocation, hasFile, locationToUrl, setBase, setStaticHost, type ViewerLocation } from "./router.js";
import { GROUP_BOARD_COPPER_BACK, GROUP_BOARD_COPPER_FRONT } from "./packed.js";
import { LocateReason, drawnSheets, searchPattern } from "./query.js";
import { delayedBusy } from "./busy.js";
import { expectationCaptionStrip } from "./expectcaption.js";
import { undrawnStrip } from "./undrawn.js";
import { staleLinkStrip } from "./stalelink.js";
import { selectionFromCell, type Selection } from "./selection.js";
import { propertiesPanelIsland } from "./propertiespanel.js";
import { PropertiesPresenter } from "./propertiespresenter.js";
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

// hideEngineNote clears the top bar's note when it is still about `about`, such as a file whose
// stalled fetch went through on a later attempt.
function hideEngineNote(about: string): void {
  const el = document.getElementById("engine-note");
  if (el?.textContent?.includes(about)) el.hidden = true;
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

// SERVE_LANE_MAX_BYTES is the serve worker's memory limit (agni issues 911, 974, 990). A worker that has
// crept past it is replaced, when its live heap says a replacement would come back smaller
// (wouldShrink in wasm/client.ts), by one that restores the open design from the browser's cache. A
// worker that is past it only because the open design is that large is kept.
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
      onRestart: (lane, s) =>
        console.info(
          `agni: the ${lane} worker held ${Math.round(s.memoryBytes / (1 << 20))} MB, ${Math.round((s.heap?.heapAlloc ?? 0) / (1 << 20))} MB of it live, and was replaced`,
        ),
    },
  );
  const network = globalThis.fetch.bind(globalThis);
  const timed = timingMode() !== "";
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
        // A transfer that stops sending is abandoned and asked for again, and the top bar says which
        // file is slow, rather than the page waiting on it forever (agni issue 1002).
        fetchFile: (mount, path) =>
          fetchFile(network, fileURL(mount, path), {
            onStall: (s) =>
              showEngineNote(
                `Fetching ${path} stalled after ${Math.round(s.bytes / (1 << 20))} MB; ${s.attempt < s.attempts ? `asking again (${s.attempt + 1} of ${s.attempts})` : "giving up"}.`,
              ),
            onDone: (d) => {
              if (d.attempts > 1) hideEngineNote(path);
              if (timed) console.info(`agni fetch ${mount}/${path}: ${JSON.stringify({ bytes: d.bytes, ms: Math.round(d.ms), attempts: d.attempts })}`);
            },
          }),
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
    const tablesEl = document.getElementById("tables-panel");
    const reviewEl = document.getElementById("review-panel");
    const conventionEl = document.getElementById("convention-bar");
    const projectEl = document.getElementById("project-bar");
    const partsEl = document.getElementById("parts-panel");
    const propertiesEl = document.getElementById("properties-panel");
    const sheetTabsEl = document.getElementById("sheet-tabs");
    if (!canvasEl || !pickerEl || !compareTreeEl || !svgEl || !controlsEl || !findingsEl || !rulesEl || !sheetTabsEl)
      return children;
    if (!compareEl || !diffBarEl || !diffSvgA || !diffSvgB || !diffPhA || !diffPhB || !diffChangesEl)
      return children;
    if (!sheetOverviewEl || !queryEl || !coverageEl || !partsEl || !reviewEl || !conventionEl) return children;
    if (!traceEl || !tablesEl) return children;
    if (!projectEl || !propertiesEl) return children;

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
      tablesPresenter.setCompareOpen(s.active && s.error === "");
    });

    // The Tables panel (agni issues 898 and 956) shows an answer the page already holds as Tabulate's
    // tables and saves them as csv through Encode. Its answers come from the presenters that hold
    // them, and a comparison's from the open pair, asked again with the panel's diff options.
    const tables = tablesPanelIsland(tablesEl, this._eventBus, {
      onSource: (key, opts) => void tablesPresenter.show(key, opts),
      onSelect: (i) => tablesPresenter.select(i),
      onSort: (col) => void tablesPresenter.sortBy(col),
      onSave: () => void tablesPresenter.saveActive(),
      onLocate: (kind, subject) => {
        void presenter.locateEntity(kind, subject);
        if (dockApi) dockApi.getPanel("canvas")?.api.setActive();
      },
    });
    const sideLabel = (r: { path: string }) => r.path.split("/").filter(Boolean).pop() ?? r.path;
    const tablesPresenter = new TablesPresenter(
      tableClient(),
      tables.view,
      {
        answer: async (source: TablesSourceKey, opts) => {
          if (source === "checks") {
            const answer = presenter.checksAnswer();
            return { answer, label: "Checks", stem: presenter.designStem(), reason: "Run the checks first. The tables are the run on screen." };
          }
          const pair = diffPresenter.sides();
          if (!pair) return { label: "Comparison", stem: "", reason: "Open a comparison with Compare first." };
          const stem = `${sideLabel(pair.a).replace(/\.[^.]+$/, "")}-vs-${sideLabel(pair.b).replace(/\.[^.]+$/, "")}`;
          if (source === "compare") {
            const resp = await diffClient().diffDesigns({
              aUri: artifactUri(pair.a.mount, pair.a.path),
              bUri: artifactUri(pair.b.mount, pair.b.path),
              nearRenames: opts.nearRenames ? {} : undefined,
              includeEqual: opts.equal,
            });
            return { answer: { case: "diff", value: resp }, label: `${sideLabel(pair.a)} to ${sideLabel(pair.b)}`, stem };
          }
          const side = source === "compare-a" ? pair.a : pair.b;
          const run = await checksClient().checkDesign({ uri: artifactUri(side.mount, side.path), rules: [], overlay: presenter.overlayConfig() });
          return { answer: { case: "check", value: run }, label: `Checks on ${sideLabel(side)}`, stem: sideLabel(side).replace(/\.[^.]+$/, "") };
        },
      },
      saveFile,
    );

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
      propertiesPresenter.selectDesign();
    });
    // The merged checks panel lists rule findings for the loaded design (grouped/sorted client-side)
    // and hosts the on-demand Run button; clicking a finding highlights its subject (net/ref_des),
    // pressing Run evaluates the current rule selection (WS9).
    const findings = findingsPanelIsland(findingsEl, this._eventBus, {
      // A finding names its subject, so clicking one selects it too and Properties follows.
      onSelect: (subject, sheet, netId) =>
        void presenter.selectFinding(subject, sheet, netId).then((sel) => {
          if (sel) propertiesPresenter.selectEntity(sel);
        }),
      // A context entity is not a finding, so it locates through the same path a query result cell
      // does (locateEntity) rather than through selectFinding, whose lookup is by finding subject
      // and would find nothing (agni issue 349).
      onLocateContext: (kind, subject, pin) => void presenter.locateEntity(kind, subject, undefined, LocateReason.UNSPECIFIED, pin),
      onRun: () => void presenter.runChecks(),
      // A verdict is addressed by its derived id, not by subject, because a passing verdict has no
      // finding for selectFinding to look up and two rules can hold verdicts about one subject.
      onSelectVerdict: (id) => void presenter.locateVerdict(id),
      onSave: (format) =>
        void (format === "csv" ? presenter.saveChecksCSV((a, s, n) => tablesPresenter.saveAnswer(a, s, n)) : presenter.saveReport(format)),
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
      onRun: (text, bindings, limit, tab) => void presenter.runQuery(text, bindings, limit, tab),
      onSaveCSV: (tab) => void presenter.saveQueryCSV((a, s, n) => tablesPresenter.saveAnswer(a, s, n), tab),
      // A tab coming forward shows what it last located, so the drawing matches the answer in front.
      // One that located nothing releases what the tab before it had located.
      onActivate: (_tab, located, previous) => {
        if (located) void presenter.locateEntity(located.kind, located.subject, located.sheet, located.reason, located.pin ?? "");
        else if (previous) void presenter.releaseFocus();
      },
      onClose: (tab) => presenter.forgetQueryTab(tab),
      onRunSet: (set, tabs) => void presenter.runQuerySet(set, tabs),
      // A set file from disk is parsed by the engine, which checks it as `agni query --set` does, so
      // the browser holds no YAML parser of its own (C22).
      onSetFile: (name, text) =>
        void queryClient()
          .parseQuerySet({ text })
          .then((r) => {
            if (r.set) query.view.openSet(r.set.title ? r.set : { ...r.set, title: name });
          })
          .catch((e: unknown) => query.view.setLocateNote(`${name}: ${e instanceof Error ? e.message : String(e)}`)),
      onLocate: (kind, subject, sheet, reason, pin) => {
        void presenter.locateEntity(kind, subject, sheet, reason, pin ?? "");
        // A located cell is also selected, so Properties follows the walk through an answer. A pin
        // arrives as its component and designator, the other cells name themselves.
        const sel = kind === "pin" ? (pin ? { kind: "pin" as const, ref: subject, pin } : null) : selectionFromCell(kind, subject);
        if (sel) propertiesPresenter.selectEntity(sel);
      },
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
    // Properties describes whatever is selected (agni issue 902). A click on the drawing selects and
    // highlights, and leaves the query panel alone, since a stray click used to overwrite whatever the
    // reader had typed there. Asking about the selection is one of the panel's Ask rows, each opening a
    // served question in a query tab of its own and bringing the query panel forward.
    const properties = propertiesPanelIsland(propertiesEl, this._eventBus, {
      onFocus: (focus) => {
        // A sheet reference names a sheet the selection is drawn on, which may not be the one on screen.
        if (focus.level === "sheet" && focus.sheetId !== propertiesPresenter.activeSheetId()) void presenter.showSheet(focus.sheetId);
        propertiesPresenter.select(focus);
        if (focus.level === "entity") locateSelection(focus.sel);
      },
      onAsk: (sel, question) => {
        query.view.askAbout(sel, question);
        if (dockApi) dockApi.getPanel("query")?.api.setActive();
      },
    });
    // Where a selected part or net is drawn is the engine's answer to the served find-by-name query,
    // anchored to the whole name, since each entity in a query answer carries the sheets it is drawn
    // on. Before the catalog arrives there is no template, and the panel lists no sheets.
    let findByName: { query: string; bind: string } | null = null;
    const propertiesPresenter = new PropertiesPresenter(properties.view, {
      getDesign: (req) => designClient().getDesign(req),
      drawn: () => drawnOnSheet(svgEl),
      drawnOn: async (uri, sel) => {
        if (!findByName) return [];
        const name = sel.kind === "net" ? (sel.net ?? "") : (sel.ref ?? "");
        const resp = await queryClient().runQuery({
          uri,
          query: findByName.query,
          overlay: presenter.overlayConfig(),
          bindings: { [findByName.bind]: { kind: { case: "text", value: `^${searchPattern(name)}$` } } },
        });
        return drawnSheets(resp, sel.kind, name);
      },
    });
    const locateSelection = (sel: Selection) =>
      void presenter.locateEntity(sel.kind, sel.ref ?? sel.net ?? sel.busId ?? "", undefined, undefined, sel.pin);
    svgView.onPick = (sel) => {
      locateSelection(sel);
      propertiesPresenter.selectEntity(sel);
      if (dockApi) dockApi.getPanel("properties")?.api.setActive();
    };
    // A click on empty paper names the sheet being looked at.
    svgView.onPickEmpty = () => {
      propertiesPresenter.selectSheet();
      if (dockApi) dockApi.getPanel("properties")?.api.setActive();
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
      onLocate: (kind, subject) => {
        void presenter.locateEntity(kind, subject);
        const sel = selectionFromCell(kind, subject);
        if (sel) propertiesPresenter.selectEntity(sel);
      },
      // The checklist page, written by the rpc `agni review --format html` calls (agni issue 1030).
      onSave: () => void presenter.saveReviewReport(),
    }, { staticHost: onStaticHost });
    // The relation catalog (WS9-037) is static per build and design-independent, so fetch it once
    // at startup and push it to the panel's picker; a failure just leaves the picker empty (the
    // panel falls back to the syntax hint).
    void queryClient()
      .listRelations({})
      .then((r) => {
        query.view.setRelations(r.relations);
        query.view.setExamples(r.examples); // WS14-002: starter queries beside the relation picker
        query.view.setEntityQueries(r.entityQueries); // the questions about a selection
        propertiesPresenter.setQuestions(r.entityQueries);
        // The find-by-name template (agni issue 338). A server that sends none leaves the panel
        // with no search mode, since the template names relations only the server defines.
        const sq = r.searchQuery;
        query.view.setSearch(sq ? { query: sq.query, teaches: sq.teaches, bind: sq.bind, pattern: sq.pattern } : null);
        findByName = sq ? { query: sq.query, bind: sq.bind } : null;
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
        sheetNavs: [sheetTabs.view, { setState: (st) => propertiesPresenter.setSheets(st) }],
        summary: (text) => propertiesPresenter.setSummary(text),
        controls: controls.view,
        findings: {
          setState: (st) => {
            findings.view.setState(st);
            propertiesPresenter.setFindings(st);
          },
          setFindingLocateNote: (note) => findings.view.setFindingLocateNote(note),
        },
        expectationCaption: setExpectCaption,
        undrawnNote: setUndrawnNote,
        staleLinkNote: setStaleLinkNote,
        rules: rules.view,
        report: (report) => propertiesPresenter.setReport(report),
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
        parts: {
          setState: (st) => {
            parts.view.setState(st);
            propertiesPresenter.setDatasheet(st);
          },
        },
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
      tables.island,
      review.island,
      conventionBar.island,
      projectBar.island,
      parts.island,
      properties.island,
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

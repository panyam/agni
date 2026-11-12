// Entry point for the design browse page (/designs, WS9-049 phase 2). Choosing a design previews
// it read-only, and Open leaves for that design's work page. The page shapes are in
// docsite/content/architecture/web-app.md#the-pages.
//
// This page builds no ViewerPresenter, no WebGL canvas, no checks / query / diff clients and no
// render-mode or layout controls. Browsing happens before you have chosen, so it costs one design
// summary and one SVG.

import { BaseComponent, EventBus, LifecycleController, type LCMComponent } from "@panyam/tsappkit";
import { fileTreeIsland } from "./filetree.js";
import { SvgView } from "./svgview.js";
import { DesignPreview } from "./preview.js";
import { browseStage, type StageElements } from "./browsestage.js";
import { designClient } from "./api.js";
import { currentLocation, emptyLocation, hasDir, locationToUrl, type ViewerLocation } from "./router.js";

// syncUrl reflects the browsed folder into the address bar. Only folders are addressable here. A
// design's URL is its work page, so pushing one per preview would make Back walk through previews.
function syncUrl(loc: ViewerLocation): void {
  document.title = hasDir(loc) ? `${loc.path || loc.mount} — Agni designs` : "Agni designs";
  const url = locationToUrl(loc);
  if (url !== window.location.pathname) window.history.pushState(null, "", url);
}

class BrowseRoot extends BaseComponent {
  // revealDir expands the tree to a folder URL on restore (deep link / back-forward), set once the
  // file-tree island is built.
  revealDir: (mount: string, path: string) => void = () => {};

  override performLocalInit(): LCMComponent[] {
    const children: LCMComponent[] = [];

    const treeEl = document.getElementById("browse-tree");
    const previewEl = document.getElementById("browse-preview");
    const els: Partial<StageElements> = {
      note: document.getElementById("browse-note") ?? undefined,
      name: document.getElementById("browse-name") ?? undefined,
      summary: document.getElementById("browse-summary") ?? undefined,
      open: (document.getElementById("browse-open") as HTMLButtonElement | null) ?? undefined,
    };
    if (!treeEl || !previewEl || !els.note || !els.name || !els.summary || !els.open) return children;

    const stage = browseStage(els as StageElements, new SvgView(previewEl), (url) => window.location.assign(url));
    const preview = new DesignPreview(designClient(), stage);

    // The viewer's own file tree, reused whole. No presenter feeds it sheets here, so it shows a flat
    // file list and keeps its lazy directory listing, auto-reveal and no-reader filter.
    const tree = fileTreeIsland(treeEl, this._eventBus, {
      onFileSelect: (mount, path) => {
        stage.setTarget({ mount, path });
        tree.view.setState({ mount, path, sheets: [], activeId: "" }); // highlight it in the list
        void preview.show(mount, path);
      },
      // A folder re-addresses the URL and empties the stage, so the previous preview does not stay on
      // screen under a new location.
      onDirSelect: (mount, path) => {
        stage.setTarget(null);
        tree.view.setState({ mount: "", path: "", sheets: [], activeId: "" });
        preview.clear();
        syncUrl({ ...emptyLocation(), mount, path, isDir: true });
      },
      // No sheet ever reaches this tree (nothing pushes sheet state), so this cannot fire.
      onSheetSelect: () => {},
    });
    this.revealDir = tree.revealDir;

    // Double-click and Enter open the CURRENT selection, which a plain click has already set, so
    // they bind on the tree container and the island needs no new handler or prop.
    treeEl.addEventListener("dblclick", () => stage.open());
    treeEl.addEventListener("keydown", (e) => {
      if (e.key === "Enter") stage.open();
    });

    children.push(tree.island);
    return children;
  }
}

const bus = new EventBus();
const controller = new LifecycleController(bus);
const root = new BrowseRoot("app", document.body, bus);
void controller
  .initializeFromRoot(root)
  .then(() => {
    // applyUrl expands the tree to the folder the URL addresses, at boot and on every popstate.
    // Revealing pushes no URL, so unlike the viewer's restore this needs no re-entrancy guard.
    const applyUrl = (): void => {
      const loc = currentLocation();
      if (hasDir(loc)) root.revealDir(loc.mount, loc.path);
    };
    window.addEventListener("popstate", applyUrl);
    applyUrl();
  })
  .catch((err) => {
    console.error(err);
  });

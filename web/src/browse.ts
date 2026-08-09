// Entry point for the design browse page (/designs, WS9-049 phase 2), the third page beside the
// viewer's main.ts and the workbench's datasheets.ts. The page is server-rendered by
// goapplib/templar (a file-list sidebar and a preview stage). This boots the tsappkit lifecycle
// over that shell: the file-tree island initializes, choosing a design previews it read-only, and
// Open leaves for that design's work page.
//
// What this page deliberately does NOT build is the point of it: no ViewerPresenter, no WebGL
// canvas, no checks / query / diff clients, no render-mode or layout controls. Browsing is the
// moment before you have chosen, so it costs one design summary and one SVG.

import { BaseComponent, EventBus, LifecycleController, type LCMComponent } from "@panyam/tsappkit";
import { fileTreeIsland } from "./filetree.js";
import { SvgView } from "./svgview.js";
import { DesignPreview } from "./preview.js";
import { browseStage, type StageElements } from "./browsestage.js";
import { designClient } from "./api.js";
import { currentLocation, emptyLocation, hasDir, locationToUrl, type ViewerLocation } from "./router.js";

// syncUrl reflects the browsed folder into the address bar. Only folders are addressable here: a
// previewed design does not own the URL, because the URL that names a design is its work page, and
// pushing that from the browse page would make Back walk through previews.
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

    // The viewer's own file tree, reused whole. The presenter is what normally feeds it sheets; here
    // nothing ever does, so its sheet state stays empty and no sheets nest under a file — the tree
    // degrades to the flat file list this page wants, with its lazy directory listing, auto-reveal,
    // and no-reader filter intact.
    const tree = fileTreeIsland(treeEl, this._eventBus, {
      onFileSelect: (mount, path) => {
        stage.setTarget({ mount, path });
        tree.view.setState({ mount, path, sheets: [], activeId: "" }); // highlight it in the list
        void preview.show(mount, path);
      },
      // A folder is a place to look, not a thing to open: it re-addresses the URL and empties the
      // stage rather than leaving the previously previewed design on screen under a new location.
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

    // The same open action on the two gestures a file list is expected to answer to. Both are
    // bound on the tree container rather than inside the island: they act on the CURRENT selection,
    // which a plain click has already set, so the island needs no new handler and no new prop.
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
    // applyUrl expands the tree to whatever folder the URL addresses, once at boot (deep link) and
    // on every popstate (back/forward). Revealing pushes no URL, so unlike the viewer's restore
    // this needs no re-entrancy guard.
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

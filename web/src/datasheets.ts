// Entry point for the extraction workbench shell (the /datasheets page, WS13-006), a page distinct
// from the viewer's main.ts. The page is server-rendered by goapplib/templar (a datasheet-tree
// sidebar and the region-viewer hole). This boots the tsappkit lifecycle over that shell: the tree
// island and the region-viewer island initialize, a tree selection loads a datasheet into the
// viewer, and the open datasheet lives in the URL (/datasheets/files/<mount>/<path>) so a refresh
// or shared link reopens it.

import { BaseComponent, EventBus, LifecycleController, type LCMComponent } from "@panyam/tsappkit";
import { dsTreeIsland, type DsTreeView } from "./dstree.js";
import { workbenchIsland, type RegionView } from "./regionview.js";
import { realPdfSource } from "./pdfrender.js";
import { paramsPanelIsland } from "./paramspanel.js";
import type { Parameter } from "./gen/agni/v1/param/param_pb.js";
import { currentDs, dsToUrl, hasDatasheet, type DsLocation } from "./dsrouter.js";
import { baseName, noteOpen } from "./recents.js";

// restoring guards the URL feedback loop, like the viewer's main.ts: while replaying a URL (initial
// load or back/forward) we load the datasheet but must not push a duplicate history entry.
let restoring = false;

// syncUrl reflects the open datasheet into the address bar, pushing history only on a real change
// and not mid-restore, so normal navigation builds a back-stack while a replay does not.
function syncUrl(loc: DsLocation): void {
  document.title = hasDatasheet(loc) ? `${loc.path || loc.mount} — Agni datasheets` : "Agni datasheets";
  if (restoring) return;
  const url = dsToUrl(loc);
  if (url !== window.location.pathname) window.history.pushState(null, "", url);
}

class DatasheetsRoot extends BaseComponent {
  // view/tree are exposed so the boot code can drive a deep-link restore once the islands init.
  view: RegionView | null = null;
  tree: DsTreeView | null = null;
  // open is exposed for the same reason, and it is what the restore SHOULD drive: the boot code
  // used to reach past it to view.load + tree.setState, which is the same sequence minus whatever
  // open gains later. It gained recording, and a deep-linked datasheet stopped being recorded.
  open: ((mount: string, path: string) => void) | null = null;

  override performLocalInit(): LCMComponent[] {
    const children: LCMComponent[] = [];
    const treeEl = document.getElementById("ds-tree");
    const viewEl = document.getElementById("ds-view");
    const paramsEl = document.getElementById("ds-params");
    if (!treeEl || !viewEl || !paramsEl) return children;

    // The workbench pushes its parameter list to the params panel (forward ref: the panel is built
    // after the workbench so its onLocate can call workbench.locate).
    let pushParams: (p: Parameter[]) => void = () => {};
    // pdf.js enters the app HERE and nowhere else, so the workbench can be rendered by a test.
    const region = workbenchIsland(viewEl, this._eventBus, (p) => pushParams(p), realPdfSource);
    const params = paramsPanelIsland(paramsEl, this._eventBus, (page, regionId) => region.view.locate(page, regionId));
    pushParams = params.view.setState;

    let treeView: DsTreeView | null = null;
    // open is the single "show this datasheet" action, shared by a tree click and a URL restore:
    // load it into the viewer, highlight it in the tree, and reflect it into the URL.
    const open = (mount: string, path: string): void => {
      region.view.load(mount, path);
      treeView?.setState({ mount, path });
      syncUrl({ mount, path });
      // Feeds the landing page's Recent list. Every way of showing a datasheet goes through here,
      // including a deep link and back/forward, so arriving by URL counts as an opening the way it
      // does in the viewer.
      noteOpen({ kind: "datasheet", mount, path, label: baseName(path) });
    };
    const tree = dsTreeIsland(treeEl, this._eventBus, open);
    treeView = tree.view;
    this.view = region.view;
    this.tree = tree.view;
    this.open = open;

    children.push(tree.island, region.island, params.island);
    return children;
  }
}

const bus = new EventBus();
const controller = new LifecycleController(bus);
const root = new DatasheetsRoot("app", document.body, bus);
void controller.initializeFromRoot(root).then(() => {
  // applyUrl opens whatever the current URL addresses, once at boot (deep link) and on every
  // popstate (back/forward). The restoring flag keeps the replay from pushing a duplicate entry.
  const applyUrl = (): void => {
    const loc = currentDs();
    if (!hasDatasheet(loc)) return; // bare /datasheets, leave the empty shell
    restoring = true;
    try {
      // The same action a tree click takes. syncUrl is a no-op while restoring, so replaying a URL
      // through it pushes no duplicate history entry.
      root.open?.(loc.mount, loc.path);
    } finally {
      restoring = false;
    }
  };
  window.addEventListener("popstate", () => applyUrl());
  applyUrl();
});

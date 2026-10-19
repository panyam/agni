// Entry point for the Load page (/designs/<mount>/<path>/load, agni issue 1072). It reads the design
// the address names into the engine's store and then opens the viewer at the same address, so the
// viewer opens a design already parsed. The browser mount's Load page with no design is where files
// are dropped or picked, and where the drops this browser keeps are listed.
//
// The page is plain DOM: every string it shows about a drop came from the visitor's files, so each is
// set as text and never parsed as markup.

import { designListing, loadedKey, pageEngine, refuseBrowserMounts, showEngineNote, startPageEngine, withEngineParams } from "./engineboot.js";
import { designClient, snapshots, workspaceClient } from "./api.js";
import { currentLocation, dropPageUrl, emptyLocation, hasFile, locationToUrl, swapPage } from "./router.js";
import { artifactUri } from "@agni/web-shared/uri.js";
import { LoadFlow, type LoadStep } from "./loadflow.js";
import { LOADED_PARAM, recordLoaded } from "./loaded.js";
import { browserDropStore, dropOf, type DropStore, type SavedDrop } from "./drops.js";
import { BROWSER_MOUNT, installDrop, proposeDrop, type DropDeps } from "./wasm/drop.js";
import type { WasmEngine } from "./wasm/client.js";

const MARKS: Record<LoadStep["state"], string> = { waiting: "○", running: "◐", done: "✓", failed: "✗" };

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function storage(): Storage | undefined {
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

// viewerUrl is the viewer's address for the design this page is reading, marked as arriving from
// here (LOADED_PARAM).
function viewerUrl(): string {
  const u = new URL(swapPage(window.location.pathname, window.location.search, "view"), window.location.origin);
  u.searchParams.set(LOADED_PARAM, "1");
  return u.pathname + u.search;
}

// loadUrl is the Load page's address for a dropped design.
function loadUrl(path: string): string {
  return withEngineParams(locationToUrl({ ...emptyLocation(), mount: BROWSER_MOUNT, path }, "load"));
}

function renderSteps(list: HTMLElement, steps: LoadStep[]): void {
  list.replaceChildren(
    ...steps.map((s) => {
      const li = el("li", "ld-step");
      li.dataset.step = s.id;
      li.dataset.state = s.state;
      li.append(el("span", "ld-step-mark", MARKS[s.state]));
      const label = el("span", "ld-step-label", s.label);
      if (s.detail) label.append(el("div", "ld-step-detail", s.detail));
      li.append(label);
      return li;
    }),
  );
}

// loadDesign reads the design the address names, then opens the viewer on it.
async function loadDesign(mount: string, path: string, drops: DropStore): Promise<void> {
  const section = document.getElementById("load-design");
  const list = document.getElementById("load-steps");
  const failedEl = document.getElementById("load-failed");
  const anyway = document.getElementById("load-anyway") as HTMLAnchorElement | null;
  if (!section || !list || !failedEl || !anyway) return;
  section.hidden = false;
  const title = document.getElementById("load-title");
  if (title) title.textContent = path.split("/").pop() || path;
  document.title = `Loading ${path} — Agni`;
  const uri = artifactUri(mount, path);
  const fail = (reason: string, viewer = true) => {
    failedEl.textContent = reason;
    failedEl.hidden = false;
    anyway.href = viewer ? viewerUrl() : dropPageUrl();
    anyway.textContent = viewer ? "Open the viewer anyway" : "Drop the files again";
    anyway.hidden = false;
  };
  const wasm = pageEngine() === "wasm";
  // A dropped design's files are the page's own, so the engine is given them before anything asks.
  let dropped: Record<string, Uint8Array> | undefined;
  if (mount === BROWSER_MOUNT) {
    if (!wasm) {
      refuseBrowserMounts();
      fail("A dropped design is analysed in your browser, so this page needs the browser engine: add ?engine=wasm to its address.", false);
      return;
    }
    try {
      dropped = await drops.files(dropOf(path));
    } catch (e) {
      fail(`This browser would not read the files it kept: ${e instanceof Error ? e.message : String(e)}.`, false);
      return;
    }
    if (!Object.keys(dropped).length) {
      fail("This browser no longer keeps the files of this dropped design.", false);
      return;
    }
  }
  let flow: LoadFlow | undefined;
  let engine: WasmEngine | undefined;
  try {
    engine = await startPageEngine({ onFetched: (p, bytes) => flow?.fetched(p, bytes) });
  } catch (e) {
    fail(`The browser engine would not start: ${e instanceof Error ? e.message : String(e)}.`, false);
    return;
  }
  if (engine && dropped) await engine.add(BROWSER_MOUNT, dropped);
  flow = new LoadFlow(
    {
      uri,
      fetches: wasm && mount !== BROWSER_MOUNT,
      listing: async () => {
        const l = await designListing(mount, path, drops);
        if (!l) throw new Error("This browser no longer keeps the files of this dropped design.");
        return l;
      },
      drawing: async () => {
        await designClient().getDesign({ snapshot: await snapshots().of(uri) });
      },
      board: async () => {
        const t = (await snapshots().readBoard(uri)).tiers;
        return { boardRead: t?.boardRead ?? false, boardPersisted: t?.boardPersisted ?? false };
      },
      record: (stamp) => recordLoaded(storage(), loadedKey(mount, path), stamp),
      open: () => window.location.replace(viewerUrl()),
      log: (line) => console.info(line),
    },
    {
      steps: (steps) => renderSteps(list, steps),
      failed: (reason) => fail(`The design could not be read: ${reason}`),
    },
  );
  renderSteps(list, flow.steps());
  await flow.run();
}

function describe(d: SavedDrop): string {
  const mb = (d.bytes / (1 << 20)).toFixed(1);
  return `${d.files} file${d.files === 1 ? "" : "s"} · ${mb} MB · dropped ${new Date(d.savedAt).toLocaleString()}`;
}

// renderDrops lists the drops this browser keeps, each reopened through the same proposal a new drop
// shows, or deleted.
async function renderDrops(listEl: HTMLElement, deps: DropDeps): Promise<void> {
  let all: SavedDrop[];
  try {
    all = await deps.drops.list();
  } catch (e) {
    listEl.replaceChildren(el("li", "ld-empty", `This browser keeps no dropped files here: ${e instanceof Error ? e.message : String(e)}.`));
    return;
  }
  if (!all.length) {
    listEl.replaceChildren(el("li", "ld-empty", "Nothing yet."));
    return;
  }
  listEl.replaceChildren(
    ...all.map((d) => {
      const li = el("li", "ld-drop");
      li.dataset.drop = d.id;
      const name = el("span", "ld-drop-name", d.name);
      name.append(el("div", "ld-drop-meta", describe(d)));
      const open = el("button", "ld-drop-open", "Open");
      open.type = "button";
      open.addEventListener("click", () => {
        void deps.drops
          .files(d.id)
          .then((files) => proposeDrop(deps, d.id, files))
          .catch((e: unknown) => deps.refuse(`Could not open ${d.name}: ${String(e)}`));
      });
      const del = el("button", "ld-drop-delete", "Delete");
      del.type = "button";
      del.addEventListener("click", () => {
        void deps.drops
          .remove(d.id)
          .then(() => renderDrops(listEl, deps))
          .catch((e: unknown) => deps.refuse(`Could not delete ${d.name}: ${String(e)}`));
      });
      li.append(name, open, del);
      return li;
    }),
  );
}

// dropPage shows the drop target and the drops this browser keeps.
async function dropPage(drops: DropStore): Promise<void> {
  const section = document.getElementById("load-drop");
  const dialog = document.getElementById("drop-dialog") as HTMLDialogElement | null;
  const button = document.getElementById("drop-open");
  const input = document.getElementById("drop-input") as HTMLInputElement | null;
  const listEl = document.getElementById("saved-drops");
  if (!section || !dialog || !button || !input || !listEl) return;
  section.hidden = false;
  document.title = "Open your own design — Agni";
  const engine = await startPageEngine();
  if (!engine) refuseBrowserMounts();
  const files = workspaceClient();
  const deps: DropDeps = {
    engine,
    drops,
    propose: (uri) => files.proposeDesigns({ uri }),
    open: (path) => window.location.assign(loadUrl(path)),
    refuse: (reason) => showEngineNote(`${reason}.`),
    target: document.body,
    dialog,
    button,
    input,
    folderButton: document.getElementById("drop-folder") ?? undefined,
    folderInput: (document.getElementById("drop-folder-input") as HTMLInputElement | null) ?? undefined,
  };
  installDrop(deps);
  await renderDrops(listEl, deps);
  // A new drop lands in the listing too, once its proposal is up.
  dialog.addEventListener("close", () => void renderDrops(listEl, deps));
}

const drops = browserDropStore();
const here = currentLocation();
void (async () => {
  if (hasFile(here)) await loadDesign(here.mount, here.path, drops);
  else await dropPage(drops);
})().catch((err: unknown) => {
  console.error(err);
  showEngineNote(`error: ${String(err)}`);
});

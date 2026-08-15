import { createEffect, createSignal, For, Show, type Accessor } from "solid-js";
import { artifactUri, uriPath } from "./uri.js";
import type { Client } from "@connectrpc/connect";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import { WorkspaceService, type DirEntry, type Mount } from "./gen/agni/v1/webapi/workspace_pb.js";
import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";
import { workspaceClient } from "./api.js";
import type { SheetsState, SheetsView } from "./sheets.js";

type WsClient = Client<typeof WorkspaceService>;

// TreeHandlers are the tree's intents: the user chose a file, a folder, or a sheet within the
// open file. All carry semantic ids, never DOM events. A folder selection changes nothing on
// screen (whatever was rendered stays); it only makes the folder the current URL, so a refresh
// reopens the tree expanded to it.
export interface TreeHandlers {
  onFileSelect: (mount: string, path: string) => void;
  onDirSelect: (mount: string, path: string) => void;
  onSheetSelect: (sheetId: string) => void;
}

// RevealTarget is a folder the tree should auto-expand to (the mount root is path ""), null when
// there is none. It is set on a deep-link/back-forward restore of a folder URL, separately from
// the open file, so the tree can reveal a folder that has no design loaded under it.
export type RevealTarget = { mount: string; path: string } | null;

// ctx is threaded to every node so leaves can reach the client, the current sheets, the reveal
// target, and the handlers without prop drilling each field.
interface Ctx extends TreeHandlers {
  client: WsClient;
  sheets: Accessor<SheetsState>;
  revealTarget: Accessor<RevealTarget>;
}

interface SheetNodeData {
  sheet: SheetRef;
  children: SheetNodeData[];
}

// sheetForest groups a flat sheet list into a parent/child forest by parent_id: sheets with an
// empty (or unknown) parent are top-level roots, and there may be several (EDIF pages are all
// top-level; a KiCad design has one root with descendants).
function sheetForest(sheets: SheetRef[]): SheetNodeData[] {
  const nodes = sheets.map((s) => ({ sheet: s, children: [] as SheetNodeData[] }));
  const byId = new Map(nodes.map((n) => [n.sheet.id, n]));
  const roots: SheetNodeData[] = [];
  for (const n of nodes) {
    const parent = n.sheet.parentId ? byId.get(n.sheet.parentId) : undefined;
    if (parent) parent.children.push(n);
    else roots.push(n);
  }
  return roots;
}

function SheetNode(props: { ctx: Ctx; node: SheetNodeData; depth: number }) {
  const active = (): boolean => props.ctx.sheets().activeId === props.node.sheet.id;
  return (
    <li>
      <button
        class={`node sheet${active() ? " active" : ""}`}
        style={{ "padding-left": `${props.depth * 12 + 4}px` }}
        onClick={() => props.ctx.onSheetSelect(props.node.sheet.id)}
      >
        {props.node.sheet.name || props.node.sheet.id}
      </button>
      <Show when={props.node.children.length}>
        <ul class="children">
          <For each={props.node.children}>{(c) => <SheetNode ctx={props.ctx} node={c} depth={props.depth + 1} />}</For>
        </ul>
      </Show>
    </li>
  );
}

// FileNode is a design file. Clicking it opens the file; when it is the open file, its sheet
// forest renders nested beneath it (driven by the presenter-pushed SheetsState). On a deep-link
// restore the file becomes active without a click, so it scrolls itself into view.
function FileNode(props: { ctx: Ctx; mount: string; entry: DirEntry; depth: number }) {
  const isOpen = (): boolean => props.ctx.sheets().mount === props.mount && props.ctx.sheets().path === uriPath(props.entry.uri);
  let btn: HTMLButtonElement | undefined;
  createEffect(() => {
    if (isOpen()) btn?.scrollIntoView({ block: "nearest" });
  });
  return (
    <li>
      <button
        ref={btn}
        class={`node file${isOpen() ? " active" : ""}`}
        style={{ "padding-left": `${props.depth * 12 + 4}px` }}
        onClick={() => props.ctx.onFileSelect(props.mount, uriPath(props.entry.uri))}
      >
        <span class="twist" /> {props.entry.name} <span class="fmt">{props.entry.format}</span>
      </button>
      <Show when={isOpen() && props.ctx.sheets().sheets.length > 0}>
        <ul class="children">
          <For each={sheetForest(props.ctx.sheets().sheets)}>{(n) => <SheetNode ctx={props.ctx} node={n} depth={props.depth + 1} />}</For>
        </ul>
      </Show>
    </li>
  );
}

// subtreeHasActive reports whether the open file (from the presenter-pushed SheetsState) lives
// somewhere under this directory, so its ancestors can auto-expand to reveal it. The mount root
// (path "") contains every file in its mount; a subdirectory contains files whose path is under
// its own path.
function subtreeHasActive(mount: string, dirPath: string, s: SheetsState): boolean {
  if (s.mount !== mount || !s.path) return false;
  return dirPath === "" || s.path.startsWith(dirPath + "/");
}

// revealsTarget reports whether this directory should open to reveal the reveal target: true when
// the target folder is this directory itself or one of its descendants, so every ancestor down to
// the target expands. Unlike subtreeHasActive (a file, whose path never equals a dir path), the
// target is a folder, so an exact path match counts — the target folder opens to show its own
// contents.
function revealsTarget(mount: string, dirPath: string, t: RevealTarget): boolean {
  if (!t || t.mount !== mount) return false;
  return dirPath === "" || t.path === dirPath || t.path.startsWith(dirPath + "/");
}

// DirNode is a directory (mount root or subdirectory). It lazily lists one level on expand, and
// auto-expands to reveal the deep-linked file when a restore makes a descendant the open file.
function DirNode(props: { ctx: Ctx; mount: string; path: string; label: string; depth: number }) {
  const [open, setOpen] = createSignal(false);
  const [entries, setEntries] = createSignal<DirEntry[] | null>(null);
  const [error, setError] = createSignal<string | null>(null);

  // loadEntries lists this directory's one level, once. Shared by manual toggle and auto-reveal.
  const loadEntries = async (): Promise<void> => {
    if (entries() !== null) return;
    try {
      // pruneEmptyDirs asks the server to leave out subdirectories with no readable design under
      // them. It has to be answered server-side: a client sees one level per call, so it cannot
      // tell a folder of designs from a folder of folders of nothing without walking the tree.
      const resp = await props.ctx.client.listDir({ uri: artifactUri(props.mount, props.path), pruneEmptyDirs: true });
      setEntries(resp.entries);
    } catch (e) {
      setError(String(e));
    }
  };

  const toggle = async (): Promise<void> => {
    // A click also makes this folder the current location (a valid, refreshable URL), whether it
    // expands or collapses. It renders nothing new; the presenter's view is left untouched.
    props.ctx.onDirSelect(props.mount, props.path);
    const next = !open();
    setOpen(next);
    if (next) await loadEntries();
  };

  // Auto-reveal: when the open file (or a restored folder URL) lands at or under this directory,
  // open it and list its children. As each level's entries load, the next-level DirNodes mount and
  // their own effects run, so the expansion cascades down to the target. This only opens; a user's
  // manual collapse of an unrelated branch is left alone.
  createEffect(() => {
    if (subtreeHasActive(props.mount, props.path, props.ctx.sheets()) || revealsTarget(props.mount, props.path, props.ctx.revealTarget())) {
      setOpen(true);
      void loadEntries();
    }
  });

  return (
    <li>
      <button class="node dir" style={{ "padding-left": `${props.depth * 12 + 4}px` }} onClick={toggle}>
        <span class="twist">{open() ? "▾" : "▸"}</span> {props.label}
      </button>
      <Show when={open()}>
        <ul class="children">
          <Show when={error()}>{(msg) => <li class="error">{msg()}</li>}</Show>
          {/* Files with no reader (empty format) are hidden — library files, lock files, and
              sidecars were drowning real designs (2026-07-14 feedback; reversal of the earlier
              show-greyed choice). The server still lists them, so this stays a view filter. */}
          <For each={(entries() ?? []).filter((e) => e.isDir || e.format)}>
            {(e) =>
              e.isDir ? (
                <DirNode ctx={props.ctx} mount={props.mount} path={uriPath(e.uri)} label={e.name} depth={props.depth + 1} />
              ) : (
                <FileNode ctx={props.ctx} mount={props.mount} entry={e} depth={props.depth + 1} />
              )
            }
          </For>
        </ul>
      </Show>
    </li>
  );
}

// hiddenNote words the pruned-mount count for the sidebar. A mount is something an operator
// configured by hand, so one missing from the tree has to be accounted for: without this line
// there is no way to tell "that folder holds no designs" from "that mount failed to resolve".
function hiddenNote(hidden: number, shown: number): string {
  const folders = `${hidden} ${hidden === 1 ? "folder" : "folders"}`;
  return shown === 0 ? `No designs in any of the ${folders} being served` : `${folders} hidden (no designs)`;
}

function FileTree(props: { ctx: Ctx }) {
  const [mounts, setMounts] = createSignal<Mount[]>([]);
  const [pruned, setPruned] = createSignal(0);
  const [error, setError] = createSignal<string | null>(null);
  // pruneEmptyMounts applies the same rule to the roots that pruneEmptyDirs applies inside them: a
  // mount serving only datasheets or only library files is one this tree can never show anything
  // in. The datasheets tree roots on the same mounts and does not set it.
  props.ctx.client
    .listMounts({ pruneEmptyMounts: true })
    .then((r) => {
      setMounts(r.mounts);
      setPruned(r.prunedMounts);
    })
    .catch((e) => setError(String(e)));

  return (
    <ul class="tree">
      <Show when={error()}>{(msg) => <li class="error">{msg()}</li>}</Show>
      <For each={mounts()}>{(m) => <DirNode ctx={props.ctx} mount={m.name} path="" label={m.name} depth={0} />}</For>
      <Show when={pruned() > 0}>
        <li class="note">{hiddenNote(pruned(), mounts().length)}</li>
      </Show>
    </ul>
  );
}

// fileTreeIsland mounts the file+sheet tree and returns its SheetsView (which the presenter pushes
// the open file's sheets to, so they render nested under that file's node) plus a revealDir hook.
// revealDir asks the tree to expand to a folder that has no design under it — used when a folder
// URL is restored on load or back/forward. Framework reactivity stays in this leaf (CONSTRAINTS
// C11).
export function fileTreeIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: TreeHandlers,
): { island: SolidIsland; view: SheetsView; revealDir: (mount: string, path: string) => void } {
  const [sheets, setSheets] = signalView<SheetsState>({ mount: "", path: "", sheets: [], activeId: "" });
  const [revealTarget, setRevealTarget] = createSignal<RevealTarget>(null);
  const ctx: Ctx = { client: workspaceClient(), sheets, revealTarget, ...handlers };
  const island = new SolidIsland("file-tree", el, () => <FileTree ctx={ctx} />, eventBus);
  return { island, view: { setState: setSheets }, revealDir: (mount, path) => setRevealTarget({ mount, path }) };
}

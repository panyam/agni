import { createEffect, createSignal, For, Show, type Accessor } from "solid-js";
import { artifactUri, uriPath } from "@agni/web-shared/uri.js";
import { ConnectError, type Client } from "@connectrpc/connect";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import { FileKind, WorkspaceService, type DirEntry, type Mount } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import type { SheetRef } from "@panyam/agni-client/gen/agni/v1/webapi/design_view_pb";
import { type SheetNode as SheetTreeNode, sheetForest } from "./sheettree.js";
import { connectionClient, workspaceClient } from "./api.js";
import type { Connection } from "@panyam/agni-client/gen/agni/v1/webapi/connections_pb";
import { hiddenNote } from "@agni/web-shared/treeprune.js";
import type { SheetsState, SheetsView } from "./sheets.js";

// DESIGN_OPENS is what this tree passes as `opens`, one constant so the request and the view filter
// cannot drift: a tree that asked the server to prune by one kind and filtered rows by another would
// hide folders it then showed files from.
const DESIGN_OPENS = [FileKind.DESIGN];

type WsClient = Client<typeof WorkspaceService>;

// TreeHandlers are the tree's intents, fired when the user chooses a file, a folder, or a sheet
// within the open file. All carry semantic ids, never DOM events. A folder selection changes
// nothing on screen and only makes the folder the current URL, so a refresh reopens the tree
// expanded to it.
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
  // connections lists the server's connections, for the add-mount form's picker.
  connections: () => Promise<Connection[]>;
  client: WsClient;
  sheets: Accessor<SheetsState>;
  revealTarget: Accessor<RevealTarget>;
  // manage turns on adding a GitHub mount and removing a runtime one (agni issue 1749). Only the
  // /mounts/ browse page sets it, so the viewer's sidebar stays a plain tree.
  manage: boolean;
  reveal: (mount: string, path: string) => void;
}

// TreeOptions are the island's switches beyond its handlers.
export interface TreeOptions {
  manageMounts?: boolean;
}

// errorText is what a failed call says to a person: the server's own message for a Connect error,
// which names the flag or the ref that was wrong, and the error's text otherwise.
const errorText = (e: unknown): string => (e instanceof ConnectError ? e.rawMessage : String(e));

type SheetNodeData = SheetTreeNode<SheetRef>;

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

// revealsTarget reports whether this directory should open to reveal the reveal target, which is
// when the target folder is this directory or one of its descendants, so every ancestor down to the
// target expands. Unlike subtreeHasActive (a file, whose path never equals a dir path), an exact
// path match counts, so the target folder opens to show its own contents.
function revealsTarget(mount: string, dirPath: string, t: RevealTarget): boolean {
  if (!t || t.mount !== mount) return false;
  return dirPath === "" || t.path === dirPath || t.path.startsWith(dirPath + "/");
}

// DirNode is a directory (mount root or subdirectory). It lazily lists one level on expand, and
// auto-expands to reveal the deep-linked file when a restore makes a descendant the open file.
function DirNode(props: { ctx: Ctx; mount: string; path: string; label: string; depth: number; onRemove?: () => void }) {
  const [open, setOpen] = createSignal(false);
  const [entries, setEntries] = createSignal<DirEntry[] | null>(null);
  const [error, setError] = createSignal<string | null>(null);

  // loadEntries lists this directory's one level, once. Shared by manual toggle and auto-reveal.
  const loadEntries = async (): Promise<void> => {
    if (entries() !== null) return;
    try {
      // `opens` tells the server what this tree can open, so it can leave out subdirectories with
      // no design under them. That has to happen server-side, because a client sees one level per
      // call and cannot tell a folder of designs from a folder of empty folders without walking.
      const resp = await props.ctx.client.listDir({ uri: artifactUri(props.mount, props.path), opens: DESIGN_OPENS });
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
    <li classList={{ removable: !!props.onRemove }}>
      {/* The folder's button stays the li's direct child, since pages and specs find a folder as
          `li > button.node.dir`. The remove control sits over the row's right end instead. */}
      <button class="node dir" style={{ "padding-left": `${props.depth * 12 + 4}px` }} onClick={toggle}>
        <span class="twist">{open() ? "▾" : "▸"}</span> {props.label}
      </button>
      <Show when={props.onRemove}>
        {(remove) => (
          <button class="mount-rm" type="button" title={`Remove the mount ${props.label}`} aria-label={`Remove the mount ${props.label}`} onClick={() => remove()()}>
            ×
          </button>
        )}
      </Show>
      <Show when={open()}>
        <ul class="children">
          <Show when={error()}>{(msg) => <li class="error">{msg()}</li>}</Show>
          {/* Files this page cannot open (library files, lock files, sidecars) are hidden rather
              than greyed, because they drowned real designs. The filter reads the server's kind
              label rather than the format string, so the datasheets tree applies the same rule
              with the other kind. The server still lists them all, so this is a view filter. */}
          <For each={(entries() ?? []).filter((e) => e.isDir || e.kind === FileKind.DESIGN)}>
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

// AddMountForm makes a runtime mount over a GitHub repo. The server downloads the repo at the ref
// before it answers, so the form says it is working and stays disabled until then. A refusal shows
// the server's own message, such as a ref GitHub does not know.
function AddMountForm(props: { client: WsClient; connections: () => Promise<Connection[]>; onAdded: (name: string) => void }) {
  const [busy, setBusy] = createSignal(false);
  const [msg, setMsg] = createSignal("");
  // The server's connections, for a private repo (agni issue 1724). With none, the picker stays
  // hidden and every mount is a public one.
  const [conns, setConns] = createSignal<Connection[]>([]);
  void props
    .connections()
    .then(setConns)
    .catch(() => setConns([]));
  const submit = async (e: SubmitEvent): Promise<void> => {
    e.preventDefault();
    const form = e.currentTarget as HTMLFormElement;
    const field = (n: string) => ((form.elements.namedItem(n) as HTMLInputElement | null)?.value ?? "").trim();
    const name = field("name");
    const repo = field("repo")
      .replace(/^https?:\/\/github\.com\//, "")
      .replace(/^github:\/\//, "")
      .replace(/\.git$/, "");
    setBusy(true);
    setMsg(`Downloading ${repo}…`);
    try {
      const connection = field("connection");
      await props.client.createMount({ name, source: `github://${repo}`, ref: field("ref"), ...(connection ? { connection } : {}) });
      setMsg("");
      form.reset();
      props.onAdded(name);
    } catch (err) {
      setMsg(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <li class="mount-add">
      <details>
        <summary>Add a GitHub mount</summary>
        <form onSubmit={(e) => void submit(e)}>
          <input name="name" placeholder="name" required aria-label="Mount name" />
          <input name="repo" placeholder="owner/repo" required aria-label="GitHub repo" />
          <input name="ref" placeholder="ref (default branch)" aria-label="Tag, branch or commit" />
          <Show when={conns().length > 0}>
            <select name="connection" aria-label="Connection for a private repo">
              <option value="">public repo, no connection</option>
              <For each={conns()}>{(c) => <option value={c.name}>{c.displayName || c.name.replace(/^connections\//, "")}</option>}</For>
            </select>
          </Show>
          <button type="submit" disabled={busy()}>
            Add
          </button>
        </form>
        <Show when={msg()}>{(m) => <div class="mount-add-msg">{m()}</div>}</Show>
      </details>
    </li>
  );
}

function FileTree(props: { ctx: Ctx }) {
  const [mounts, setMounts] = createSignal<Mount[]>([]);
  const [pruned, setPruned] = createSignal(0);
  const [canCreate, setCanCreate] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  // The same declaration applies to the roots, since a mount serving only datasheets or only
  // library files is one this tree can never show anything in. The datasheets tree roots on the
  // same mounts and declares the other kind, so it keeps exactly the ones this drops.
  const load = (): Promise<void> =>
    props.ctx.client
      .listMounts({ opens: DESIGN_OPENS })
      .then((r) => {
        setMounts(r.mounts);
        setPruned(r.prunedMounts);
        setCanCreate(r.canCreateMounts);
        setError(null);
      })
      .catch((e) => {
        setError(String(e));
      });
  void load();

  // remove deletes a runtime mount after the person confirms, since its snapshot goes with it.
  const remove = async (name: string): Promise<void> => {
    if (!window.confirm(`Remove the mount ${name}? Its snapshot is deleted from this server.`)) return;
    try {
      await props.ctx.client.deleteMount({ name });
      await load();
    } catch (e) {
      setError(errorText(e));
    }
  };

  return (
    <ul class="tree">
      <Show when={error()}>{(msg) => <li class="error">{msg()}</li>}</Show>
      <For each={mounts()}>
        {(m) => (
          <DirNode
            ctx={props.ctx}
            mount={m.name}
            path=""
            label={m.name}
            depth={0}
            onRemove={props.ctx.manage && !m.fixed ? () => void remove(m.name) : undefined}
          />
        )}
      </For>
      <Show when={pruned() > 0}>
        <li class="note">{hiddenNote(pruned(), mounts().length, "designs")}</li>
      </Show>
      <Show when={props.ctx.manage && canCreate()}>
        <AddMountForm
          client={props.ctx.client}
          connections={props.ctx.connections}
          onAdded={(name) => {
            void load().then(() => props.ctx.reveal(name, ""));
          }}
        />
      </Show>
    </ul>
  );
}

// fileTreeIsland mounts the file+sheet tree and returns its SheetsView (which the presenter pushes
// the open file's sheets to, so they render nested under that file's node) plus a revealDir hook.
// revealDir asks the tree to expand to a folder that has no design under it, for a folder URL
// restored on load or back/forward. Framework reactivity stays in this leaf (CONSTRAINTS C11).
export function fileTreeIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  handlers: TreeHandlers,
  options: TreeOptions = {},
): { island: SolidIsland; view: SheetsView; revealDir: (mount: string, path: string) => void } {
  const [sheets, setSheets] = signalView<SheetsState>({ mount: "", path: "", sheets: [], activeId: "" });
  const [revealTarget, setRevealTarget] = createSignal<RevealTarget>(null);
  const reveal = (mount: string, path: string): void => {
    setRevealTarget({ mount, path });
  };
  const ctx: Ctx = {
    client: workspaceClient(),
    // The client is made on first use, since only the /mounts/ page's add form lists connections.
    connections: () =>
      connectionClient()
        .listConnections({})
        .then((r) => r.connections),
    sheets,
    revealTarget,
    manage: options.manageMounts ?? false,
    reveal,
    ...handlers,
  };
  const island = new SolidIsland("file-tree", el, () => <FileTree ctx={ctx} />, eventBus);
  return { island, view: { setState: setSheets }, revealDir: reveal };
}

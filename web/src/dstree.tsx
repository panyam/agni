import { createEffect, createSignal, For, Show, type Accessor } from "solid-js";
import { artifactUri, uriPath } from "./uri.js";
import type { Client } from "@connectrpc/connect";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland, signalView } from "@panyam/tsappkit-solid";
import { WorkspaceService, type DirEntry, type Mount } from "./gen/agni/v1/webapi/workspace_pb.js";
import { workspaceClient } from "./api.js";

type WsClient = Client<typeof WorkspaceService>;

// DsTreeState is the open datasheet the tree highlights (mount + mount-relative path), pushed by
// the boot code so a deep-link restore highlights and reveals the datasheet without a click.
export interface DsTreeState {
  mount: string;
  path: string;
}

// DsTreeView is the handle the boot code pushes the open datasheet to.
export interface DsTreeView {
  setState: (s: DsTreeState) => void;
}

// isDatasheet reports whether a file is a datasheet the workbench lists. WorkspaceService returns
// every file (PDFs come back with an empty format, since agni has no PDF *design* reader), so the
// datasheets tree filters by extension here, the way the viewer tree filters by reader format.
function isDatasheet(name: string): boolean {
  return name.toLowerCase().endsWith(".pdf");
}

interface Ctx {
  client: WsClient;
  active: Accessor<DsTreeState>;
  onSelect: (mount: string, path: string) => void;
}

// subtreeHasActive reports whether the open datasheet lives under this directory, so its ancestors
// auto-expand to reveal it. The mount root (path "") contains every file in its mount.
function subtreeHasActive(mount: string, dirPath: string, s: DsTreeState): boolean {
  if (s.mount !== mount || !s.path) return false;
  return dirPath === "" || s.path.startsWith(dirPath + "/");
}

function DatasheetNode(props: { ctx: Ctx; mount: string; entry: DirEntry; depth: number }) {
  const isOpen = (): boolean => props.ctx.active().mount === props.mount && props.ctx.active().path === uriPath(props.entry.uri);
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
        onClick={() => props.ctx.onSelect(props.mount, uriPath(props.entry.uri))}
      >
        <span class="twist" /> {props.entry.name} <span class="fmt">pdf</span>
      </button>
    </li>
  );
}

function DirNode(props: { ctx: Ctx; mount: string; path: string; label: string; depth: number }) {
  const [open, setOpen] = createSignal(false);
  const [entries, setEntries] = createSignal<DirEntry[] | null>(null);
  const [error, setError] = createSignal<string | null>(null);

  const loadEntries = async (): Promise<void> => {
    if (entries() !== null) return;
    try {
      const resp = await props.ctx.client.listDir({ uri: artifactUri(props.mount, props.path) });
      setEntries(resp.entries);
    } catch (e) {
      setError(String(e));
    }
  };

  const toggle = async (): Promise<void> => {
    const next = !open();
    setOpen(next);
    if (next) await loadEntries();
  };

  // Auto-reveal the branch that holds the open datasheet on a deep-link restore.
  createEffect(() => {
    if (subtreeHasActive(props.mount, props.path, props.ctx.active())) {
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
          {/* Only directories and datasheet (.pdf) files; every other listed file is hidden, the
              way the viewer tree hides files with no reader (filetree.tsx). */}
          <For each={(entries() ?? []).filter((e) => e.isDir || isDatasheet(e.name))}>
            {(e) =>
              e.isDir ? (
                <DirNode ctx={props.ctx} mount={props.mount} path={uriPath(e.uri)} label={e.name} depth={props.depth + 1} />
              ) : (
                <DatasheetNode ctx={props.ctx} mount={props.mount} entry={e} depth={props.depth + 1} />
              )
            }
          </For>
        </ul>
      </Show>
    </li>
  );
}

function DatasheetTree(props: { ctx: Ctx }) {
  const [mounts, setMounts] = createSignal<Mount[]>([]);
  const [error, setError] = createSignal<string | null>(null);
  props.ctx.client
    .listMounts({})
    .then((r) => setMounts(r.mounts))
    .catch((e) => setError(String(e)));

  return (
    <ul class="tree">
      <Show when={error()}>{(msg) => <li class="error">{msg()}</li>}</Show>
      <For each={mounts()}>{(m) => <DirNode ctx={props.ctx} mount={m.name} path="" label={m.name} depth={0} />}</For>
    </ul>
  );
}

// dsTreeIsland mounts the datasheet tree and returns its DsTreeView (which the boot pushes the open
// datasheet to, so the tree highlights and reveals it). onSelect is the user opening a datasheet.
export function dsTreeIsland(
  el: HTMLElement,
  eventBus: EventBus | null,
  onSelect: (mount: string, path: string) => void,
): { island: SolidIsland; view: DsTreeView } {
  const [active, setActive] = signalView<DsTreeState>({ mount: "", path: "" });
  const ctx: Ctx = { client: workspaceClient(), active, onSelect };
  const island = new SolidIsland("ds-tree", el, () => <DatasheetTree ctx={ctx} />, eventBus);
  return { island, view: { setState: setActive } };
}

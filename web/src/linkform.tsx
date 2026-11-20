// The designs page's link form (agni issue 1727). It links an artifact of a design to a file or
// folder in a mount at a ref, as `agni designs link` does. A person picks a mount, or adds one over a
// GitHub repo, then walks the mount's folders to the file, and names a tag, branch or commit. The
// server reads the files at that ref, keeps them, and records where they came from, so a later sync
// or a move to another ref reads them again.

import { createSignal, For, Show } from "solid-js";
import { create } from "@bufbuild/protobuf";
import { ConnectError } from "@connectrpc/connect";
import { artifactUri } from "@agni/web-shared/uri.js";
import { ArtifactSourceSchema, type Artifact } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import { FileKind, MountKind, type DirEntry, type Mount } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import { connectionClient, designClient, workspaceClient } from "./api.js";
import { AddMountForm } from "./mountform.js";

// LinkTarget is what a link reads: a path in a mount at a ref. An empty ref follows the mount's
// branch, and a tag or commit pins the artifact.
export interface LinkTarget {
  artifact: string;
  mount: string;
  path: string;
  ref: string;
}

// LinkClient is the part of DesignService a link calls.
export interface LinkClient {
  linkArtifact(req: { name: string; source: ReturnType<typeof create<typeof ArtifactSourceSchema>>; allowMissing: boolean }): Promise<Artifact>;
}

// ARTIFACT_ID is the server's pattern for an artifact id.
const ARTIFACT_ID = /^[a-z0-9][a-z0-9._-]*$/;

// artifactFor names the artifact a picked file most likely is, by its extension: a schematic, a
// board or a netlist. Any other file is named by its stem, lowercased to an id the server takes.
export function artifactFor(path: string): string {
  const file = path.slice(path.lastIndexOf("/") + 1);
  const lower = file.toLowerCase();
  if (/\.(kicad_sch|sch|schdoc|dsn)$/.test(lower)) return "schematic";
  if (/\.(kicad_pcb|brd|pcbdoc|tgz)$/.test(lower)) return "board";
  if (/\.(net|edif|edf|edn|cir|qcv)$/.test(lower)) return "netlist";
  const stem = lower.replace(/\.[^.]*$/, "").replace(/[^a-z0-9._-]+/g, "-").replace(/^[^a-z0-9]+/, "");
  return stem || "files";
}

// linkInto links target as an artifact of design, "designs/{design}", making the artifact when the
// design has none of that name.
export async function linkInto(c: LinkClient, design: string, t: LinkTarget): Promise<Artifact> {
  return c.linkArtifact({
    name: `${design}/artifacts/${t.artifact}`,
    source: create(ArtifactSourceSchema, { mount: t.mount, path: t.path, ref: t.ref }),
    allowMissing: true,
  });
}

// errorText is the server's own message for a Connect error, and the error's text otherwise.
const errorText = (e: unknown): string => ConnectError.from(e).rawMessage || String(e);

// parent is the folder above path in a mount, "" at its root.
function parent(path: string): string {
  const i = path.lastIndexOf("/");
  return i < 0 ? "" : path.slice(0, i);
}

// LinkFields are the controls that choose a target: the mount, a file in it, the ref and the
// artifact's id. It reports the target as the person changes it, and null while it is incomplete.
export function LinkFields(props: { onTarget: (t: LinkTarget | null) => void; artifact?: string }) {
  const ws = workspaceClient();
  const [mounts, setMounts] = createSignal<Mount[]>([]);
  const [canCreate, setCanCreate] = createSignal(false);
  const [signInUrl, setSignInUrl] = createSignal("");
  const [mount, setMount] = createSignal("");
  const [dir, setDir] = createSignal("");
  const [entries, setEntries] = createSignal<DirEntry[]>([]);
  const [picked, setPicked] = createSignal("");
  const [ref, setRef] = createSignal("");
  const [artifact, setArtifact] = createSignal(props.artifact ?? "");
  const [msg, setMsg] = createSignal("");

  const report = () => {
    const a = artifact().trim();
    props.onTarget(mount() && picked() && ARTIFACT_ID.test(a) ? { artifact: a, mount: mount(), path: picked(), ref: ref().trim() } : null);
  };
  const browse = (m: string, path: string) => {
    setDir(path);
    setMsg("");
    ws.listDir({ uri: artifactUri(m, path), opens: [FileKind.DESIGN] })
      .then((r) => setEntries(r.entries.filter((e) => e.isDir || e.kind === FileKind.DESIGN)))
      .catch((e) => {
        setEntries([]);
        setMsg(errorText(e));
      });
  };
  const choose = (name: string) => {
    setMount(name);
    setPicked("");
    // A mount over a repo has history, and its own ref is where a link starts unless the person
    // names another.
    setRef(mounts().find((m) => m.name === name)?.ref ?? "");
    if (name) browse(name, "");
    else setEntries([]);
    report();
  };
  const pick = (path: string) => {
    setPicked(path);
    if (!props.artifact) setArtifact(artifactFor(path));
    report();
  };
  const loadMounts = (select?: string) =>
    ws
      .listMounts({})
      .then((r) => {
        // Only a mount with history can be followed or moved to another ref, but a folder mount
        // links too, and a sync reads it again.
        setMounts(r.mounts.filter((m) => !m.sourceHidden && m.kind !== MountKind.UPLOADS));
        setCanCreate(r.canCreateMounts);
        setSignInUrl(r.signInUrl);
        if (select) choose(select);
      })
      .catch((e) => setMsg(errorText(e)));
  void loadMounts();

  return (
    <div class="lb-link" data-testid="link-fields">
      <label>
        Mount{" "}
        <select name="mount" value={mount()} onChange={(e) => choose(e.currentTarget.value)}>
          <option value="">choose a mount</option>
          <For each={mounts()}>{(m) => <option value={m.name}>{m.source ? `${m.name} (${m.source.replace(/^github:\/\//, "")})` : m.name}</option>}</For>
        </select>
      </label>
      <Show when={canCreate()}>
        <AddMountForm client={ws} connections={() => connectionClient().listConnections({}).then((r) => r.connections)} signInUrl={signInUrl()} onAdded={(name) => void loadMounts(name)} />
      </Show>
      <Show when={mount()}>
        <div class="lb-browse" data-testid="link-browse">
          <p class="lb-note">
            In {mount()}/{dir()}{" "}
            <Show when={dir()}>
              <button type="button" onClick={() => browse(mount(), parent(dir()))}>
                Up
              </button>
            </Show>
          </p>
          <ul>
            <For each={entries()}>
              {(e) => {
                const path = dir() ? `${dir()}/${e.name}` : e.name;
                return (
                  <li>
                    <button type="button" class={e.isDir ? "lb-dir" : picked() === path ? "lb-file lb-picked" : "lb-file"} data-path={path} onClick={() => (e.isDir ? browse(mount(), path) : pick(path))}>
                      {e.isDir ? `${e.name}/` : e.name}
                    </button>
                  </li>
                );
              }}
            </For>
          </ul>
        </div>
        <label>
          Ref{" "}
          <input
            name="link-ref"
            placeholder="tag, branch or commit; empty follows the mount's branch"
            value={ref()}
            onInput={(e) => {
              setRef(e.currentTarget.value);
              report();
            }}
          />
        </label>
        <Show when={!props.artifact}>
          <label>
            Artifact{" "}
            <input
              name="artifact"
              pattern="[a-z0-9][a-z0-9._\-]*"
              placeholder="schematic"
              value={artifact()}
              onInput={(e) => {
                setArtifact(e.currentTarget.value);
                report();
              }}
            />
          </label>
        </Show>
        <Show when={picked()}>
          <p class="lb-note" data-testid="link-picked">
            Links {picked()}
            {ref() ? `@${ref()}` : ", following the mount's branch"}
          </p>
        </Show>
      </Show>
      <Show when={msg()}>
        <p class="lb-status">{msg()}</p>
      </Show>
    </div>
  );
}

// LinkArtifact links one more artifact into a design from its page.
export function LinkArtifact(props: { design: string; onLinked: () => void }) {
  const [target, setTarget] = createSignal<LinkTarget | null>(null);
  const [busy, setBusy] = createSignal(false);
  const [status, setStatus] = createSignal("");
  // The fields load the mounts, so they appear only once a person opens the form.
  const [open, setOpen] = createSignal(false);
  const submit = async () => {
    const t = target();
    if (!t) return;
    setBusy(true);
    setStatus(`Reading ${t.mount}/${t.path}…`);
    try {
      await linkInto(designClient(), props.design, t);
      setStatus("");
      props.onLinked();
    } catch (err) {
      setStatus(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <details class="lb-link-more" data-testid="link-artifact" onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary>Link an artifact from a mount</summary>
      {/* No <form> here: LinkFields holds the add-mount form, and forms don't nest. */}
      <Show when={open()}>
        <LinkFields onTarget={setTarget} />
      </Show>
      <button type="button" class="lb-submit" disabled={busy() || !target()} onClick={() => void submit()}>
        Link
      </button>
      <p class="lb-status" data-testid="link-status">
        {status()}
      </p>
    </details>
  );
}

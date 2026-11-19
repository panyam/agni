// A mounted folder's designs, on the browse page (agni issue 1736). A folder in a mount is not a
// design until a person adds it. Choosing a folder lists the designs already read from it, and the
// boards the server proposes there that no design holds yet, each with a box to tick. "Add as
// designs" keeps the ticked ones (DesignService.AddDesigns), and the server names each for its
// folder. A box stands for a proposal's entry, since one folder can propose several boards.

import { createEffect, createSignal, For, Show, type Accessor } from "solid-js";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland } from "@panyam/tsappkit-solid";
import type { Design } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import type { ArtifactSource } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import type { ProposedDesign } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import { artifactUri } from "@agni/web-shared/uri.js";
import { designClient, workspaceClient } from "./api.js";
import { explain } from "./librarypanels.jsx";

// FolderTarget is the folder the panel shows, a mount and a path in it.
export interface FolderTarget {
  mount: string;
  path: string;
}

// unclaimed keeps the proposals no kept design reads: none of the sources reads from the proposal's
// folder in that mount.
export function unclaimed(proposals: ProposedDesign[], mount: string, sources: ArtifactSource[]): ProposedDesign[] {
  return proposals.filter((p) => !sources.some((s) => s.mount === mount && (s.path === p.folder || s.path.startsWith(p.folder + "/") || p.folder === "")));
}

// folderShown names a proposal's folder relative to the folder asked about, "." for that folder.
export function folderShown(folder: string, base: string): string {
  if (folder === base) return ".";
  return base && folder.startsWith(base + "/") ? folder.slice(base.length + 1) : folder;
}

// FolderData is what the panel loads for one folder.
interface FolderData {
  designs: Design[];
  proposals: ProposedDesign[];
}

async function load(t: FolderTarget): Promise<FolderData> {
  const uri = artifactUri(t.mount, t.path);
  const c = designClient();
  const designs: Design[] = [];
  let pageToken = "";
  for (let pages = 0; pages < 1000; pages++) {
    const r = await c.listDesigns({ pageToken, filter: `source = "${uri}"` });
    designs.push(...r.designs);
    pageToken = r.nextPageToken;
    if (!pageToken) break;
  }
  const sources: ArtifactSource[] = [];
  for (const d of designs) {
    const arts = await c.listArtifacts({ parent: d.name });
    for (const a of arts.artifacts) if (a.source) sources.push(a.source);
  }
  const proposed = await workspaceClient().proposeDesigns({ uri });
  return { designs, proposals: unclaimed(proposed.designs, t.mount, sources) };
}

function FolderDesigns(props: { target: Accessor<FolderTarget | null> }) {
  const [data, setData] = createSignal<FolderData | null>(null);
  const [picked, setPicked] = createSignal<ProposedDesign[]>([]);
  const [series, setSeries] = createSignal("");
  const [status, setStatus] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const reload = () => {
    const t = props.target();
    setData(null);
    setPicked([]);
    if (!t) return;
    setStatus("Looking for designs…");
    void load(t)
      .then((d) => {
        setData(d);
        setStatus("");
      })
      .catch((err) => setStatus(explain(err)));
  };
  createEffect(reload);
  const add = async () => {
    const t = props.target();
    if (!t || picked().length === 0) return;
    setBusy(true);
    setStatus("Adding…");
    try {
      await designClient().addDesigns({ uri: artifactUri(t.mount, t.path), designs: picked().map((p) => ({ folder: p.folder, entry: p.design?.entryUri ?? "", series: series() })) });
      reload();
    } catch (err) {
      setStatus(explain(err));
    } finally {
      setBusy(false);
    }
  };
  const toggle = (p: ProposedDesign, on: boolean) => setPicked(on ? [...picked(), p] : picked().filter((x) => x !== p));
  return (
    <Show when={props.target()}>
      {(t) => (
        <div class="br-folder" data-testid="folder-designs">
          <h2>
            {t().mount}/{t().path}
          </h2>
          <Show when={data()}>
            {(d) => (
              <>
                <h3>Designs from this folder</h3>
                <Show when={d().designs.length > 0} fallback={<p class="br-folder-note">None yet. A folder becomes a design only when you add it.</p>}>
                  <ul class="br-folder-list">
                    <For each={d().designs}>
                      {(x) => (
                        <li data-design={x.name}>
                          <a href={`/designs/${encodeURIComponent(x.name.replace(/^designs\//, ""))}`}>{x.title || x.name.replace(/^designs\//, "")}</a>
                        </li>
                      )}
                    </For>
                  </ul>
                </Show>
                <h3>Boards found here</h3>
                <Show when={d().proposals.length > 0} fallback={<p class="br-folder-note">No other boards in this folder.</p>}>
                  <ul class="br-folder-list">
                    <For each={d().proposals}>
                      {(p) => (
                        <li data-proposal={p.folder} data-entry={p.design?.entryUri}>
                          <label>
                            <input type="checkbox" name="proposal" value={p.design?.entryUri} checked={picked().includes(p)} onChange={(e) => toggle(p, e.currentTarget.checked)} /> {folderShown(p.folder, t().path)} <span class="br-folder-name">{p.design?.name}</span> <span class="br-folder-note">{p.files.length} files</span>
                          </label>
                          <Show when={p.note}>
                            <span class="br-folder-note"> {p.note}</span>
                          </Show>
                        </li>
                      )}
                    </For>
                  </ul>
                  <p>
                    <label>
                      Series <input name="series" value={series()} onInput={(e) => setSeries(e.currentTarget.value)} />
                    </label>{" "}
                    <button type="button" class="br-open" data-testid="add-designs" disabled={busy() || picked().length === 0} onClick={() => void add()}>
                      {picked().length ? `Add ${picked().length} as designs` : "Add as designs"}
                    </button>
                  </p>
                </Show>
              </>
            )}
          </Show>
          <p class="br-folder-note" data-testid="folder-status">
            {status()}
          </p>
        </div>
      )}
    </Show>
  );
}

// folderDesignsIsland mounts the panel, and show points it at a folder or at none.
export function folderDesignsIsland(el: HTMLElement, eventBus: EventBus | null): { island: SolidIsland; show: (t: FolderTarget | null) => void } {
  const [target, setTarget] = createSignal<FolderTarget | null>(null);
  const island = new SolidIsland("folder-designs", el, () => <FolderDesigns target={target} />, eventBus);
  return {
    island,
    show: (t) => {
      el.style.display = t ? "block" : "none";
      setTarget(t);
    },
  };
}

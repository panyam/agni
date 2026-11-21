// The compare picker's list of stored designs (agni issue 1774). The file tree beside it offers the
// mounted files, and a stored design or a revision of it lives in the uploads mount under a folder
// per revision, which no one browses for. So this lists the server's designs by series, and each
// one's revisions on request, and a pick resolves the design's entry to the same mount and path the
// tree would give.

import { createSignal, For, Show } from "solid-js";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland } from "@panyam/tsappkit-solid";
import { uriMount, uriPath } from "@agni/web-shared/uri.js";
import type { Design } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import type { DesignRevision } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import { designClient } from "./api.js";
import type { CompareTarget } from "./comparepicker.js";

// entryTarget is the mount and path a design's entry names, or null for a design no reader opens yet.
export function entryTarget(d: Design): CompareTarget | null {
  return d.entryUri ? { mount: uriMount(d.entryUri), path: uriPath(d.entryUri) } : null;
}

// revisionLabel names a revision by its refs, else its id, and marks the current one.
export function revisionLabel(r: DesignRevision): string {
  return (r.refs.length ? r.refs.join(", ") : r.revisionId) + (r.current ? " (current)" : "");
}

// listStored pages through the server's stored designs that a reader opens, by series then id. A
// server without --store answers with none.
async function listStored(): Promise<Design[]> {
  const c = designClient();
  const out: Design[] = [];
  let pageToken = "";
  for (let pages = 0; pages < 100; pages++) {
    const r = await c.listDesigns({ pageToken });
    out.push(...r.designs.filter((d) => !d.readOnly && d.entryUri));
    pageToken = r.nextPageToken;
    if (!pageToken) break;
  }
  return out.sort((a, b) => a.series.localeCompare(b.series) || a.name.localeCompare(b.name));
}

// Revisions lists one design's revisions once its row opens, each picking that revision's files.
function Revisions(props: { design: Design; onPick: (t: CompareTarget) => void }) {
  const [revisions, setRevisions] = createSignal<DesignRevision[] | null>(null);
  const [error, setError] = createSignal("");
  const load = () => {
    if (revisions() !== null) return;
    designClient()
      .listDesignRevisions({ parent: props.design.name })
      .then((r) => setRevisions(r.revisions))
      .catch((e) => {
        setRevisions([]);
        setError(String(e));
      });
  };
  const pick = async (r: DesignRevision) => {
    try {
      const t = entryTarget(await designClient().getDesign({ name: `${props.design.name}@${r.revisionId}` }));
      if (t) props.onPick(t);
    } catch (e) {
      setError(String(e));
    }
  };
  return (
    <details class="compare-revisions" onToggle={(e) => e.currentTarget.open && load()}>
      <summary>revisions</summary>
      <Show when={revisions()?.length === 0 && !error()}>
        <span class="note">none kept</span>
      </Show>
      <ul>
        <For each={revisions() ?? []}>
          {(r) => (
            <li>
              <button type="button" class="node file" data-revision={r.revisionId} onClick={() => void pick(r)}>
                {revisionLabel(r)}
              </button>
            </li>
          )}
        </For>
      </ul>
      <Show when={error()}>{(m) => <span class="error">{m()}</span>}</Show>
    </details>
  );
}

function CompareDesigns(props: { onPick: (t: CompareTarget) => void }) {
  const [designs, setDesigns] = createSignal<Design[]>([]);
  void listStored()
    .then(setDesigns)
    .catch(() => setDesigns([]));
  // The wrapper always renders, hidden while there is nothing to list, so the page's hole is filled
  // on a server that keeps no designs too.
  return (
    <div class="compare-designs-list" data-testid="compare-designs" hidden={designs().length === 0}>
      <Show when={designs().length > 0}>
        <div class="compare-designs-head">Stored designs</div>
        <ul>
          <For each={designs()}>
            {(d) => (
              <li data-design={d.name}>
                <button type="button" class="node file" onClick={() => entryTarget(d) && props.onPick(entryTarget(d) as CompareTarget)}>
                  {d.title || d.name.replace(/^designs\//, "")}
                  <Show when={d.series}>
                    <span class="note"> · {d.series}</span>
                  </Show>
                </button>
                <Revisions design={d} onPick={props.onPick} />
              </li>
            )}
          </For>
        </ul>
      </Show>
    </div>
  );
}

// compareDesignsIsland mounts the list in el, calling onPick with the chosen design's or revision's
// entry.
export function compareDesignsIsland(el: HTMLElement, eventBus: EventBus | null, onPick: (t: CompareTarget) => void): SolidIsland {
  return new SolidIsland("compare-designs", el, () => <CompareDesigns onPick={onPick} />, eventBus);
}

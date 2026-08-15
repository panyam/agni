// The landing page's two lists: what this browser opened lately, and what the server's projects
// declare. Both are the same shape (a heading over rows that leave for a work page), so they share
// a file rather than each taking one the way the viewer's panels do.
//
// Neither list is a file browser. The tree stays one click away behind the Designs and Datasheets
// cards, and these lists exist to skip it for the two cases where drilling down is pure cost: you
// were just here, or someone declared the design by name.

import { createSignal, For, Show } from "solid-js";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland } from "@panyam/tsappkit-solid";
import { emptyLocation, locationToUrl } from "./router.js";
import { dsToUrl } from "./dsrouter.js";
import { uriMount, uriPath } from "./uri.js";
import { clearRecents, loadRecents, type Recent, type RecentKind } from "./recents.js";
import { projectClient } from "./api.js";
import type { Design } from "./gen/agni/v1/webapi/project_pb.js";

// openUrl is where an artifact reopens, by kind. Both halves go through their page's own URL
// builder rather than assembling a path here, so this cannot drift from what those pages parse.
export function openUrl(kind: RecentKind, mount: string, path: string): string {
  return kind === "datasheet" ? dsToUrl({ mount, path }) : locationToUrl({ ...emptyLocation(), mount, path });
}

// ago words an age the way someone reading a list scans it. Coarse on purpose: the question is
// "was this today", never "was this 43 minutes ago", and a precise answer costs a re-render to
// stay true.
export function ago(then: number, now: number): string {
  const mins = Math.floor((now - then) / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return days === 1 ? "yesterday" : `${days}d ago`;
}

function RecentRow(props: { entry: Recent; now: number }) {
  return (
    <li class="ld-row">
      <a class="ld-link" href={openUrl(props.entry.kind, props.entry.mount, props.entry.path)}>
        <span class="ld-label">{props.entry.label}</span>
        <span class="ld-where">
          {props.entry.mount}/{props.entry.path}
        </span>
      </a>
      <span class="ld-kind">{props.entry.kind}</span>
      <span class="ld-when">{ago(props.entry.at, props.now)}</span>
    </li>
  );
}

function Recents(props: { now: number }) {
  const [entries, setEntries] = createSignal<Recent[]>(loadRecents());
  return (
    <section class="ld-section">
      <h2>
        Recent
        <Show when={entries().length > 0}>
          <button
            class="ld-clear"
            onClick={() => {
              clearRecents();
              setEntries([]);
            }}
          >
            clear
          </button>
        </Show>
      </h2>
      <Show
        when={entries().length > 0}
        fallback={<p class="ld-empty">Nothing opened yet. Start from Designs or Datasheets above.</p>}
      >
        <ul class="ld-list">
          <For each={entries()}>{(e) => <RecentRow entry={e} now={props.now} />}</For>
        </ul>
      </Show>
    </section>
  );
}

// ProjectGroup is one project and the designs declared under it.
interface ProjectGroup {
  title: string;
  designs: Design[];
}

// loadProjects fetches every project and its designs. A deployment with no descriptors is the
// ordinary case rather than an error (see project.proto), so the failure and the empty answer land
// in the same place: the section renders nothing at all.
async function loadProjects(): Promise<ProjectGroup[]> {
  const client = projectClient();
  const { projects } = await client.listProjects({});
  const groups = await Promise.all(
    projects.map(async (p) => ({
      title: p.title || p.name,
      designs: (await client.listDesigns({ parent: p.name })).designs,
    })),
  );
  return groups.filter((g) => g.designs.length > 0);
}

function Projects() {
  const [groups, setGroups] = createSignal<ProjectGroup[]>([]);
  void loadProjects()
    .then(setGroups)
    .catch(() => setGroups([]));

  return (
    <Show when={groups().length > 0}>
      <section class="ld-section">
        <h2>Projects</h2>
        <For each={groups()}>
          {(g) => (
            <div class="ld-project">
              <h3>{g.title}</h3>
              <ul class="ld-list">
                <For each={g.designs}>
                  {(d) => (
                    <li class="ld-row">
                      {/* A declared design opens its ENTRY file, the netlist analysis reads (C21),
                          never a companion view that happens to sort first in the folder. */}
                      <a class="ld-link" href={openUrl("design", uriMount(d.entryUri), uriPath(d.entryUri))}>
                        <span class="ld-label">{d.title || d.name}</span>
                        <span class="ld-where">{uriPath(d.entryUri)}</span>
                      </a>
                    </li>
                  )}
                </For>
              </ul>
            </div>
          )}
        </For>
      </section>
    </Show>
  );
}

// recentsIsland mounts the Recent list. `now` is passed in rather than read here so the page owns
// the one clock read and a test states the age it is asserting.
export function recentsIsland(el: HTMLElement, eventBus: EventBus | null, now: number): SolidIsland {
  return new SolidIsland("landing-recents", el, () => <Recents now={now} />, eventBus);
}

// projectsIsland mounts the declared-designs list, which renders nothing when the server has no
// project descriptors under its mounts.
export function projectsIsland(el: HTMLElement, eventBus: EventBus | null): SolidIsland {
  return new SolidIsland("landing-projects", el, () => <Projects />, eventBus);
}

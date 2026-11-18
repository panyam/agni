// The store's project and pack pages (agni issue 1697). /projects/ lists the projects a server holds
// and makes one, and /projects/<id> shows a project's designs and its default packs. /packs/ lists the
// packs and keeps one from a dropped project folder, and /packs/<id> shows what a pack holds and
// replaces it. The designs pages are in librarypanels.tsx.

import { createSignal, For, Show } from "solid-js";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland } from "@panyam/tsappkit-solid";
import type { Design, Project } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import type { Pack } from "@panyam/agni-client/gen/agni/v1/webapi/packs_pb";
import { packClient, projectClient, uploadsClient } from "./api.js";
import { DesignList, explain, FilePicker, listAll, listPacks, listProjects, StoreNav, when } from "./librarypanels.jsx";
import { designId, type Picked } from "./libraryupload.js";
import { importPack, lastSegment, packUrl, projectUrl } from "./storepages.js";

// Projects lists the projects a server holds, with the form to make one.
function Projects() {
  const [projects, setProjects] = createSignal<Project[]>([]);
  const [status, setStatus] = createSignal("Loading…");
  const [id, setId] = createSignal("");
  const [title, setTitle] = createSignal("");
  void listProjects()
    .then((ps) => {
      setProjects(ps);
      setStatus(ps.length ? "" : "No projects yet.");
    })
    .catch((err) => setStatus(explain(err)));
  const make = async () => {
    try {
      const p = await projectClient().createProject({ projectId: id(), project: { title: title() } });
      window.location.assign(projectUrl(lastSegment(p.name)));
    } catch (err) {
      setStatus(explain(err));
    }
  };
  return (
    <div class="lb-library">
      <StoreNav here="projects" />
      <p class="lb-tagline">A project lists designs and the packs a report on them applies first. A design can be in several projects.</p>
      <p class="lb-status" data-testid="projects-status">
        {status()}
      </p>
      <ul class="lb-list">
        <For each={projects()}>
          {(p) => (
            <li class="lb-row" data-project={p.name}>
              <a class="lb-link" href={projectUrl(lastSegment(p.name))}>
                <span class="lb-label">{p.title || lastSegment(p.name)}</span>
                <span class="lb-id">{lastSegment(p.name)}</span>
              </a>
              <span class="lb-when">
                {p.designs.length} designs
                <Show when={p.defaultPacks.length}> · packs {p.defaultPacks.map(lastSegment).join(", ")}</Show>
              </span>
            </li>
          )}
        </For>
      </ul>
      <form
        class="lb-new"
        data-testid="new-project"
        onSubmit={(ev) => {
          ev.preventDefault();
          void make();
        }}
      >
        <h2>New project</h2>
        <label>
          Id <input name="id" required pattern="[a-z0-9][a-z0-9._\-]*" placeholder="bench" value={id()} onInput={(e) => setId(e.currentTarget.value)} />
        </label>
        <label>
          Title <input name="title" placeholder="Bring-up bench" value={title()} onInput={(e) => setTitle(e.currentTarget.value)} />
        </label>
        <button type="submit" class="lb-submit" disabled={!id()}>
          Create
        </button>
      </form>
    </div>
  );
}

// ProjectPage shows one project: its designs, in the design list's form, and its default packs.
function ProjectPage(props: { id: string }) {
  const name = `projects/${props.id}`;
  const [project, setProject] = createSignal<Project | null>(null);
  const [designs, setDesigns] = createSignal<Design[]>([]);
  const [all, setAll] = createSignal<Design[]>([]);
  const [packs, setPacks] = createSignal<Pack[]>([]);
  const [status, setStatus] = createSignal("Loading…");
  const load = async () => {
    const p = await projectClient().getProject({ name });
    setProject(p);
    setDesigns((await listAll(`project = "${name}"`)).designs);
    setStatus("");
  };
  void load().catch((err) => setStatus(explain(err)));
  void listAll()
    .then((r) => setAll(r.designs))
    .catch(() => setAll([]));
  void listPacks()
    .then(setPacks)
    .catch(() => setPacks([]));
  const update = (patch: { designs?: string[]; defaultPacks?: string[] }, paths: string[]) => {
    setStatus("Saving…");
    void projectClient()
      .updateProject({ project: { name, ...patch }, updateMask: { paths } })
      .then(load)
      .catch((err) => setStatus(explain(err)));
  };
  const setDesignList = (names: string[]) => update({ designs: names }, ["designs"]);
  const setPackList = (names: string[]) => update({ defaultPacks: names }, ["default_packs"]);
  const outside = () => all().filter((d) => !project()?.designs.includes(d.name));
  const unused = () => packs().filter((k) => !project()?.defaultPacks.includes(k.name));
  return (
    <div class="lb-design" data-testid="project-page">
      <StoreNav here="projects" />
      <Show when={project()}>
        {(p) => (
          <>
            <h2 class="lb-title">{p().title || props.id}</h2>
            <p class="lb-meta">{name}</p>
            <h2>Default packs</h2>
            <p class="lb-note">A report that names this project applies these first, in this order.</p>
            <ol class="lb-packs" data-testid="project-packs">
              <For each={p().defaultPacks}>
                {(k, i) => (
                  <li data-pack={k}>
                    <a href={packUrl(lastSegment(k))}>{k}</a>{" "}
                    <Show when={i() > 0}>
                      <button type="button" onClick={() => setPackList(moveUp(p().defaultPacks, i()))}>
                        Up
                      </button>
                    </Show>{" "}
                    <button type="button" class="lb-delete" onClick={() => setPackList(p().defaultPacks.filter((x) => x !== k))}>
                      Remove
                    </button>
                  </li>
                )}
              </For>
            </ol>
            <Show when={unused().length > 0}>
              <AddOne label="Add a pack" testid="add-pack" options={unused().map((k) => ({ value: k.name, label: k.title || lastSegment(k.name) }))} onAdd={(k) => setPackList([...p().defaultPacks, k])} />
            </Show>
            <h2>Designs</h2>
            <Show when={designs().length > 0} fallback={<p class="lb-note">No designs in this project yet.</p>}>
              <DesignList
                designs={designs()}
                action={(d) => (
                  <button type="button" class="lb-delete" onClick={() => setDesignList(p().designs.filter((n) => n !== d.name))}>
                    Remove
                  </button>
                )}
              />
            </Show>
            <Show when={outside().length > 0}>
              <AddOne label="Add a design" testid="add-design" options={outside().map((d) => ({ value: d.name, label: d.title || designId(d.name) }))} onAdd={(d) => setDesignList([...p().designs, d])} />
            </Show>
          </>
        )}
      </Show>
      <p class="lb-status" data-testid="project-status">
        {status()}
      </p>
    </div>
  );
}

// moveUp swaps the item at i with the one before it.
export function moveUp(list: string[], i: number): string[] {
  const out = [...list];
  [out[i - 1], out[i]] = [out[i], out[i - 1]];
  return out;
}

// AddOne is a select and a button that adds the chosen option.
function AddOne(props: { label: string; testid: string; options: { value: string; label: string }[]; onAdd: (value: string) => void }) {
  const [chosen, setChosen] = createSignal("");
  return (
    <p class="lb-add" data-testid={props.testid}>
      <select value={chosen()} onChange={(e) => setChosen(e.currentTarget.value)}>
        <option value="">{props.label.toLowerCase()}…</option>
        <For each={props.options}>{(o) => <option value={o.value}>{o.label}</option>}</For>
      </select>{" "}
      <button type="button" disabled={!chosen()} onClick={() => props.onAdd(chosen())}>
        {props.label}
      </button>
    </p>
  );
}

// Packs lists the packs a server keeps, with the form that keeps one from a project folder.
function Packs() {
  const [packs, setPacks] = createSignal<Pack[]>([]);
  const [status, setStatus] = createSignal("Loading…");
  void listPacks()
    .then((ks) => {
      setPacks(ks);
      setStatus(ks.length ? "" : "No packs yet.");
    })
    .catch((err) => setStatus(explain(err)));
  return (
    <div class="lb-library">
      <StoreNav here="packs" />
      <p class="lb-tagline">A pack holds the rules a design is judged by, what a project.yaml holds: its naming convention, checklists, interface profiles and datasheet parameters.</p>
      <p class="lb-status" data-testid="packs-status">
        {status()}
      </p>
      <ul class="lb-list">
        <For each={packs()}>
          {(k) => (
            <li class="lb-row" data-pack={k.name}>
              <a class="lb-link" href={packUrl(lastSegment(k.name))}>
                <span class="lb-label">{k.title || lastSegment(k.name)}</span>
                <span class="lb-id">{lastSegment(k.name)}</span>
              </a>
              <span class="lb-when">
                {k.files.length} files · {when(k)}
              </span>
            </li>
          )}
        </For>
      </ul>
      <PackUpload label="New pack" onDone={(k) => window.location.assign(packUrl(lastSegment(k.name)))} />
    </div>
  );
}

// PackUpload keeps a pack from a dropped project folder. With no id, the project's name is the id.
function PackUpload(props: { label: string; id?: string; onDone: (k: Pack) => void }) {
  const [id, setId] = createSignal(props.id ?? "");
  const [picked, setPicked] = createSignal<Picked | null>(null);
  const [status, setStatus] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const submit = async () => {
    const files = picked();
    if (!files) return;
    setBusy(true);
    setStatus("Sending the project's rules…");
    try {
      props.onDone(await importPack(packClient(), uploadsClient(), { packId: id(), title: "" }, files));
    } catch (err) {
      setStatus(explain(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <form
      class="lb-new"
      data-testid="pack-upload"
      onSubmit={(ev) => {
        ev.preventDefault();
        void submit();
      }}
    >
      <h2>{props.label}</h2>
      <Show when={props.id === undefined}>
        <label>
          Id <input name="id" pattern="[a-z0-9][a-z0-9._\-]*" placeholder="the project's name" value={id()} onInput={(e) => setId(e.currentTarget.value)} />
        </label>
      </Show>
      <FilePicker label="Drop a project folder, with its project.yaml at the top, or" testid="pack-files" onPick={setPicked} />
      <Show when={picked()}>{(p) => <p class="lb-note">{Object.keys(p()).length} files picked; only the ones project.yaml names are sent</p>}</Show>
      <button type="submit" class="lb-submit" disabled={busy() || !picked()}>
        Keep the pack
      </button>
      <p class="lb-status" data-testid="pack-upload-status">
        {status()}
      </p>
    </form>
  );
}

// PackPage shows what one pack holds, and replaces or deletes it.
function PackPage(props: { id: string }) {
  const name = `packs/${props.id}`;
  const [pack, setPack] = createSignal<Pack | null>(null);
  const [status, setStatus] = createSignal("Loading…");
  void packClient()
    .getPack({ name })
    .then((k) => {
      setPack(k);
      setStatus("");
    })
    .catch((err) => setStatus(explain(err)));
  const tier = (label: string, uris: string[]) => (
    <Show when={uris.length > 0}>
      <tr>
        <th>{label}</th>
        <td>{uris.join(", ")}</td>
      </tr>
    </Show>
  );
  return (
    <div class="lb-design" data-testid="pack-page">
      <StoreNav here="packs" />
      <Show when={pack()}>
        {(k) => {
          const cfg = () => k().config;
          return (
            <>
              <h2 class="lb-title">{k().title || props.id}</h2>
              <p class="lb-meta">
                {name} · {k().hash.slice(0, 19)}… · updated {when(k())}
              </p>
              <table class="lb-artifacts" data-testid="pack-tiers">
                <tbody>
                  <Show when={cfg()?.conventions?.name}>
                    <tr>
                      <th>Convention</th>
                      <td>
                        {cfg()?.conventions?.name} ({cfg()?.conventions?.rules.length === 1 ? "1 rule" : `${cfg()?.conventions?.rules.length ?? 0} rules`})
                      </td>
                    </tr>
                  </Show>
                  <Show when={(cfg()?.checklists.length ?? 0) > 0}>
                    <tr>
                      <th>Checklists</th>
                      <td>{cfg()?.checklists.map((c) => c.name).join(", ")}</td>
                    </tr>
                  </Show>
                  {tier("Profiles", cfg()?.profileUris ?? [])}
                  {tier("Params", cfg()?.paramUris ?? [])}
                  {tier("Symbols", cfg()?.symbolPathUris ?? [])}
                  {tier("Library", cfg()?.libraryUris ?? [])}
                  {tier("Query sets", cfg()?.querySetUris ?? [])}
                  <Show when={cfg()?.intent}>
                    <tr>
                      <th>Intent</th>
                      <td>declared</td>
                    </tr>
                  </Show>
                </tbody>
              </table>
              <details>
                <summary>{k().files.length} files</summary>
                <ul class="lb-files">
                  <For each={k().files}>{(f) => <li>{f.path}</li>}</For>
                </ul>
              </details>
              <PackUpload
                label="Replace from a project folder"
                id={props.id}
                onDone={(next) => {
                  setPack(next);
                  setStatus("Replaced.");
                }}
              />
              <button
                type="button"
                class="lb-delete"
                onClick={() =>
                  void packClient()
                    .deletePack({ name })
                    .then(() => window.location.assign("/packs/"))
                    .catch((err) => setStatus(explain(err)))
                }
              >
                Delete pack
              </button>
            </>
          );
        }}
      </Show>
      <p class="lb-status" data-testid="pack-status">
        {status()}
      </p>
    </div>
  );
}

// projectsIsland, projectIsland, packsIsland and packIsland mount the four pages.
export function projectsIsland(el: HTMLElement, eventBus: EventBus | null): SolidIsland {
  return new SolidIsland("projects", el, () => <Projects />, eventBus);
}

export function projectIsland(el: HTMLElement, eventBus: EventBus | null, id: string): SolidIsland {
  return new SolidIsland("project", el, () => <ProjectPage id={id} />, eventBus);
}

export function packsIsland(el: HTMLElement, eventBus: EventBus | null): SolidIsland {
  return new SolidIsland("packs", el, () => <Packs />, eventBus);
}

export function packIsland(el: HTMLElement, eventBus: EventBus | null, id: string): SolidIsland {
  return new SolidIsland("pack", el, () => <PackPage id={id} />, eventBus);
}

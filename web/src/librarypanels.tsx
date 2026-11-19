// The designs page's two views (agni issue 1719). The list shows the designs a server holds, grouped
// by series with each series' latest marked, and a form that makes a design from dropped files. A
// design's own page lists its artifacts, and replaces or deletes one. Files go up as the CLI's
// `agni designs push` sends them (libraryupload.ts), and the server groups a folder into artifacts.

import { createMemo, createSignal, For, Show, type JSX } from "solid-js";
import { Code, ConnectError } from "@connectrpc/connect";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland } from "@panyam/tsappkit-solid";
import { filesFromDrop, filesFromFileList } from "@panyam/tsappkit/wasmhost";
import type { Design } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { ArtifactType, type Artifact, type Report } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import type { Pack } from "@panyam/agni-client/gen/agni/v1/webapi/packs_pb";
import type { Project } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { uriMount } from "@agni/web-shared/uri.js";
import { designClient, packClient, projectClient, uploadsClient } from "./api.js";
import { designViewUrl, diffUrl, filterDesigns, findingUrl, KEPT_HERE, lastSegment, packUrl, projectUrl, reportState, wheres } from "./storepages.js";
import { designId, groupBySeries, importFolder, libraryUrl, replaceArtifact, type DesignsClient, type Picked, type UploadsClient } from "./libraryupload.js";

// noStore words the answer of a server started without --store or --uploads.
const noStore = "This server keeps no designs of its own. Start it with --store and --uploads to create and upload designs here.";

// explain turns a failed call into a sentence for the status line.
export function explain(err: unknown): string {
  const ce = ConnectError.from(err);
  if (ce.code === Code.FailedPrecondition && /--(store|uploads)\b/.test(ce.rawMessage)) return noStore;
  return ce.rawMessage || String(err);
}

// typeWord is an artifact type as the page shows it.
export function typeWord(t: ArtifactType): string {
  switch (t) {
    case ArtifactType.NETLIST:
      return "netlist";
    case ArtifactType.SCHEMATIC:
      return "schematic";
    case ArtifactType.BOARD:
      return "board";
    default:
      return "other files";
  }
}

export function when(d: { updateTime?: { seconds: bigint } }): string {
  const s = d.updateTime?.seconds;
  return s === undefined ? "" : new Date(Number(s) * 1000).toLocaleString();
}

// viewerLink opens a design's entry in the viewer, by its mount path, or nothing for a design no
// reader can open yet.
function viewerLink(d: Design): JSX.Element {
  return (
    <Show when={d.entryUri} fallback={<span class="lb-note">nothing to open yet</span>}>
      <a class="lb-open" href={designViewUrl(d.name, [])}>
        Open in viewer
      </a>
    </Show>
  );
}

// listAll pages through every design the server holds, or those filter keeps.
export async function listAll(filter = ""): Promise<{ designs: Design[]; unreachable: string[] }> {
  const c = designClient();
  const designs: Design[] = [];
  const unreachable: string[] = [];
  let pageToken = "";
  for (let pages = 0; pages < 1000; pages++) {
    const r = await c.listDesigns({ pageToken, filter });
    designs.push(...r.designs);
    unreachable.push(...r.unreachable);
    pageToken = r.nextPageToken;
    if (!pageToken) break;
  }
  return { designs, unreachable };
}

// StoreNav links the store's three lists, marking the one shown.
export function StoreNav(props: { here: "designs" | "projects" | "packs" }) {
  const link = (k: "designs" | "projects" | "packs", label: string) => (
    <a class="lb-nav-link" classList={{ "lb-nav-here": props.here === k }} href={`/${k}/`}>
      {label}
    </a>
  );
  return (
    <nav class="lb-nav" data-testid="store-nav">
      {link("designs", "Designs")}
      {link("projects", "Projects")}
      {link("packs", "Packs")}
    </nav>
  );
}

// DesignList shows designs grouped by series with each series' latest marked. action, when given,
// adds a control to each row.
export function DesignList(props: { designs: Design[]; action?: (d: Design) => JSX.Element }) {
  return (
    <For each={groupBySeries(props.designs)}>
      {(g) => (
        <section class="lb-series" data-series={g.series}>
          <h2>{g.series ? `Series ${g.series}` : "No series"}</h2>
          <ul class="lb-list">
            <For each={g.designs}>
              {(d) => (
                <li class="lb-row" data-design={d.name}>
                  <a class="lb-link" href={libraryUrl(designId(d.name))}>
                    <span class="lb-label">{d.title || designId(d.name)}</span>
                    <span class="lb-id">{designId(d.name)}</span>
                  </a>
                  <Show when={d.seriesLatest}>
                    <span class="lb-latest">latest</span>
                  </Show>
                  <Show when={d.readOnly}>
                    <span class="lb-where">{uriMount(d.uri || d.entryUri)}</span>
                  </Show>
                  <span class="lb-when">{when(d)}</span>
                  {viewerLink(d)}
                  {props.action?.(d)}
                </li>
              )}
            </For>
          </ul>
        </section>
      )}
    </For>
  );
}

// FilePicker is a pair of buttons that pick files or a whole folder, and a drop zone, each handing
// what it read to onPick.
export function FilePicker(props: { label: string; onPick: (files: Picked) => void; testid: string }) {
  let files!: HTMLInputElement;
  let folder!: HTMLInputElement;
  const [over, setOver] = createSignal(false);
  const take = (read: Promise<Picked>) => void read.then(props.onPick);
  return (
    <div
      class="lb-drop"
      classList={{ "lb-drop-over": over() }}
      data-testid={props.testid}
      onDragOver={(ev) => {
        if (ev.dataTransfer?.types.includes("Files")) {
          ev.preventDefault();
          setOver(true);
        }
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(ev) => {
        if (!ev.dataTransfer?.types.includes("Files")) return;
        ev.preventDefault();
        setOver(false);
        take(filesFromDrop(ev.dataTransfer));
      }}
    >
      <span>{props.label}</span>
      <button type="button" class="lb-pick" onClick={() => files.click()}>
        Choose files
      </button>
      <button type="button" class="lb-pick" onClick={() => folder.click()}>
        Choose folder
      </button>
      <input
        ref={files}
        type="file"
        multiple
        hidden
        data-testid={props.testid + "-files"}
        onChange={() => {
          if (files.files?.length) take(filesFromFileList(files.files));
          files.value = "";
        }}
      />
      <input
        ref={folder}
        type="file"
        hidden
        data-testid={props.testid + "-folder"}
        {...{ webkitdirectory: true }}
        onChange={() => {
          if (folder.files?.length) take(filesFromFileList(folder.files));
          folder.value = "";
        }}
      />
    </div>
  );
}

// LibraryClient is the part of DesignService the new-design form calls.
export interface LibraryClient extends DesignsClient {
  createDesign(req: { designId: string; design: { title: string; series: string } }): Promise<Design>;
  deleteDesign(req: { name: string }): Promise<unknown>;
}

// createAndUpload makes a design and sends its files. A design it made and could not fill is
// deleted before the error is passed on, so a failed upload leaves nothing behind.
export async function createAndUpload(c: LibraryClient, uploads: UploadsClient, d: { id: string; title: string; series: string }, files: Picked, status: (s: string) => void = () => undefined): Promise<void> {
  status("Creating the design…");
  const made = await c.createDesign({ designId: d.id, design: { title: d.title, series: d.series } });
  try {
    status("Sending files…");
    await importFolder(c, uploads, made.name, files);
  } catch (err) {
    await c.deleteDesign({ name: made.name }).catch(() => undefined);
    throw err;
  }
}

// NewDesign makes a design and sends its files. A design it made and could not fill is deleted, so a
// failed upload leaves nothing behind.
function NewDesign(props: { onDone: (id: string) => void }) {
  const [id, setId] = createSignal("");
  const [title, setTitle] = createSignal("");
  const [series, setSeries] = createSignal("");
  const [picked, setPicked] = createSignal<Picked | null>(null);
  const [status, setStatus] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const submit = async () => {
    const files = picked();
    if (!id() || !files) return;
    setBusy(true);
    try {
      await createAndUpload(designClient(), uploadsClient(), { id: id(), title: title(), series: series() }, files, setStatus);
      props.onDone(id());
    } catch (err) {
      setStatus(explain(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <form
      class="lb-new"
      data-testid="new-design"
      onSubmit={(ev) => {
        ev.preventDefault();
        void submit();
      }}
    >
      <h2>New design</h2>
      <label>
        Id <input name="id" required pattern="[a-z0-9][a-z0-9._\-]*" placeholder="bb-1.1.3" value={id()} onInput={(e) => setId(e.currentTarget.value)} />
      </label>
      <label>
        Title <input name="title" placeholder="Baseboard rev 1.1.3" value={title()} onInput={(e) => setTitle(e.currentTarget.value)} />
      </label>
      <label>
        Series <input name="series" placeholder="baseboard" value={series()} onInput={(e) => setSeries(e.currentTarget.value)} />
      </label>
      <FilePicker label="Drop a design's folder here, or" testid="new-files" onPick={setPicked} />
      <Show when={picked()}>{(p) => <p class="lb-note">{Object.keys(p()).length} files picked</p>}</Show>
      <button type="submit" class="lb-submit" disabled={busy() || !id() || !picked()}>
        Create and upload
      </button>
      <p class="lb-status" data-testid="new-status">
        {status()}
      </p>
    </form>
  );
}

// ALL_WHERES is the filter value that keeps designs wherever they are kept.
const ALL_WHERES = "*";

// Library lists the server's designs by series, with a filter and the form to make one.
function Library() {
  const [designs, setDesigns] = createSignal<Design[]>([]);
  const [unreachable, setUnreachable] = createSignal<string[]>([]);
  const [status, setStatus] = createSignal("Loading…");
  const [text, setText] = createSignal("");
  const [where, setWhere] = createSignal(ALL_WHERES);
  void listAll()
    .then((r) => {
      setDesigns(r.designs);
      setUnreachable(r.unreachable);
      setStatus(r.designs.length ? "" : "No designs yet.");
    })
    .catch((err) => setStatus(explain(err)));
  const shown = createMemo(() => filterDesigns(designs(), { text: text(), where: where() === ALL_WHERES ? undefined : where() }));
  return (
    <div class="lb-library">
      <StoreNav here="designs" />
      <p class="lb-tagline">
        The designs this server holds, kept here or in a mounted folder. <a href="/mounts/">Browse the mounted folders</a> for a file that is not a design yet.
      </p>
      <div class="lb-filter" data-testid="design-filter">
        <input type="search" name="text" placeholder="Filter by id, title or series" value={text()} onInput={(e) => setText(e.currentTarget.value)} />
        <select name="where" value={where()} onChange={(e) => setWhere(e.currentTarget.value)}>
          <option value={ALL_WHERES}>Kept anywhere</option>
          <option value={KEPT_HERE}>Kept on this server</option>
          <For each={wheres(designs())}>{(m) => <option value={m}>In the mount {m}</option>}</For>
        </select>
      </div>
      <p class="lb-status" data-testid="library-status">
        {status()}
      </p>
      <Show when={designs().length > 0 && shown().length === 0}>
        <p class="lb-note">No design matches the filter.</p>
      </Show>
      <DesignList designs={shown()} />
      <Show when={unreachable().length > 0}>
        <p class="lb-note">Left out, as two mounted folders declare each: {unreachable().join(", ")}</p>
      </Show>
      <NewDesign onDone={(id) => window.location.assign(libraryUrl(id))} />
    </div>
  );
}

// DesignInfo edits a stored design's title and series.
function DesignInfo(props: { design: Design; onSaved: (d: Design) => void }) {
  const [title, setTitle] = createSignal(props.design.title);
  const [series, setSeries] = createSignal(props.design.series);
  const [status, setStatus] = createSignal("");
  const save = async () => {
    setStatus("Saving…");
    try {
      const d = await designClient().updateDesign({ design: { name: props.design.name, title: title(), series: series() }, updateMask: { paths: ["title", "series"] } });
      setStatus("Saved.");
      props.onSaved(d);
    } catch (err) {
      setStatus(explain(err));
    }
  };
  return (
    <form
      class="lb-info"
      data-testid="design-info"
      onSubmit={(ev) => {
        ev.preventDefault();
        void save();
      }}
    >
      <label>
        Title <input name="title" value={title()} onInput={(e) => setTitle(e.currentTarget.value)} />
      </label>
      <label>
        Series <input name="series" value={series()} onInput={(e) => setSeries(e.currentTarget.value)} />
      </label>
      <button type="submit">Save</button>
      <span class="lb-note">{status()}</span>
    </form>
  );
}

// DiffWith picks another design and opens the viewer comparing this one against it.
function DiffWith(props: { design: Design }) {
  const [others, setOthers] = createSignal<Design[]>([]);
  const [chosen, setChosen] = createSignal("");
  void listAll()
    .then((r) => setOthers(r.designs.filter((d) => d.name !== props.design.name && d.entryUri)))
    .catch(() => setOthers([]));
  const href = () => {
    const b = others().find((d) => d.name === chosen());
    return b ? diffUrl(props.design, b) : "";
  };
  return (
    <Show when={props.design.entryUri && others().length > 0}>
      <p class="lb-diff" data-testid="diff-with">
        Diff with{" "}
        <select name="against" value={chosen()} onChange={(e) => setChosen(e.currentTarget.value)}>
          <option value="">choose a design</option>
          <For each={others()}>{(d) => <option value={d.name}>{d.title || designId(d.name)}</option>}</For>
        </select>{" "}
        <Show when={href()}>
          <a class="lb-open" href={href()}>
            Open the diff
          </a>
        </Show>
      </p>
    </Show>
  );
}

// listReports pages through a design's reports, newest first.
async function listReports(parent: string): Promise<Report[]> {
  const out: Report[] = [];
  let pageToken = "";
  for (let pages = 0; pages < 1000; pages++) {
    const r = await designClient().listReports({ parent, pageToken });
    out.push(...r.reports);
    pageToken = r.nextPageToken;
    if (!pageToken) break;
  }
  return out;
}

// listPacks and listProjects page through what the server keeps, for the Validate form.
async function listPacks(): Promise<Pack[]> {
  const out: Pack[] = [];
  let pageToken = "";
  for (let pages = 0; pages < 1000; pages++) {
    const r = await packClient().listPacks({ pageToken });
    out.push(...r.packs);
    pageToken = r.nextPageToken;
    if (!pageToken) break;
  }
  return out;
}

export async function listProjects(): Promise<Project[]> {
  const out: Project[] = [];
  let pageToken = "";
  for (let pages = 0; pages < 1000; pages++) {
    const r = await projectClient().listProjects({ pageToken });
    out.push(...r.projects);
    pageToken = r.nextPageToken;
    if (!pageToken) break;
  }
  return out;
}

export { listPacks };

// ReportView shows one kept report: what it ran against, whether that changed since, and its findings
// grouped by rule, each rule folded open on its subjects.
function ReportView(props: { report: Report; onDelete: () => void }) {
  const findings = () => props.report.results?.findings ?? [];
  const groups = createMemo(() => findingsByRule(findings()));
  const design = () => props.report.name.replace(/\/reports\/.*$/, "");
  return (
    <div class="lb-report" data-testid="report" data-report={props.report.name} data-total={props.report.findingCount}>
      <p class="lb-meta">
        {props.report.packs.length ? "Packs " : "No pack"}
        <For each={props.report.packs}>
          {(k, i) => (
            <>
              {i() > 0 ? ", " : ""}
              <a href={packUrl(lastSegment(k))}>{k}</a>
            </>
          )}
        </For>
        <Show when={props.report.project}>
          {" · through "}
          <a href={projectUrl(lastSegment(props.report.project))}>{props.report.project}</a>
        </Show>
        {" · ran "}
        {props.report.createTime ? new Date(Number(props.report.createTime.seconds) * 1000).toLocaleString() : ""}
      </p>
      <p class="lb-state" classList={{ "lb-stale": reportState(props.report) !== "current" }} data-testid="report-state">
        {reportState(props.report)}
      </p>
      <p class="lb-note">
        <a class="lb-open" data-testid="report-view" href={designViewUrl(design(), props.report.packs)}>
          Open in the viewer under {props.report.packs.length ? "these packs" : "no pack"}
        </a>
        {" · "}
        {props.report.findingCount} findings from {groups().length} rules{" "}
        <button type="button" class="lb-delete" onClick={props.onDelete}>
          Delete report
        </button>
      </p>
      <div class="lb-findings">
        <For each={groups()}>
          {(g) => (
            <details class="lb-rule" data-rule={g.rule}>
              <summary>
                <span class={`lb-sev lb-sev-${g.severity}`}>{g.severity}</span> <span class="lb-label">{g.rule}</span> <span class="lb-id">{g.findings.length}</span>
              </summary>
              <ul>
                <For each={g.findings}>
                  {(f) => {
                    const href = findingUrl(design(), props.report.packs, f);
                    const subject = f.subject ? `${f.subject.kind} ${f.subject.ref}` : "design";
                    return (
                      <li>
                        <Show when={href} fallback={<span class="lb-subject" title="This rule states no verdicts, so the viewer has nothing to open it on.">{subject}</span>}>
                          <a class="lb-subject" href={href}>
                            {subject}
                          </a>
                        </Show>{" "}
                        {f.message}
                      </li>
                    );
                  }}
                </For>
              </ul>
            </details>
          )}
        </For>
      </div>
    </div>
  );
}

type Finding = NonNullable<Report["results"]>["findings"][number];

// severityRank orders a report's rules, errors first.
const severityRank: Record<string, number> = { error: 0, warning: 1, info: 2 };

// findingsByRule groups a report's findings by rule, errors first, then the rules with most findings.
export function findingsByRule(findings: Finding[]): { rule: string; severity: string; findings: Finding[] }[] {
  const by = new Map<string, Finding[]>();
  for (const f of findings) by.set(f.rule, [...(by.get(f.rule) ?? []), f]);
  return [...by.entries()]
    .map(([rule, fs]) => ({ rule, severity: fs[0].severity, findings: fs }))
    .sort((a, b) => (severityRank[a.severity] ?? 3) - (severityRank[b.severity] ?? 3) || b.findings.length - a.findings.length || a.rule.localeCompare(b.rule));
}

// Validate picks packs, and a project whose default packs apply first, and keeps a new report.
function Validate(props: { design: string; onMade: (r: Report) => void }) {
  const [packs, setPacks] = createSignal<Pack[]>([]);
  const [projects, setProjects] = createSignal<Project[]>([]);
  const [chosen, setChosen] = createSignal<string[]>([]);
  const [project, setProject] = createSignal("");
  const [status, setStatus] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const load = () => {
    void listPacks().then(setPacks).catch((err) => setStatus(explain(err)));
    void listProjects()
      .then((ps) => setProjects(ps.filter((p) => p.designs.includes(props.design) || p.defaultPacks.length > 0)))
      .catch(() => setProjects([]));
  };
  const toggle = (name: string, on: boolean) => setChosen(on ? [...chosen(), name] : chosen().filter((n) => n !== name));
  const run = async () => {
    setBusy(true);
    setStatus("Running the check…");
    try {
      const r = await designClient().createReport({ parent: props.design, project: project(), report: { packs: chosen() } });
      setStatus("");
      props.onMade(r);
    } catch (err) {
      setStatus(explain(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <details class="lb-validate" data-testid="validate" onToggle={(e) => e.currentTarget.open && load()}>
      <summary>Validate against…</summary>
      <Show when={packs().length > 0} fallback={<p class="lb-note">No packs on this server yet. <a href="/packs/">Add one</a>.</p>}>
        <fieldset>
          <legend>Packs, in the order they layer</legend>
          <For each={packs()}>
            {(k) => (
              <label>
                <input type="checkbox" name="pack" value={k.name} checked={chosen().includes(k.name)} onChange={(e) => toggle(k.name, e.currentTarget.checked)} /> {k.title || lastSegment(k.name)}{" "}
                <span class="lb-id">{k.name}</span>
              </label>
            )}
          </For>
        </fieldset>
      </Show>
      <label>
        Project{" "}
        <select name="project" value={project()} onChange={(e) => setProject(e.currentTarget.value)}>
          <option value="">none</option>
          <For each={projects()}>
            {(p) => (
              <option value={p.name}>
                {p.title || lastSegment(p.name)}
                {p.defaultPacks.length ? ` (applies ${p.defaultPacks.join(", ")} first)` : ""}
              </option>
            )}
          </For>
        </select>
      </label>
      <button type="button" class="lb-submit" disabled={busy()} onClick={() => void run()}>
        Run and keep the report
      </button>
      <p class="lb-status" data-testid="validate-status">
        {status()}
      </p>
    </details>
  );
}

// Reports shows a design's kept reports as tabs, newest first, with the form that runs a new one.
function Reports(props: { design: string }) {
  const [reports, setReports] = createSignal<Report[]>([]);
  const [selected, setSelected] = createSignal("");
  const [status, setStatus] = createSignal("");
  // full holds each report's whole run once its tab is opened. The listing leaves runs out, since a
  // run with every verdict runs to megabytes on a large board (agni issue 1753).
  const [full, setFull] = createSignal<Record<string, Report>>({});
  const open = (name: string) => {
    setSelected(name);
    if (!name || full()[name]) return;
    void designClient()
      .getReport({ name })
      .then((r) => setFull({ ...full(), [name]: r }))
      .catch((err) => setStatus(explain(err)));
  };
  const load = async (select?: string) => {
    const rs = await listReports(props.design);
    setReports(rs);
    if (select) open(select);
    else if (!rs.some((r) => r.name === selected())) open(rs[0]?.name ?? "");
  };
  void load().catch((err) => setStatus(explain(err)));
  const current = () => full()[selected()];
  return (
    <section class="lb-reports" data-testid="reports">
      <h2>Reports</h2>
      <Validate
        design={props.design}
        onMade={(r) => {
          setFull({ ...full(), [r.name]: r });
          void load(r.name).catch((err) => setStatus(explain(err)));
        }}
      />
      <Show when={reports().length > 0} fallback={<p class="lb-note">No reports yet.</p>}>
        <div class="lb-tabs" role="tablist">
          <For each={reports()}>
            {(r) => (
              <button type="button" role="tab" class="lb-tab" classList={{ "lb-tab-on": r.name === selected(), "lb-stale": reportState(r) !== "current" }} aria-selected={r.name === selected()} data-report={r.name} onClick={() => open(r.name)}>
                {lastSegment(r.name)}
              </button>
            )}
          </For>
        </div>
        <Show when={current()}>
          {(r) => (
            <ReportView
              report={r()}
              onDelete={() =>
                void designClient()
                  .deleteReport({ name: r().name })
                  .then(() => load())
                  .catch((err) => setStatus(explain(err)))
              }
            />
          )}
        </Show>
      </Show>
      <p class="lb-status">{status()}</p>
    </section>
  );
}

// DesignPage shows one design: its title and series, its artifacts, a diff with another design, and
// its reports. A design declared in a mounted folder has no artifacts and is read-only.
function DesignPage(props: { id: string }) {
  const name = `designs/${props.id}`;
  const [design, setDesign] = createSignal<Design | null>(null);
  const [artifacts, setArtifacts] = createSignal<Artifact[]>([]);
  const [status, setStatus] = createSignal("Loading…");
  const load = async () => {
    const c = designClient();
    const d = await c.getDesign({ name });
    const arts: Artifact[] = [];
    let pageToken = "";
    for (let pages = 0; pages < 1000; pages++) {
      const r = await c.listArtifacts({ parent: name, pageToken });
      arts.push(...r.artifacts);
      pageToken = r.nextPageToken;
      if (!pageToken) break;
    }
    setDesign(d);
    setArtifacts(arts);
    setStatus("");
  };
  const reload = () => void load().catch((err) => setStatus(explain(err)));
  reload();
  const act = (what: string, run: () => Promise<unknown>) => {
    setStatus(what);
    void run()
      .then(load)
      .catch((err) => setStatus(explain(err)));
  };
  const mounted = () => design()?.readOnly ?? false;
  return (
    <div class="lb-design" data-testid="design-page">
      <StoreNav here="designs" />
      <Show when={design()}>
        {(d) => (
          <>
            <h2 class="lb-title">{d().title || props.id}</h2>
            <p class="lb-meta">
              {designId(d().name)}
              <Show when={d().series}>
                {" · series "}
                {d().series}
                <Show when={d().seriesLatest}> (latest)</Show>
              </Show>
              {" · updated "}
              {when(d())}
            </p>
            <p>{viewerLink(d())}</p>
            <Show when={!d().readOnly}>
              <DesignInfo design={d()} onSaved={setDesign} />
            </Show>
            <DiffWith design={d()} />
          </>
        )}
      </Show>
      <Show
        when={!mounted()}
        fallback={<p class="lb-note">This design is declared in a mounted folder, so it is read-only here.</p>}
      >
        <table class="lb-artifacts" data-testid="artifacts">
          <thead>
            <tr>
              <th>Artifact</th>
              <th>Type</th>
              <th>Reads first</th>
              <th>Files</th>
              <th>Updated</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <For each={artifacts()}>
              {(a) => {
                const short = a.name.slice(a.name.lastIndexOf("/") + 1);
                return (
                  <tr data-artifact={short}>
                    <td>{short}</td>
                    <td>{typeWord(a.type)}</td>
                    <td>{a.entry || "-"}</td>
                    <td>{a.files.length}</td>
                    <td>{when(a)}</td>
                    <td>
                      <FilePicker
                        label="Replace with"
                        testid={`replace-${short}`}
                        onPick={(files) => act(`Replacing ${short}…`, () => replaceArtifact(designClient(), uploadsClient(), a.name, files))}
                      />
                      <button type="button" class="lb-delete" onClick={() => act(`Deleting ${short}…`, () => designClient().deleteArtifact({ name: a.name }))}>
                        Delete
                      </button>
                    </td>
                  </tr>
                );
              }}
            </For>
          </tbody>
        </table>
        <FilePicker
          label="Add a design's folder, grouped into artifacts:"
          testid="import-files"
          onPick={(files) => act("Sending files…", () => importFolder(designClient(), uploadsClient(), name, files))}
        />
      </Show>
      <p class="lb-status" data-testid="design-status">
        {status()}
      </p>
      <Show when={design()}>
        <Reports design={name} />
      </Show>
    </div>
  );
}

// libraryIsland mounts the design list and the new-design form.
export function libraryIsland(el: HTMLElement, eventBus: EventBus | null): SolidIsland {
  return new SolidIsland("library", el, () => <Library />, eventBus);
}

// designIsland mounts one design's page.
export function designIsland(el: HTMLElement, eventBus: EventBus | null, id: string): SolidIsland {
  return new SolidIsland("library-design", el, () => <DesignPage id={id} />, eventBus);
}

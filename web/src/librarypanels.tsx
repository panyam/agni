// The designs page's two views (agni issue 1719). The list shows the designs a server holds, grouped
// by series with each series' latest marked, and a form that makes a design from dropped files. A
// design's own page lists its artifacts, and replaces or deletes one. Files go up as the CLI's
// `agni designs push` sends them (libraryupload.ts), and the server groups a folder into artifacts.

import { createSignal, For, Show, type JSX } from "solid-js";
import { Code, ConnectError } from "@connectrpc/connect";
import type { EventBus } from "@panyam/tsappkit";
import { SolidIsland } from "@panyam/tsappkit-solid";
import { filesFromDrop, filesFromFileList } from "@panyam/tsappkit/wasmhost";
import type { Design } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { ArtifactType, type Artifact } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import { uriMount, uriPath } from "@agni/web-shared/uri.js";
import { designClient, uploadsClient } from "./api.js";
import { openUrl } from "./landingpanels.jsx";
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

function when(d: { updateTime?: { seconds: bigint } }): string {
  const s = d.updateTime?.seconds;
  return s === undefined ? "" : new Date(Number(s) * 1000).toLocaleString();
}

// viewerLink opens a design's entry in the viewer, by its mount path, or nothing for a design no
// reader can open yet.
function viewerLink(d: Design): JSX.Element {
  return (
    <Show when={d.entryUri} fallback={<span class="lb-note">nothing to open yet</span>}>
      <a class="lb-open" href={openUrl("design", uriMount(d.entryUri), uriPath(d.entryUri))}>
        Open in viewer
      </a>
    </Show>
  );
}

// listAll pages through every design the server holds.
async function listAll(): Promise<{ designs: Design[]; unreachable: string[] }> {
  const c = designClient();
  const designs: Design[] = [];
  const unreachable: string[] = [];
  let pageToken = "";
  for (let pages = 0; pages < 1000; pages++) {
    const r = await c.listDesigns({ pageToken });
    designs.push(...r.designs);
    unreachable.push(...r.unreachable);
    pageToken = r.nextPageToken;
    if (!pageToken) break;
  }
  return { designs, unreachable };
}

// FilePicker is a pair of buttons that pick files or a whole folder, and a drop zone, each handing
// what it read to onPick.
function FilePicker(props: { label: string; onPick: (files: Picked) => void; testid: string }) {
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

// Library lists the server's designs by series, with the form to make one.
function Library() {
  const [designs, setDesigns] = createSignal<Design[]>([]);
  const [unreachable, setUnreachable] = createSignal<string[]>([]);
  const [status, setStatus] = createSignal("Loading…");
  void listAll()
    .then((r) => {
      setDesigns(r.designs);
      setUnreachable(r.unreachable);
      setStatus(r.designs.length ? "" : "No designs yet.");
    })
    .catch((err) => setStatus(explain(err)));
  return (
    <div class="lb-library">
      <p class="lb-status" data-testid="library-status">
        {status()}
      </p>
      <For each={groupBySeries(designs())}>
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
                    <span class="lb-when">{when(d)}</span>
                    {viewerLink(d)}
                  </li>
                )}
              </For>
            </ul>
          </section>
        )}
      </For>
      <Show when={unreachable().length > 0}>
        <p class="lb-note">Left out, as two mounted folders declare each: {unreachable().join(", ")}</p>
      </Show>
      <NewDesign onDone={(id) => window.location.assign(libraryUrl(id))} />
    </div>
  );
}

// DesignPage shows one design and its artifacts. A design declared in a mounted folder has no
// artifacts and is read-only.
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
  const mounted = () => {
    const d = design();
    return d !== null && artifacts().length === 0 && d.entryUri !== "";
  };
  return (
    <div class="lb-design" data-testid="design-page">
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

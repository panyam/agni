// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { DesignSchema } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { ArtifactSchema, ArtifactType, DesignRevisionSchema, ReportSchema } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import { FileKind, MountKind, MountSchema } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import { artifactUri } from "@agni/web-shared/uri.js";
import { artifactFor } from "./linkform.jsx";
import { PackSchema } from "@panyam/agni-client/gen/agni/v1/webapi/packs_pb";
import { createAndLink, createAndUpload, designIsland, explain, findingsByRule, libraryIsland } from "./librarypanels.jsx";

const fake = vi.hoisted(() => ({
  designs: [] as unknown[],
  artifacts: [] as unknown[],
  design: undefined as unknown,
  listError: undefined as unknown,
  reports: [] as unknown[],
  packs: [] as unknown[],
  created: [] as unknown[],
  synced: [] as unknown[],
  linked: [] as unknown[],
  unlinked: [] as unknown[],
  madeDesigns: [] as unknown[],
  deletedDesigns: [] as unknown[],
  linkError: undefined as unknown,
  revisions: [] as unknown[],
  mounts: [] as unknown[],
  dirs: {} as Record<string, unknown[]>,
}));
vi.mock("./api.js", () => ({
  designClient: () => ({
    listDesigns: async () => {
      if (fake.listError) throw fake.listError;
      return { designs: fake.designs, unreachable: [], nextPageToken: "" };
    },
    getDesign: async () => fake.design,
    syncDesign: async (req: unknown) => {
      fake.synced.push(req);
      return { artifacts: [] };
    },
    listArtifacts: async () => ({ artifacts: fake.artifacts, nextPageToken: "" }),
    listDesignRevisions: async () => ({ revisions: fake.revisions, nextPageToken: "" }),
    createDesign: async (req: { designId: string }) => {
      fake.madeDesigns.push(req);
      return create(DesignSchema, { name: `designs/${req.designId}` });
    },
    deleteDesign: async (req: unknown) => {
      fake.deletedDesigns.push(req);
      return {};
    },
    linkArtifact: async (req: { name: string; source: { mount: string; path: string; ref: string }; allowMissing: boolean }) => {
      if (fake.linkError) throw fake.linkError;
      fake.linked.push({ name: req.name, mount: req.source.mount, path: req.source.path, ref: req.source.ref, allowMissing: req.allowMissing });
      return create(ArtifactSchema, { name: req.name });
    },
    unlinkArtifact: async (req: unknown) => {
      fake.unlinked.push(req);
      return create(ArtifactSchema, {});
    },
    listReports: async () => ({ reports: (fake.reports as { results?: unknown }[]).map((r) => ({ ...r, results: undefined })), nextPageToken: "" }),
    getReport: async (req: { name: string }) => (fake.reports as { name: string }[]).find((r) => r.name === req.name),
    createReport: async (req: unknown) => {
      fake.created.push(req);
      const made = create(ReportSchema, { name: "designs/bb/reports/new", packs: ["packs/house"] });
      fake.reports = [made, ...fake.reports];
      return made;
    },
  }),
  packClient: () => ({ listPacks: async () => ({ packs: fake.packs, nextPageToken: "" }) }),
  projectClient: () => ({ listProjects: async () => ({ projects: [], nextPageToken: "" }) }),
  uploadsClient: () => ({}),
  workspaceClient: () => ({
    listMounts: async () => ({ mounts: fake.mounts, prunedMounts: 0, canCreateMounts: false, signInUrl: "" }),
    listDir: async (req: { uri: string }) => ({ entries: fake.dirs[req.uri] ?? [] }),
  }),
  connectionClient: () => ({ listConnections: async () => ({ connections: [] }) }),
}));

function mount(island: (el: HTMLElement) => { activate: () => void }) {
  const el = document.createElement("div");
  document.body.appendChild(el);
  island(el).activate();
  return el;
}

beforeEach(() => {
  document.body.replaceChildren();
  fake.designs = [];
  fake.artifacts = [];
  fake.design = undefined;
  fake.listError = undefined;
  fake.reports = [];
  fake.packs = [];
  fake.created = [];
  fake.synced = [];
  fake.linked = [];
  fake.unlinked = [];
  fake.madeDesigns = [];
  fake.deletedDesigns = [];
  fake.linkError = undefined;
  fake.revisions = [];
  fake.mounts = [];
  fake.dirs = {};
});

describe("the library list", () => {
  it("groups designs by series and marks each series' latest", async () => {
    fake.designs = [
      create(DesignSchema, { name: "designs/bb-1.1.2", title: "Rev 1.1.2", series: "bb", entryUri: "mount://store/bb-1.1.2/schematic/bb.kicad_sch" }),
      create(DesignSchema, { name: "designs/bb-1.1.3", title: "Rev 1.1.3", series: "bb", seriesLatest: true, entryUri: "mount://store/bb-1.1.3/schematic/bb.kicad_sch" }),
      create(DesignSchema, { name: "designs/draft", title: "" }),
    ];
    const el = mount((e) => libraryIsland(e, null));
    await vi.waitFor(() => expect(el.querySelectorAll(".lb-row").length).toBe(3));
    const series = [...el.querySelectorAll(".lb-series")].map((s) => (s as HTMLElement).dataset.series);
    expect(series).toEqual(["bb", ""]);
    const latest = [...el.querySelectorAll(".lb-row")].filter((r) => r.querySelector(".lb-latest")).map((r) => (r as HTMLElement).dataset.design);
    expect(latest).toEqual(["designs/bb-1.1.3"]);
    expect(el.querySelector('[data-design="designs/bb-1.1.3"] a.lb-link')?.getAttribute("href")).toBe("/designs/bb-1.1.3");
    expect(el.querySelector('[data-design="designs/bb-1.1.3"] a.lb-open')?.getAttribute("href")).toBe("/designs/bb-1.1.3/view");
    expect(el.querySelector('[data-design="designs/draft"] a.lb-open')).toBeNull();
  });

  it("says when the server keeps no designs", () => {
    const err = new ConnectError("this server keeps no designs; start it with --store", Code.FailedPrecondition);
    expect(explain(err)).toMatch(/Start it with --store and --uploads/);
    expect(explain(new ConnectError("no design designs/x", Code.NotFound))).toBe("no design designs/x");
  });
});

describe("a design's page", () => {
  it("lists a stored design's artifacts with their types", async () => {
    fake.design = create(DesignSchema, { name: "designs/bb", title: "Baseboard", entryUri: "mount://store/bb/schematic/bb.kicad_sch", artifacts: ["designs/bb/artifacts/board", "designs/bb/artifacts/schematic"] });
    fake.artifacts = [
      create(ArtifactSchema, { name: "designs/bb/artifacts/board", type: ArtifactType.BOARD, entry: "bb.kicad_pcb", files: [{ path: "bb.kicad_pcb" }] }),
      create(ArtifactSchema, { name: "designs/bb/artifacts/schematic", type: ArtifactType.SCHEMATIC, entry: "bb.kicad_sch", files: [{ path: "bb.kicad_sch" }, { path: "power.kicad_sch" }] }),
    ];
    const el = mount((e) => designIsland(e, null, "bb"));
    await vi.waitFor(() => expect(el.querySelectorAll("tr[data-artifact]").length).toBe(2));
    const rows = [...el.querySelectorAll("tr[data-artifact]")].map((r) => [...r.querySelectorAll("td")].slice(0, 4).map((td) => td.textContent));
    expect(rows).toEqual([
      ["board", "board", "bb.kicad_pcb", "1"],
      ["schematic", "schematic", "bb.kicad_sch", "2"],
    ]);
    expect(el.querySelector(".lb-title")?.textContent).toBe("Baseboard");
  });

  it("names each artifact's source, and offers a sync when a newer version waits at it", async () => {
    fake.design = create(DesignSchema, { name: "designs/bb", artifacts: ["designs/bb/artifacts/notes", "designs/bb/artifacts/schematic"] });
    fake.artifacts = [
      create(ArtifactSchema, { name: "designs/bb/artifacts/notes", type: ArtifactType.OTHER, files: [{ path: "notes.md" }], source: { mount: "uploads" } }),
      create(ArtifactSchema, {
        name: "designs/bb/artifacts/schematic",
        type: ArtifactType.SCHEMATIC,
        entry: "bb.kicad_sch",
        files: [{ path: "bb.kicad_sch" }],
        source: { mount: "board", path: "hw/bb.kicad_sch", ref: "main", follow: true, version: "3411086e6d6ad2384d0609c53cd6b96323f97c24", readHash: "sha256:ab", newerVersion: "8e087f4f946a925041380699dcde51ae8519f621" },
      }),
    ];
    const el = mount((e) => designIsland(e, null, "bb"));
    await vi.waitFor(() => expect(el.querySelectorAll("tr[data-artifact]").length).toBe(2));
    const source = (id: string) => el.querySelector(`tr[data-artifact="${id}"] .lb-source`)?.textContent;
    expect(source("notes")).toBe("uploaded");
    expect(source("schematic")).toBe("board/hw/bb.kicad_sch@main at 3411086e6d6a, followed newer at the source: 8e087f4f946a Sync Move Unlink");
    expect(el.querySelector('[data-testid="sync-notes"]')).toBeNull();
    (el.querySelector('[data-testid="sync-schematic"]') as HTMLButtonElement).click();
    await vi.waitFor(() => expect(fake.synced).toEqual([{ name: "designs/bb", artifacts: ["schematic"] }]));
  });

  it("shows a mounted design as read-only", async () => {
    fake.design = create(DesignSchema, { name: "designs/gateway", entryUri: "mount://tut/designs/gateway/gateway.edn", readOnly: true });
    const el = mount((e) => designIsland(e, null, "gateway"));
    await vi.waitFor(() => expect(el.textContent).toMatch(/declared in a mounted folder/));
    expect(el.querySelector('[data-testid="artifacts"]')).toBeNull();
    expect(el.querySelector('[data-testid="import-files"]')).toBeNull();
  });
});

// set changes a control as a person would, and tells Solid.
function set(el: Element | null, value: string, event = "input") {
  const c = el as HTMLInputElement | HTMLSelectElement;
  c.value = value;
  c.dispatchEvent(new Event(event, { bubbles: true }));
}

describe("linking a design to a mount (agni issue 1727)", () => {
  const linkedSchematic = (source: Record<string, unknown>) =>
    create(ArtifactSchema, { name: "designs/bb/artifacts/schematic", type: ArtifactType.SCHEMATIC, entry: "bb.kicad_sch", files: [{ path: "bb.kicad_sch" }], source: { mount: "board", path: "hw/bb.kicad_sch", readHash: "sha256:ab", ...source } });

  it("moves a pinned artifact to another ref, unlinks it, and says who it opens for", async () => {
    fake.design = create(DesignSchema, { name: "designs/bb" });
    fake.artifacts = [linkedSchematic({ ref: "1.1.2", version: "3411086e6d6ad2384d0609c53cd6b96323f97c24", accessSource: "github://acme/board" })];
    const el = mount((e) => designIsland(e, null, "bb"));
    await vi.waitFor(() => expect(el.querySelector('[data-testid="move-schematic"]')).not.toBeNull());
    expect(el.querySelector('tr[data-artifact="schematic"] .lb-access')?.textContent).toBe(" · opens for who can read github://acme/board");
    // A pinned artifact with nothing newer has nothing to sync.
    expect(el.querySelector('[data-testid="sync-schematic"]')).toBeNull();
    const move = el.querySelector('[data-testid="move-schematic"]') as HTMLButtonElement;
    expect(move.disabled).toBe(true);
    set(el.querySelector('[data-testid="ref-schematic"]'), " 1.1.3 ");
    await vi.waitFor(() => expect(move.disabled).toBe(false));
    move.click();
    await vi.waitFor(() => expect(fake.linked).toEqual([{ name: "designs/bb/artifacts/schematic", mount: "board", path: "hw/bb.kicad_sch", ref: "1.1.3", allowMissing: false }]));
    (el.querySelector('[data-testid="unlink-schematic"]') as HTMLButtonElement).click();
    await vi.waitFor(() => expect(fake.unlinked).toEqual([{ name: "designs/bb/artifacts/schematic" }]));
  });

  it("offers a sync on a followed artifact, and no link controls on an uploaded one", async () => {
    fake.design = create(DesignSchema, { name: "designs/bb" });
    fake.artifacts = [create(ArtifactSchema, { name: "designs/bb/artifacts/notes", files: [{ path: "notes.md" }], source: { mount: "uploads" } }), linkedSchematic({ ref: "main", follow: true })];
    const el = mount((e) => designIsland(e, null, "bb"));
    await vi.waitFor(() => expect(el.querySelector('[data-testid="sync-schematic"]')).not.toBeNull());
    expect(el.querySelector('tr[data-artifact="notes"] .lb-link-controls')).toBeNull();
    expect(el.querySelector('tr[data-artifact="schematic"] .lb-access')).toBeNull();
    (el.querySelector('[data-testid="sync-schematic"]') as HTMLButtonElement).click();
    await vi.waitFor(() => expect(fake.synced).toEqual([{ name: "designs/bb", artifacts: ["schematic"] }]));
  });

  it("lists the design's revisions, each opening in the viewer", async () => {
    fake.design = create(DesignSchema, { name: "designs/bb" });
    fake.revisions = [
      create(DesignRevisionSchema, { name: "designs/bb@8e087f4f946a", revisionId: "8e087f4f946a", refs: ["1.1.3"], current: true }),
      create(DesignRevisionSchema, { name: "designs/bb@3411086e6d6a", revisionId: "3411086e6d6a", refs: [] }),
    ];
    const el = mount((e) => designIsland(e, null, "bb"));
    await vi.waitFor(() => expect(el.querySelectorAll("tr[data-revision]").length).toBe(2));
    const rows = [...el.querySelectorAll("tr[data-revision]")].map((r) => [(r as HTMLElement).dataset.revision, r.querySelector("td")?.textContent, r.querySelector("a")?.getAttribute("href")]);
    expect(rows).toEqual([
      ["1.1.3", "1.1.3", "/designs/bb%401.1.3/view"],
      ["3411086e6d6a", "3411086e6d6a", "/designs/bb%403411086e6d6a/view"],
    ]);
    expect(el.querySelector('tr[data-revision="1.1.3"]')?.textContent).toMatch(/current/);
  });

  it("makes a design by linking a file it walks to in a mount, at the mount's ref", async () => {
    fake.mounts = [create(MountSchema, { name: "uploads", kind: MountKind.UPLOADS }), create(MountSchema, { name: "board", kind: MountKind.GITHUB, source: "github://acme/board", ref: "1.1.2" })];
    fake.dirs = {
      [artifactUri("board", "")]: [{ name: "hw", isDir: true }, { name: "README.md", isDir: false, kind: FileKind.UNSPECIFIED }],
      [artifactUri("board", "hw")]: [{ name: "bb.kicad_sch", isDir: false, kind: FileKind.DESIGN }],
    };
    const el = mount((e) => libraryIsland(e, null));
    const form = () => el.querySelector('[data-testid="new-design"]') as HTMLElement;
    await vi.waitFor(() => expect(form()).not.toBeNull());
    set(form().querySelector('input[name="id"]'), "bb");
    const link = form().querySelector('input[name="from"][value="link"]') as HTMLInputElement;
    link.checked = true;
    link.dispatchEvent(new Event("change", { bubbles: true }));
    await vi.waitFor(() => expect(form().querySelectorAll('select[name="mount"] option').length).toBe(2));
    // The uploads mount holds uploads, not sources, so it isn't offered.
    expect([...form().querySelectorAll('select[name="mount"] option')].map((o) => o.textContent)).toEqual(["choose a mount", "board (acme/board)"]);
    set(form().querySelector('select[name="mount"]'), "board", "change");
    await vi.waitFor(() => expect(form().querySelector('button[data-path="hw"]')).not.toBeNull());
    expect(form().querySelector('button[data-path="README.md"]')).toBeNull();
    expect((form().querySelector('input[name="ref"]') as HTMLInputElement).value).toBe("1.1.2");
    const submit = form().querySelector("button.lb-submit") as HTMLButtonElement;
    expect(submit.textContent).toBe("Create and link");
    expect(submit.disabled).toBe(true);
    (form().querySelector('button[data-path="hw"]') as HTMLButtonElement).click();
    await vi.waitFor(() => expect(form().querySelector('button[data-path="hw/bb.kicad_sch"]')).not.toBeNull());
    (form().querySelector('button[data-path="hw/bb.kicad_sch"]') as HTMLButtonElement).click();
    await vi.waitFor(() => expect(submit.disabled).toBe(false));
    expect((form().querySelector('input[name="artifact"]') as HTMLInputElement).value).toBe("schematic");
    expect(form().querySelector('[data-testid="link-picked"]')?.textContent).toBe("Links hw/bb.kicad_sch@1.1.2");
    submit.click();
    await vi.waitFor(() => expect(fake.linked).toHaveLength(1));
    expect(fake.madeDesigns).toEqual([{ designId: "bb", design: { title: "", series: "" } }]);
    expect(fake.linked).toEqual([{ name: "designs/bb/artifacts/schematic", mount: "board", path: "hw/bb.kicad_sch", ref: "1.1.2", allowMissing: true }]);
  });

  it("deletes a design it made when the link fails, and passes the error on", async () => {
    fake.linkError = new ConnectError("GitHub has no ref 9.9", Code.NotFound);
    const c = (await import("./api.js")).designClient() as unknown as Parameters<typeof createAndLink>[0];
    await expect(createAndLink(c, { id: "bb", title: "", series: "" }, { artifact: "schematic", mount: "board", path: "hw/bb.kicad_sch", ref: "9.9" })).rejects.toThrow(/no ref 9.9/);
    expect(fake.deletedDesigns).toEqual([{ name: "designs/bb" }]);
  });

  it("names a picked file's artifact by its kind", () => {
    expect(artifactFor("hw/bb.kicad_sch")).toBe("schematic");
    expect(artifactFor("hw/bb.kicad_pcb")).toBe("board");
    expect(artifactFor("out/bb.NET")).toBe("netlist");
    expect(artifactFor("docs/Bill Of Materials.csv")).toBe("bill-of-materials");
  });
});

describe("the library's filter", () => {
  it("offers no choice of where when every design is kept on the server", async () => {
    fake.designs = [create(DesignSchema, { name: "designs/bb", entryUri: "mount://uploads/bb/r/schematic/bb.kicad_sch" })];
    const el = mount((e) => libraryIsland(e, null));
    await vi.waitFor(() => expect(el.querySelectorAll(".lb-row").length).toBe(1));
    expect(el.querySelector('select[name="where"]')).toBeNull();
  });

  it("narrows the list by text and by where a design is kept", async () => {
    fake.designs = [
      create(DesignSchema, { name: "designs/bb-1.1.3", title: "Baseboard", series: "bb", entryUri: "mount://uploads/bb-1.1.3/r/schematic/bb.kicad_sch" }),
      create(DesignSchema, { name: "designs/gateway", title: "Gateway", uri: "mount://tut/designs/gateway", entryUri: "mount://tut/designs/gateway/gateway.edn", readOnly: true }),
    ];
    const el = mount((e) => libraryIsland(e, null));
    await vi.waitFor(() => expect(el.querySelectorAll(".lb-row").length).toBe(2));
    const shown = () => [...el.querySelectorAll(".lb-row")].map((r) => (r as HTMLElement).dataset.design);
    const where = el.querySelector('select[name="where"]') as HTMLSelectElement;
    expect([...where.options].map((o) => o.textContent)).toEqual(["Kept anywhere", "Kept on this server", "In the mount tut"]);
    where.value = "";
    where.dispatchEvent(new Event("change"));
    await vi.waitFor(() => expect(shown()).toEqual(["designs/bb-1.1.3"]));
    where.value = "tut";
    where.dispatchEvent(new Event("change"));
    await vi.waitFor(() => expect(shown()).toEqual(["designs/gateway"]));
    where.value = "*";
    where.dispatchEvent(new Event("change"));
    const text = el.querySelector('input[name="text"]') as HTMLInputElement;
    text.value = "BASE";
    text.dispatchEvent(new Event("input", { bubbles: true }));
    await vi.waitFor(() => expect(shown()).toEqual(["designs/bb-1.1.3"]));
  });
});

describe("a design's reports", () => {
  it("shows each report as a tab, marks a stale one, and keeps a new one run against the chosen pack", async () => {
    fake.design = create(DesignSchema, { name: "designs/bb", entryUri: "mount://uploads/bb/r/schematic/bb.kicad_sch" });
    fake.packs = [create(PackSchema, { name: "packs/house", title: "House rules" })];
    fake.reports = [
      create(ReportSchema, {
        name: "designs/bb/reports/older",
        packs: ["packs/house"],
        designChanged: true,
        findingCount: 2,
        results: {
          findings: [
            { rule: "bulk-cap", severity: "warning", subject: { kind: "net", ref: "VCC" }, message: "no bulk cap", verdictIds: ["bulk-cap:(net:VCC)"] },
            { rule: "house/naming", severity: "warning", subject: { kind: "net", ref: "x" }, message: "off-convention" },
          ],
        },
      }),
    ];
    const el = mount((e) => designIsland(e, null, "bb"));
    await vi.waitFor(() => expect(el.querySelector('[data-testid="report-state"]')?.textContent).toBe("run on earlier design files"));
    expect(el.querySelector('[data-rule="bulk-cap"] summary')?.textContent).toBe("warning bulk-cap 1");
    expect(el.querySelector('[data-rule="bulk-cap"] li')?.textContent).toBe("net VCC no bulk cap");
    expect(el.querySelector('[data-rule="bulk-cap"] li a')?.getAttribute("href")).toBe("/designs/bb/view?pack=house&verdict=bulk-cap%3A%28net%3AVCC%29&rule=bulk-cap");
    expect(el.querySelector('[data-rule="house/naming"] li a')).toBeNull();
    expect(el.querySelector('[data-testid="report"]')?.getAttribute("data-total")).toBe("2");
    const details = el.querySelector('[data-testid="validate"]') as HTMLDetailsElement;
    details.open = true;
    details.dispatchEvent(new Event("toggle"));
    await vi.waitFor(() => expect(el.querySelector('input[name="pack"]')).not.toBeNull());
    const box = el.querySelector('input[name="pack"]') as HTMLInputElement;
    box.checked = true;
    box.dispatchEvent(new Event("change"));
    (el.querySelector('[data-testid="validate"] button') as HTMLButtonElement).click();
    await vi.waitFor(() => expect(el.querySelectorAll(".lb-tab").length).toBe(2));
    expect(fake.created).toEqual([{ parent: "designs/bb", project: "", report: { packs: ["packs/house"] } }]);
    expect(el.querySelector(".lb-tab-on")?.getAttribute("data-report")).toBe("designs/bb/reports/new");
    expect(el.querySelector('[data-testid="report-state"]')?.textContent).toBe("current");
  });
});

describe("findingsByRule", () => {
  it("puts errors first, then the rules with most findings", () => {
    const f = (rule: string, severity: string) => ({ rule, severity }) as Parameters<typeof findingsByRule>[0][number];
    const groups = findingsByRule([f("a", "info"), f("a", "info"), f("b", "warning"), f("c", "error"), f("d", "warning"), f("d", "warning")]);
    expect(groups.map((g) => `${g.rule}:${g.findings.length}`)).toEqual(["c:1", "d:2", "b:1", "a:2"]);
  });
});

describe("createAndUpload", () => {
  it("deletes a design it made when the upload fails, and passes the error on", async () => {
    const deleted: string[] = [];
    const c = {
      createDesign: async ({ designId }: { designId: string }) => create(DesignSchema, { name: `designs/${designId}` }),
      deleteDesign: async ({ name }: { name: string }) => deleted.push(name),
      importDesignFiles: async () => {
        throw new ConnectError("these files hold 2 designs", Code.InvalidArgument);
      },
      updateArtifact: async () => ({}) as never,
    };
    await expect(createAndUpload(c, { uploadBlobs: async () => undefined }, { id: "two", title: "", series: "" }, { "a.edn": new Uint8Array([1]) })).rejects.toThrow("2 designs");
    expect(deleted).toEqual(["designs/two"]);
  });
});

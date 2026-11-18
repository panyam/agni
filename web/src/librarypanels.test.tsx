// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { DesignSchema } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { ArtifactSchema, ArtifactType, ReportSchema } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import { PackSchema } from "@panyam/agni-client/gen/agni/v1/webapi/packs_pb";
import { createAndUpload, designIsland, explain, findingsByRule, libraryIsland } from "./librarypanels.jsx";

const fake = vi.hoisted(() => ({
  designs: [] as unknown[],
  artifacts: [] as unknown[],
  design: undefined as unknown,
  listError: undefined as unknown,
  reports: [] as unknown[],
  packs: [] as unknown[],
  created: [] as unknown[],
}));
vi.mock("./api.js", () => ({
  designClient: () => ({
    listDesigns: async () => {
      if (fake.listError) throw fake.listError;
      return { designs: fake.designs, unreachable: [], nextPageToken: "" };
    },
    getDesign: async () => fake.design,
    listArtifacts: async () => ({ artifacts: fake.artifacts, nextPageToken: "" }),
    listReports: async () => ({ reports: fake.reports, nextPageToken: "" }),
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
    expect(el.querySelector('[data-design="designs/bb-1.1.3"] a.lb-open')?.getAttribute("href")).toBe("/mounts/store/bb-1.1.3/schematic/bb.kicad_sch/view");
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

  it("shows a mounted design as read-only", async () => {
    fake.design = create(DesignSchema, { name: "designs/gateway", entryUri: "mount://tut/designs/gateway/gateway.edn", readOnly: true });
    const el = mount((e) => designIsland(e, null, "gateway"));
    await vi.waitFor(() => expect(el.textContent).toMatch(/declared in a mounted folder/));
    expect(el.querySelector('[data-testid="artifacts"]')).toBeNull();
    expect(el.querySelector('[data-testid="import-files"]')).toBeNull();
  });
});

describe("the library's filter", () => {
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
      create(ReportSchema, { name: "designs/bb/reports/older", packs: ["packs/house"], designChanged: true, results: { findings: [{ rule: "bulk-cap", severity: "warning", subject: { kind: "net", ref: "VCC" }, message: "no bulk cap" }] } }),
    ];
    const el = mount((e) => designIsland(e, null, "bb"));
    await vi.waitFor(() => expect(el.querySelector('[data-testid="report-state"]')?.textContent).toBe("run on earlier design files"));
    expect(el.querySelector('[data-rule="bulk-cap"] summary')?.textContent).toBe("warning bulk-cap 1");
    expect(el.querySelector('[data-rule="bulk-cap"] li')?.textContent).toBe("net VCC no bulk cap");
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

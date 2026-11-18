// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { DesignSchema } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { ArtifactSchema, ArtifactType } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import { createAndUpload, designIsland, explain, libraryIsland } from "./librarypanels.jsx";

const fake = vi.hoisted(() => ({
  designs: [] as unknown[],
  artifacts: [] as unknown[],
  design: undefined as unknown,
  listError: undefined as unknown,
}));
vi.mock("./api.js", () => ({
  designClient: () => ({
    listDesigns: async () => {
      if (fake.listError) throw fake.listError;
      return { designs: fake.designs, unreachable: [], nextPageToken: "" };
    },
    getDesign: async () => fake.design,
    listArtifacts: async () => ({ artifacts: fake.artifacts, nextPageToken: "" }),
  }),
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
    expect(el.querySelector('[data-design="designs/bb-1.1.3"] a.lb-open')?.getAttribute("href")).toBe("/files/store/bb-1.1.3/schematic/bb.kicad_sch/view");
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
    fake.design = create(DesignSchema, { name: "designs/gateway", entryUri: "mount://tut/designs/gateway/gateway.edn" });
    const el = mount((e) => designIsland(e, null, "gateway"));
    await vi.waitFor(() => expect(el.textContent).toMatch(/declared in a mounted folder/));
    expect(el.querySelector('[data-testid="artifacts"]')).toBeNull();
    expect(el.querySelector('[data-testid="import-files"]')).toBeNull();
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

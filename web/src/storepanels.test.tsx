// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { DesignSchema, ProjectSchema } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { PackSchema } from "@panyam/agni-client/gen/agni/v1/webapi/packs_pb";
import { packIsland, projectIsland } from "./storepanels.jsx";

const fake = vi.hoisted(() => ({
  project: undefined as unknown,
  designs: [] as unknown[],
  filters: [] as string[],
  packs: [] as unknown[],
  updates: [] as unknown[],
  pack: undefined as unknown,
}));
vi.mock("./api.js", () => ({
  designClient: () => ({
    listDesigns: async (req: { filter?: string }) => {
      fake.filters.push(req.filter ?? "");
      return { designs: req.filter ? fake.designs.slice(0, 1) : fake.designs, unreachable: [], nextPageToken: "" };
    },
  }),
  projectClient: () => ({
    getProject: async () => fake.project,
    updateProject: async (req: { project: { defaultPacks?: string[]; designs?: string[] } }) => {
      fake.updates.push(req);
      const p = fake.project as { defaultPacks: string[]; designs: string[] };
      fake.project = create(ProjectSchema, { ...p, ...req.project });
      return fake.project;
    },
  }),
  packClient: () => ({
    listPacks: async () => ({ packs: fake.packs, nextPageToken: "" }),
    getPack: async () => fake.pack,
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
  fake.filters = [];
  fake.updates = [];
});

describe("a project's page", () => {
  it("lists the project's designs through the design list's filter, and orders its default packs", async () => {
    fake.project = create(ProjectSchema, { name: "projects/bench", title: "Bench", designs: ["designs/bb-1.1.3"], defaultPacks: ["packs/a", "packs/b"] });
    fake.designs = [create(DesignSchema, { name: "designs/bb-1.1.3", title: "Rev 1.1.3" }), create(DesignSchema, { name: "designs/other" })];
    fake.packs = [create(PackSchema, { name: "packs/a" }), create(PackSchema, { name: "packs/b" }), create(PackSchema, { name: "packs/c" })];
    const el = mount((e) => projectIsland(e, null, "bench"));
    await vi.waitFor(() => expect(el.querySelectorAll(".lb-row").length).toBe(1));
    expect(fake.filters).toContain('project = "projects/bench"');
    expect([...el.querySelectorAll("[data-testid=project-packs] li")].map((li) => (li as HTMLElement).dataset.pack)).toEqual(["packs/a", "packs/b"]);
    const up = [...el.querySelectorAll("[data-testid=project-packs] li[data-pack='packs/b'] button")].find((b) => b.textContent === "Up") as HTMLButtonElement;
    up.click();
    await vi.waitFor(() => expect(fake.updates.length).toBe(1));
    expect(fake.updates[0]).toEqual({ project: { name: "projects/bench", defaultPacks: ["packs/b", "packs/a"] }, updateMask: { paths: ["default_packs"] } });
    await vi.waitFor(() => expect([...el.querySelectorAll("[data-testid=project-packs] li")].map((li) => (li as HTMLElement).dataset.pack)).toEqual(["packs/b", "packs/a"]));
    expect([...el.querySelectorAll("[data-testid=add-pack] option")].map((o) => o.textContent)).toContain("c");
  });
});

describe("a pack's page", () => {
  it("shows the pack's convention, checklists and file tiers", async () => {
    fake.pack = create(PackSchema, {
      name: "packs/jetson",
      title: "Jetson rules",
      hash: "sha256:4f95adf4462d5de5d1c7724a21037260cff7b1e5",
      config: { conventions: { name: "jetson", rules: [{ name: "r" }] }, checklists: [{ name: "review" }, { name: "drc" }], profileUris: ["profiles"], paramUris: ["params"] },
      files: [{ path: "profiles/can.yaml" }, { path: "params/a.textproto" }],
    });
    const el = mount((e) => packIsland(e, null, "jetson"));
    await vi.waitFor(() => expect(el.querySelector(".lb-title")?.textContent).toBe("Jetson rules"));
    const rows = [...el.querySelectorAll("[data-testid=pack-tiers] tr")].map((tr) => [tr.querySelector("th")?.textContent, tr.querySelector("td")?.textContent]);
    expect(rows).toEqual([
      ["Convention", "jetson (1 rule)"],
      ["Checklists", "review, drc"],
      ["Profiles", "profiles"],
      ["Params", "params"],
    ]);
  });
});

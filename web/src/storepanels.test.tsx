// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { DesignSchema, ProjectSchema } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { PackSchema } from "@panyam/agni-client/gen/agni/v1/webapi/packs_pb";
import { MountKind, MountSchema } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import { packIsland, packsIsland, projectIsland } from "./storepanels.jsx";

const fake = vi.hoisted(() => ({
  project: undefined as unknown,
  designs: [] as unknown[],
  filters: [] as string[],
  packs: [] as unknown[],
  updates: [] as unknown[],
  pack: undefined as unknown,
  links: [] as unknown[],
  syncs: [] as string[],
  mounts: [] as unknown[],
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
    linkPack: async (req: unknown) => {
      fake.links.push(req);
      return create(PackSchema, { name: "packs/house" });
    },
    syncPack: async (req: { name: string }) => {
      fake.syncs.push(req.name);
      return { pack: fake.pack, changed: true, fromVersion: "c1", toVersion: "c2c2c2c2c2c2c2c2" };
    },
  }),
  workspaceClient: () => ({
    listMounts: async () => ({ mounts: fake.mounts, canCreateMounts: false, signInUrl: "" }),
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
  fake.links = [];
  fake.syncs = [];
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

describe("a linked pack", () => {
  it("shows where its pack came from, syncs it, and offers no upload over it", async () => {
    fake.pack = create(PackSchema, {
      name: "packs/house",
      title: "House rules",
      hash: "sha256:4f95adf4462d5de5d1c7724a21037260cff7b1e5",
      source: { mount: "rules", ref: "main", follow: true, version: "0b3cf5c09bee7fe29d6b", readHash: "sha256:ab" },
    });
    const el = mount((e) => packIsland(e, null, "house"));
    await vi.waitFor(() => expect(el.querySelector("[data-testid=pack-source]")?.textContent).toContain("From rules@main at 0b3cf5c09bee, followed"));
    expect(el.querySelector("[data-testid=pack-upload]")).toBeNull();
    const sync = [...el.querySelectorAll("[data-testid=pack-source] button")].find((b) => b.textContent === "Sync") as HTMLButtonElement;
    sync.click();
    await vi.waitFor(() => expect(el.querySelector("[data-testid=pack-status]")?.textContent).toBe("Moved to c2c2c2c2c2c2."));
    expect(fake.syncs).toEqual(["packs/house"]);
  });

  it("links a pack from a mount's folder at the ref the person names", async () => {
    fake.packs = [];
    fake.mounts = [create(MountSchema, { name: "rules", source: "github://hw/agni-packs", ref: "main", kind: MountKind.GITHUB }), create(MountSchema, { name: "uploads", kind: MountKind.UPLOADS })];
    const assign = vi.fn();
    vi.stubGlobal("location", { ...window.location, assign });
    const el = mount((e) => packsIsland(e, null));
    const form = await vi.waitFor(() => {
      const f = el.querySelector("[data-testid=pack-link]") as HTMLFormElement;
      expect(f.querySelectorAll("select[name=mount] option").length).toBe(2);
      return f;
    });
    const select = form.querySelector("select[name=mount]") as HTMLSelectElement;
    select.value = "rules";
    select.dispatchEvent(new Event("change"));
    expect((form.querySelector("input[name=ref]") as HTMLInputElement).value).toBe("main");
    const id = form.querySelector("input[name=id]") as HTMLInputElement;
    id.value = "house";
    id.dispatchEvent(new Event("input", { bubbles: true }));
    const path = form.querySelector("input[name=path]") as HTMLInputElement;
    path.value = "/rules/";
    path.dispatchEvent(new Event("input", { bubbles: true }));
    form.requestSubmit();
    await vi.waitFor(() => expect(fake.links.length).toBe(1));
    expect(fake.links[0]).toEqual({ packId: "house", title: "", source: { mount: "rules", path: "rules", ref: "main" } });
    await vi.waitFor(() => expect(assign).toHaveBeenCalledWith("/packs/house"));
    vi.unstubAllGlobals();
  });
});

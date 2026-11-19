// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import { DesignSchema } from "@panyam/agni-client/gen/agni/v1/webapi/project_pb";
import { ArtifactSchema, ArtifactSourceSchema } from "@panyam/agni-client/gen/agni/v1/webapi/designs_pb";
import { ProposedDesignSchema } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import { folderDesignsIsland, folderShown, unclaimed } from "./folderdesigns.jsx";

const fake = vi.hoisted(() => ({
  designs: [] as unknown[],
  artifacts: {} as Record<string, unknown[]>,
  proposals: [] as unknown[],
  added: [] as unknown[],
  filters: [] as string[],
}));
vi.mock("./api.js", () => ({
  designClient: () => ({
    listDesigns: async (req: { filter: string }) => {
      fake.filters.push(req.filter);
      return { designs: fake.designs, unreachable: [], nextPageToken: "" };
    },
    listArtifacts: async (req: { parent: string }) => ({ artifacts: fake.artifacts[req.parent] ?? [], nextPageToken: "" }),
    addDesigns: async (req: unknown) => {
      fake.added.push(req);
      return { designs: [] };
    },
  }),
  workspaceClient: () => ({ proposeDesigns: async () => ({ designs: fake.proposals, unread: [], support: [] }) }),
  uploadsClient: () => ({}),
  packClient: () => ({}),
  projectClient: () => ({}),
}));

beforeEach(() => {
  document.body.replaceChildren();
  fake.designs = [];
  fake.artifacts = {};
  fake.proposals = [];
  fake.added = [];
  fake.filters = [];
});

const src = (mount: string, path: string) => create(ArtifactSourceSchema, { mount, path });
const proposal = (folder: string) => create(ProposedDesignSchema, { folder, files: ["a"] });

describe("unclaimed", () => {
  it("keeps the proposals no kept design reads from", () => {
    const ps = [proposal("boards/a"), proposal("boards/b")];
    expect(unclaimed(ps, "lab", [src("lab", "boards/a")]).map((p) => p.folder)).toEqual(["boards/b"]);
    expect(unclaimed(ps, "lab", [src("other", "boards/a")]).map((p) => p.folder)).toEqual(["boards/a", "boards/b"]);
  });
});

describe("folderShown", () => {
  it("names a proposal's folder from the folder asked about", () => {
    expect(folderShown("boards/a", "boards")).toBe("a");
    expect(folderShown("boards", "boards")).toBe(".");
    expect(folderShown("x/a", "")).toBe("x/a");
  });
});

describe("the folder panel", () => {
  it("lists the folder's designs and its unclaimed boards, and adds the ticked ones", async () => {
    fake.designs = [create(DesignSchema, { name: "designs/a", title: "Board A" })];
    fake.artifacts = { "designs/a": [create(ArtifactSchema, { name: "designs/a/artifacts/schematic", source: src("lab", "boards/a") })] };
    fake.proposals = [proposal("boards/a"), proposal("boards/b"), proposal("boards/c")];
    const el = document.createElement("div");
    document.body.appendChild(el);
    const panel = folderDesignsIsland(el, null);
    panel.island.activate();
    panel.show({ mount: "lab", path: "boards" });
    await vi.waitFor(() => expect(el.querySelectorAll("[data-proposal]").length).toBe(2));
    expect(fake.filters).toEqual(['source = "mount://lab/boards"']);
    expect(el.querySelector('[data-design="designs/a"] a')?.getAttribute("href")).toBe("/designs/a");
    const add = el.querySelector('[data-testid="add-designs"]') as HTMLButtonElement;
    expect(add.disabled).toBe(true);
    const box = el.querySelector('input[value="boards/c"]') as HTMLInputElement;
    box.checked = true;
    box.dispatchEvent(new Event("change"));
    const series = el.querySelector('input[name="series"]') as HTMLInputElement;
    series.value = "lab";
    series.dispatchEvent(new Event("input", { bubbles: true }));
    await vi.waitFor(() => expect(add.disabled).toBe(false));
    add.click();
    await vi.waitFor(() => expect(fake.added.length).toBe(1));
    expect(fake.added[0]).toEqual({ uri: "mount://lab/boards", designs: [{ folder: "boards/c", series: "lab" }] });
    panel.show(null);
    expect(el.style.display).toBe("none");
  });
});

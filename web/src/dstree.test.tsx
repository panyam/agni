// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { artifactUri, uriPath } from "./uri.js";
import { FileKind } from "./gen/agni/v1/webapi/workspace_pb.js";
import { dsTreeIsland } from "./dstree.jsx";

// The datasheets tree's first tests. It has always been the viewer tree's quieter twin, and the
// asymmetry is what this file exists to hold: both trees list the same mounts and must reach
// opposite conclusions about which of them are worth showing.
const fake = vi.hoisted(() => ({
  calls: [] as string[],
  opens: [] as (FileKind[] | undefined)[],
  mountOpens: [] as (FileKind[] | undefined)[],
  mounts: [{ name: "ds", root: "/ds" }] as { name: string; root: string }[],
  prunedMounts: 0,
  dirs: {} as Record<string, unknown[]>,
}));
vi.mock("./api.js", () => ({
  workspaceClient: () => ({
    listMounts: async ({ opens }: { opens?: FileKind[] } = {}) => {
      fake.mountOpens.push(opens);
      return { mounts: fake.mounts, prunedMounts: fake.prunedMounts };
    },
    listDir: async ({ uri, opens }: { uri: string; opens?: FileKind[] }) => {
      const path = uriPath(uri);
      fake.calls.push(path);
      fake.opens.push(opens);
      const entries = fake.dirs[path];
      if (!entries) throw new Error(`no such dir ${path}`);
      return { entries };
    },
  }),
}));

const dir = (name: string, path: string) => ({ name, uri: artifactUri("ds", path), isDir: true, format: "", kind: FileKind.UNSPECIFIED });
const entry = (name: string, path: string, kind: FileKind) => ({ name, uri: artifactUri("ds", path), isDir: false, format: "", kind });

function mountTree() {
  const onSelect = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const tree = dsTreeIsland(el, null, onSelect);
  tree.island.activate();
  return { el, tree, onSelect };
}

const buttons = (el: HTMLElement) => [...el.querySelectorAll("button")].map((b) => b.textContent?.replace(/[▾▸]/g, "").trim());
const buttonFor = (el: HTMLElement, label: string) => [...el.querySelectorAll("button")].find((b) => b.textContent?.includes(label));
const note = (el: HTMLElement) => el.querySelector(".note")?.textContent?.trim();
const settle = () => vi.waitFor(() => {}, { timeout: 500 });

beforeEach(() => {
  document.body.replaceChildren();
  fake.calls = [];
  fake.opens = [];
  fake.mountOpens = [];
  fake.prunedMounts = 0;
  fake.mounts = [{ name: "ds", root: "/ds" }];
  fake.dirs = {
    "": [dir("vendor", "vendor"), entry("txb0104.pdf", "txb0104.pdf", FileKind.DATASHEET), entry("board.edn", "board.edn", FileKind.DESIGN)],
    vendor: [entry("lm1117.pdf", "vendor/lm1117.pdf", FileKind.DATASHEET)],
  };
  Element.prototype.scrollIntoView = vi.fn();
});

describe("datasheets tree", () => {
  it("shows datasheets and hides what this page cannot open", async () => {
    const { el, onSelect } = mountTree();
    await vi.waitFor(() => expect(buttonFor(el, "ds")).toBeTruthy());

    buttonFor(el, "ds")!.click();
    await vi.waitFor(() => expect(buttonFor(el, "txb0104.pdf")).toBeTruthy());

    // A design file is listed by the server and dropped here: it belongs to the other page, and a
    // row that opened nothing would be worse than its absence.
    expect(buttonFor(el, "board.edn")).toBeUndefined();

    buttonFor(el, "txb0104.pdf")!.click();
    expect(onSelect).toHaveBeenCalledWith("ds", "txb0104.pdf");
  });

  // The half this tree was missing: it filtered FILES by extension but asked the server to prune
  // nothing, so a folder of schematics sat in the sidebar with nothing openable under it.
  it("declares DATASHEET on every listing so the server prunes for it", async () => {
    const { el } = mountTree();
    await vi.waitFor(() => expect(buttonFor(el, "ds")).toBeTruthy());
    buttonFor(el, "ds")!.click();
    await settle();

    expect(fake.mountOpens).toEqual([[FileKind.DATASHEET]]);
    expect(fake.opens.length).toBeGreaterThan(0);
    expect(fake.opens.every((o) => o?.length === 1 && o[0] === FileKind.DATASHEET)).toBe(true);
  });

  it("accounts for mounts the server pruned", async () => {
    fake.prunedMounts = 2;
    const { el } = mountTree();
    await vi.waitFor(() => expect(note(el)).toBe("2 folders hidden (no datasheets)"));

    // The wording is this page's, not the viewer's: the same sentence about designs would be wrong
    // here, and a mount hidden from one tree is routinely the one the other is built on.
    expect(note(el)).not.toContain("designs");
  });

  it("says so when no mount holds a datasheet", async () => {
    fake.mounts = [];
    fake.prunedMounts = 1;
    const { el } = mountTree();
    await vi.waitFor(() => expect(note(el)).toBe("No datasheets in any of the 1 folder being served"));
    expect(buttons(el)).toHaveLength(0);
  });
});

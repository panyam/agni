// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { uriPath } from "./uri.js";
import { fileTreeIsland } from "./filetree.jsx";
import type { SheetsState } from "./sheets.js";

// The island builds its own client via workspaceClient(); the mock swaps in an in-memory
// workspace so the reveal cascade's per-level listDir round-trips run against canned data.
const fake = vi.hoisted(() => ({
  calls: [] as string[],
  dirs: {} as Record<string, unknown[]>,
}));
vi.mock("./api.js", () => ({
  workspaceClient: () => ({
    listMounts: async () => ({ mounts: [{ name: "m", root: "/m" }] }),
    listDir: async ({ uri }: { uri: string }) => {
      const path = uriPath(uri);
      fake.calls.push(path);
      const entries = fake.dirs[path];
      if (!entries) throw new Error(`no such dir ${path}`);
      return { entries };
    },
  }),
}));

const dir = (name: string, path: string) => ({ name, path, isDir: true, format: "" });
const file = (name: string, path: string, format = "edif") => ({ name, path, isDir: false, format });

function mountTree() {
  const handlers = {
    onFileSelect: vi.fn(),
    onDirSelect: vi.fn(),
    onSheetSelect: vi.fn(),
  };
  const el = document.createElement("div");
  document.body.appendChild(el);
  const tree = fileTreeIsland(el, null, handlers);
  tree.island.activate();
  return { el, tree, handlers };
}

// buttons returns the visible node labels, normalized (the twist glyph and format tag stripped).
const buttons = (el: HTMLElement) =>
  [...el.querySelectorAll("button")].map((b) => b.textContent?.replace(/[▾▸]/g, "").trim());

const buttonFor = (el: HTMLElement, label: string) =>
  [...el.querySelectorAll("button")].find((b) => b.textContent?.includes(label));

const settle = () => vi.waitFor(() => {}, { timeout: 500 });

const openFile = (mount: string, path: string): SheetsState => ({
  mount,
  path,
  sheets: [{ id: "s1", name: "Root", parentId: "" } as never],
  activeId: "s1",
});

beforeEach(() => {
  document.body.replaceChildren();
  fake.calls = [];
  fake.dirs = {
    "": [dir("a", "a"), dir("z", "z"), file("top.edn", "top.edn"), file("notes.txt", "notes.txt", "")],
    a: [dir("b", "a/b"), file("mid.edn", "a/mid.edn")],
    "a/b": [file("deep.edn", "a/b/deep.edn")],
    z: [file("zonly.edn", "z/zonly.edn")],
  };
  Element.prototype.scrollIntoView = vi.fn();
});

describe("filetree island", () => {
  it("lists mounts, expands one level on click, and caches the listing", async () => {
    const { el, handlers } = mountTree();
    await vi.waitFor(() => expect(buttonFor(el, "m")).toBeTruthy());

    buttonFor(el, "m")!.click();
    await vi.waitFor(() => expect(buttonFor(el, "top.edn")).toBeTruthy());
    expect(handlers.onDirSelect).toHaveBeenCalledWith("m", "");

    // A file with no reader is hidden entirely (2026-07-14: filtered as noise; reversal of
    // the earlier show-greyed behavior).
    expect(buttonFor(el, "notes.txt")).toBeUndefined();

    // Collapse and re-expand: the level is cached, not refetched.
    buttonFor(el, "m")!.click();
    buttonFor(el, "m")!.click();
    await settle();
    expect(fake.calls.filter((p) => p === "")).toHaveLength(1);

    // A file click is an intent, not a fetch.
    buttonFor(el, "top.edn")!.click();
    expect(handlers.onFileSelect).toHaveBeenCalledWith("m", "top.edn");
  });

  it("auto-reveals the open file: expansion cascades level by level to the deep link", async () => {
    const { el, tree } = mountTree();
    await vi.waitFor(() => expect(buttonFor(el, "m")).toBeTruthy());

    tree.view.setState(openFile("m", "a/b/deep.edn"));
    await vi.waitFor(() => expect(buttonFor(el, "deep.edn")).toBeTruthy());

    // Every ancestor level was listed exactly once, in order, and the file is active with
    // its sheet rendered beneath it.
    expect(fake.calls).toEqual(["", "a", "a/b"]);
    expect(buttonFor(el, "deep.edn")!.className).toContain("active");
    expect(buttonFor(el, "Root")).toBeTruthy();
    // The sibling branch was not opened by the cascade.
    expect(buttonFor(el, "zonly.edn")).toBeUndefined();
  });

  it("revealDir expands to a folder with no open file under it", async () => {
    const { el, tree } = mountTree();
    await vi.waitFor(() => expect(buttonFor(el, "m")).toBeTruthy());

    tree.revealDir("m", "a/b");
    await vi.waitFor(() => expect(buttonFor(el, "deep.edn")).toBeTruthy());
    expect(buttonFor(el, "deep.edn")!.className).not.toContain("active");
  });

  it("leaves a manually collapsed unrelated branch alone on the next push", async () => {
    const { el, tree } = mountTree();
    await vi.waitFor(() => expect(buttonFor(el, "m")).toBeTruthy());

    // Open the unrelated branch z by hand, then collapse it again.
    buttonFor(el, "m")!.click();
    await vi.waitFor(() => expect(buttonFor(el, "z")).toBeTruthy());
    buttonFor(el, "z")!.click();
    await vi.waitFor(() => expect(buttonFor(el, "zonly.edn")).toBeTruthy());
    buttonFor(el, "z")!.click();
    await settle();
    expect(buttonFor(el, "zonly.edn")).toBeUndefined();

    // A new push targeting the a/ branch must not re-expand z.
    tree.view.setState(openFile("m", "a/mid.edn"));
    await vi.waitFor(() => expect(buttonFor(el, "mid.edn")).toBeTruthy());
    expect(buttonFor(el, "zonly.edn")).toBeUndefined();
    expect(buttons(el)).toContain("z");
  });
});

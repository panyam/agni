// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { artifactUri, uriPath } from "@agni/web-shared/uri.js";
import { FileKind } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import { Code, ConnectError } from "@connectrpc/connect";
import { fileTreeIsland, type TreeOptions } from "./filetree.jsx";
import type { SheetsState } from "./sheets.js";

// The island builds its own client via workspaceClient(); the mock swaps in an in-memory
// workspace so the reveal cascade's per-level listDir round-trips run against canned data.
const fake = vi.hoisted(() => ({
  calls: [] as string[],
  opens: [] as (FileKind[] | undefined)[],
  prunedMounts: 0,
  mounts: [{ name: "m", root: "/m" }] as { name: string; root: string; fixed?: boolean }[],
  mountOpens: [] as (FileKind[] | undefined)[],
  dirs: {} as Record<string, unknown[]>,
  canCreateMounts: false,
  signInUrl: "",
  created: [] as { name: string; source: string; ref: string; connection?: string }[],
  connections: [] as { name: string; displayName: string }[],
  deleted: [] as string[],
  createError: null as Error | null,
}));
vi.mock("./api.js", () => ({
  connectionClient: () => ({
    listConnections: async () => ({ connections: fake.connections }),
  }),
  workspaceClient: () => ({
    listMounts: async ({ opens }: { opens?: FileKind[] } = {}) => {
      fake.mountOpens.push(opens);
      return { mounts: fake.mounts, prunedMounts: fake.prunedMounts, canCreateMounts: fake.canCreateMounts, signInUrl: fake.signInUrl };
    },
    createMount: async (req: { name: string; source: string; ref: string; connection?: string }) => {
      if (fake.createError) throw fake.createError;
      fake.created.push(req);
      fake.mounts = [...fake.mounts, { name: req.name, root: "/v", fixed: false }];
      return { name: req.name };
    },
    deleteMount: async ({ name }: { name: string }) => {
      fake.deleted.push(name);
      fake.mounts = fake.mounts.filter((m) => m.name !== name);
      return {};
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

const dir = (name: string, path: string) => ({ name, uri: artifactUri("m", path), isDir: true, format: "" });
const file = (name: string, path: string, format = "edif") => ({
  name,
  uri: artifactUri("m", path),
  isDir: false,
  format,
  // The tree filters on the server's kind label, not on format, so a fixture needs one. An
  // unreadable file gets UNSPECIFIED, which is what the hiding test asserts is dropped.
  kind: format ? FileKind.DESIGN : FileKind.UNSPECIFIED,
});

function mountTree(options: TreeOptions = {}) {
  const handlers = {
    onFileSelect: vi.fn(),
    onDirSelect: vi.fn(),
    onSheetSelect: vi.fn(),
  };
  const el = document.createElement("div");
  document.body.appendChild(el);
  const tree = fileTreeIsland(el, null, handlers, options);
  tree.island.activate();
  return { el, tree, handlers };
}

// buttons returns the visible node labels, normalized (the twist glyph and format tag stripped).
const buttons = (el: HTMLElement) =>
  [...el.querySelectorAll("button")].map((b) => b.textContent?.replace(/[▾▸]/g, "").trim());

// note is the tree's account of what pruning hid, absent when it hid nothing.
const note = (el: HTMLElement) => el.querySelector(".note")?.textContent?.trim();

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
  fake.opens = [];
  fake.mountOpens = [];
  fake.prunedMounts = 0;
  fake.mounts = [{ name: "m", root: "/m", fixed: true }];
  fake.canCreateMounts = false;
  fake.signInUrl = "";
  fake.created = [];
  fake.deleted = [];
  fake.createError = null;
  fake.connections = [];
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

    // A file with no reader is hidden entirely (filtered as noise since 2026-07-14,
    // reversing the earlier show-greyed behavior).
    expect(buttonFor(el, "notes.txt")).toBeUndefined();

    // The folder half of the same rule is the server's to answer, since one level of listing
    // cannot tell a folder of designs from a folder of folders of nothing. Every listing this tree
    // asks for declares DESIGN, which is both what gets pruned and what the row filter above keeps.
    expect(fake.opens.length).toBeGreaterThan(0);
    expect(fake.opens.every((o) => o?.length === 1 && o[0] === FileKind.DESIGN)).toBe(true);

    // Collapse and re-expand. The level is cached, not refetched.
    buttonFor(el, "m")!.click();
    buttonFor(el, "m")!.click();
    await settle();
    expect(fake.calls.filter((p) => p === "")).toHaveLength(1);

    // A file click is an intent, not a fetch.
    buttonFor(el, "top.edn")!.click();
    expect(handlers.onFileSelect).toHaveBeenCalledWith("m", "top.edn");
  });

  // A mount root gets the same treatment as a folder inside one, so a mount serving only
  // datasheets does not root a branch with nothing under it. The count means an operator
  // who configured a mount can see it was hidden rather than lost.
  it("prunes empty mount roots and accounts for what it hid", async () => {
    fake.mounts = [];
    fake.prunedMounts = 2;
    const { el } = mountTree();

    await vi.waitFor(() => expect(note(el)).toBe("No designs in any of the 2 folders being served"));
    expect(fake.mountOpens).toEqual([[FileKind.DESIGN]]);
  });

  it("names the surviving mounts and still reports the hidden one", async () => {
    fake.prunedMounts = 1;
    const { el } = mountTree();

    await vi.waitFor(() => expect(buttonFor(el, "m")).toBeTruthy());
    await vi.waitFor(() => expect(note(el)).toBe("1 folder hidden (no designs)"));
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

// Managing mounts happens on the /mounts/ browse page only (agni issue 1749).
describe("managing mounts", () => {
  const submitForm = (el: HTMLElement, values: Record<string, string>) => {
    const form = el.querySelector(".mount-add form") as HTMLFormElement;
    for (const [k, v] of Object.entries(values)) (form.elements.namedItem(k) as HTMLInputElement).value = v;
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  };

  it("offers nothing in the viewer's tree, even on a server that makes mounts", async () => {
    fake.canCreateMounts = true;
    fake.mounts = [
      { name: "m", root: "/m", fixed: true },
      { name: "gh", root: "/v", fixed: false },
    ];
    const { el } = mountTree();
    await vi.waitFor(() => expect(buttonFor(el, "gh")).toBeTruthy());
    expect(el.querySelector(".mount-add")).toBeNull();
    expect(el.querySelector(".mount-rm")).toBeNull();
  });

  it("shows the add form only when the server says it can make mounts", async () => {
    const off = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(buttonFor(off.el, "m")).toBeTruthy());
    expect(off.el.querySelector(".mount-add")).toBeNull();

    fake.canCreateMounts = true;
    const on = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(on.el.querySelector(".mount-add")).not.toBeNull());
  });

  it("adds a GitHub mount from a pasted repo URL and lists it as a new root", async () => {
    fake.canCreateMounts = true;
    const { el } = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(el.querySelector(".mount-add form")).not.toBeNull());
    submitForm(el, { name: "jetson", repo: "https://github.com/antmicro/jetson-agx-thor-baseboard.git", ref: "main" });
    await vi.waitFor(() => expect(buttonFor(el, "jetson")).toBeTruthy());
    expect(fake.created).toEqual([{ name: "jetson", source: "github://antmicro/jetson-agx-thor-baseboard", ref: "main" }]);
    expect(el.querySelector(".mount-add-msg")).toBeNull();
  });

  it("offers the server's connections, and mounts a private repo through the one picked", async () => {
    fake.canCreateMounts = true;
    fake.connections = [{ name: "connections/acme", displayName: "acme org (App)" }];
    const { el } = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(el.querySelector(".mount-add select[name=connection]")).not.toBeNull());
    const options = [...el.querySelectorAll(".mount-add select[name=connection] option")].map((o) => o.textContent);
    expect(options).toEqual(["public repo, no connection", "acme org (App)"]);
    submitForm(el, { name: "board", repo: "acme/board", ref: "", connection: "connections/acme" });
    await vi.waitFor(() => expect(fake.created).toHaveLength(1));
    expect(fake.created[0]).toEqual({ name: "board", source: "github://acme/board", ref: "", connection: "connections/acme" });
  });

  it("hides the connection picker on a server with no connections", async () => {
    fake.canCreateMounts = true;
    const { el } = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(el.querySelector(".mount-add form")).not.toBeNull());
    expect(el.querySelector(".mount-add select[name=connection]")).toBeNull();
  });

  it("links a person with no connection to sign in with GitHub, and back to this page", async () => {
    fake.canCreateMounts = true;
    const off = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(off.el.querySelector(".mount-add form")).not.toBeNull());
    expect(off.el.querySelector(".mount-add-signin")).toBeNull();

    fake.signInUrl = "/signin/github";
    const on = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(on.el.querySelector(".mount-add-signin a")).not.toBeNull());
    expect(on.el.querySelector(".mount-add-signin a")?.getAttribute("href")).toBe(`/signin/github?next=${encodeURIComponent(window.location.pathname)}`);

    fake.connections = [{ name: "connections/github-alice", displayName: "alice on GitHub" }];
    const signedIn = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(signedIn.el.querySelector(".mount-add select[name=connection]")).not.toBeNull());
    expect(signedIn.el.querySelector(".mount-add-signin")).toBeNull();
  });

  it("shows the server's own message when it refuses a mount", async () => {
    fake.canCreateMounts = true;
    fake.createError = new ConnectError("GitHub has no /repos/antmicro/x/commits/9.9", Code.NotFound);
    const { el } = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(el.querySelector(".mount-add form")).not.toBeNull());
    submitForm(el, { name: "x", repo: "antmicro/x", ref: "9.9" });
    await vi.waitFor(() => expect(el.querySelector(".mount-add-msg")?.textContent).toBe("GitHub has no /repos/antmicro/x/commits/9.9"));
  });

  it("removes a runtime mount after a confirm, and never offers to remove a fixed one", async () => {
    fake.mounts = [
      { name: "m", root: "/m", fixed: true },
      { name: "gh", root: "/v", fixed: false },
    ];
    const confirm = vi.spyOn(window, "confirm").mockReturnValueOnce(false).mockReturnValueOnce(true);
    const { el } = mountTree({ manageMounts: true });
    await vi.waitFor(() => expect(buttonFor(el, "gh")).toBeTruthy());
    const rm = [...el.querySelectorAll<HTMLButtonElement>(".mount-rm")];
    expect(rm.map((b) => b.getAttribute("aria-label"))).toEqual(["Remove the mount gh"]);

    rm[0].click();
    await settle();
    expect(fake.deleted).toEqual([]);

    rm[0].click();
    await vi.waitFor(() => expect(buttonFor(el, "gh")).toBeUndefined());
    expect(fake.deleted).toEqual(["gh"]);
    expect(confirm).toHaveBeenCalledTimes(2);
    confirm.mockRestore();
  });
});

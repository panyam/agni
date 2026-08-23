// @vitest-environment jsdom
import { describe, it, expect, beforeAll, vi } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// The browse page's composition root, under test (agni issue 136).
//
// composition.test.ts does this for the viewer and explains at length why a composition root needs
// its own test: the ports are optional by design, so an unwired one is a silent no-op rather than a
// type error, and no panel-level test can see it because every panel test supplies its own
// collaborators. That argument is not about main.ts. It is about composition roots, and this page
// has one too — the tree, the preview driver, and the stage are each covered on their own, and
// nothing until now asserted that browse.ts connects them.
//
// So this boots the REAL browse.ts against the REAL BrowsePage.html and walks the flow the ticket
// names: choose a design, see it previewed, open it. What it does not assert is how the SVG looks,
// which needs a browser and stays on the successor issue.

// pageBody slices the shipped template's Body block, for the same reason composition.test.ts does:
// the holes and their ids are the contract between the server-rendered page and browse.ts, and a
// test carrying its own copy of the markup keeps passing after the page changes underneath it.
function pageBody(): string {
  const src = readFileSync(join(process.cwd(), "templates/BrowsePage.html"), "utf8");
  const m = /\{\{ define "Body" \}\}([\s\S]*?)\n\{\{ end \}\}/.exec(src);
  if (!m) throw new Error("BrowsePage.html has no Body block; the browse composition test cannot run");
  return m[1];
}

const SHEET_SVG = '<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>';

// One reply per rpc the browse flow makes. Deliberately thin: this asserts that a call was MADE and
// that its result reached the page, never what the server would really return.
const REPLIES: Record<string, unknown> = {
  ListMounts: { mounts: [{ name: "m", root: "/m" }], prunedMounts: 0 },
  ListDir: {
    entries: [
      { name: "board.edn", uri: "mount://m/board.edn", isDir: false, format: "edif", kind: 1 },
      { name: "notes.txt", uri: "mount://m/notes.txt", isDir: false, format: "", kind: 0 },
    ],
  },
  GetDesign: {
    name: "Sample Board",
    sourceFormat: "edif-2.0.0",
    componentCount: 19,
    netCount: 15,
    sheets: [{ id: "s1", name: "netlist graph" }],
    availableLayouts: ["grid"],
  },
  // The wire form of a oneof is the FIELD name, not the client-side {case, value} pair that
  // connect-web decodes it into. Writing the decoded shape here yields an empty document and a
  // blank preview, which is what this fixture did on its first run.
  GetSheet: { svg: SHEET_SVG },
};

const called: string[] = [];
const navigated: string[] = [];
const bootErrors: unknown[] = [];

const el = (id: string): HTMLElement => {
  const found = document.getElementById(id);
  if (!found) throw new Error(`the page declares no #${id}`);
  return found;
};
const buttonFor = (label: string): HTMLButtonElement | undefined =>
  [...document.querySelectorAll("button")].find((b) => b.textContent?.includes(label)) as HTMLButtonElement | undefined;

beforeAll(async () => {
  // jsdom implements neither of these, and the tree reaches for scrollIntoView whenever a node
  // becomes active. Stubbing them cannot hide a wiring bug: an island mounts or it does not.
  Element.prototype.scrollIntoView = vi.fn();
  vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
    bootErrors.push(args[0]);
  });
  (globalThis as unknown as { fetch: unknown }).fetch = async (input: unknown) => {
    const url = typeof input === "string" ? input : (input as Request).url;
    const method = url.split("/").pop() ?? "";
    called.push(method);
    return new Response(JSON.stringify(REPLIES[method] ?? {}), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  };
  // Leaving the page is a real document navigation (routing is server-owned, C11), which jsdom
  // does not implement. Recording it is the assertion: Open's whole job is to hand off a URL.
  Object.defineProperty(window, "location", {
    configurable: true,
    value: { ...window.location, pathname: "/designs/", assign: (u: string) => navigated.push(u) },
  });

  document.body.innerHTML = pageBody();
  await import("./browse.js");
  await vi.waitFor(() => expect(called).toContain("ListMounts"), 5000);
});

describe("the browse page boots", () => {
  it("raises nothing on the way up", () => {
    expect(bootErrors).toEqual([]);
  });

  it("mounts the file tree into the hole the page declares", async () => {
    await vi.waitFor(() => expect(el("browse-tree").children.length).toBeGreaterThan(0));
  });
});

describe("choose a design, see it, open it", () => {
  it("previews the chosen design and enables Open", async () => {
    await vi.waitFor(() => expect(buttonFor("m")).toBeTruthy());
    buttonFor("m")!.click(); // expand the mount
    await vi.waitFor(() => expect(buttonFor("board.edn")).toBeTruthy());

    // A file with no reader never reaches the tree, so the browse page cannot preview one.
    expect(buttonFor("notes.txt")).toBeUndefined();

    buttonFor("board.edn")!.click();
    await vi.waitFor(() => expect(called).toContain("GetSheet"), 5000);

    // The whole point of the page: the design's own drawing, and a caption naming it.
    expect(el("browse-preview").querySelector("rect")).toBeTruthy();
    expect(el("browse-name").textContent).toBe("Sample Board");
    expect(el("browse-summary").textContent).toContain("19 components");
    expect((el("browse-open") as HTMLButtonElement).disabled).toBe(false);
  });

  it("hands the design's work-page URL to a real navigation", async () => {
    (el("browse-open") as HTMLButtonElement).click();
    await vi.waitFor(() => expect(navigated).toHaveLength(1));
    expect(navigated[0]).toBe("/designs/m/board.edn/view");
  });

  // A folder is a place to look rather than a thing to open, so choosing one re-addresses the URL
  // and empties the stage instead of leaving the previous design on screen under a new location.
  it("clears the stage when a folder is chosen", async () => {
    buttonFor("m")!.click();
    await vi.waitFor(() => expect(el("browse-preview").style.display).toBe("none"));
    expect((el("browse-open") as HTMLButtonElement).disabled).toBe(true);
    expect(window.history.state === null || window.location.pathname.startsWith("/designs/")).toBe(true);
  });
});

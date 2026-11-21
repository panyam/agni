// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { artifactUri, uriPath } from "@agni/web-shared/uri.js";
import { FileKind } from "@panyam/agni-client/gen/agni/v1/webapi/workspace_pb";
import { comparePickerIsland } from "./comparepicker.js";

// The picker's tree builds its own client via workspaceClient(); swap in an in-memory workspace,
// same shape as filetree.test.tsx.
const fake = vi.hoisted(() => ({ dirs: {} as Record<string, unknown[]>, designs: [] as unknown[], revisions: {} as Record<string, unknown[]>, entries: {} as Record<string, string> }));
vi.mock("./api.js", () => ({
  workspaceClient: () => ({
    listMounts: async () => ({ mounts: [{ name: "m", root: "/m" }] }),
    listDir: async ({ uri }: { uri: string }) => ({ entries: fake.dirs[uriPath(uri)] ?? [] }),
  }),
  designClient: () => ({
    listDesigns: async () => ({ designs: fake.designs, nextPageToken: "" }),
    listDesignRevisions: async ({ parent }: { parent: string }) => ({ revisions: fake.revisions[parent] ?? [], nextPageToken: "" }),
    getDesign: async ({ name }: { name: string }) => ({ name, entryUri: fake.entries[name] ?? "" }),
  }),
}));

const file = (name: string, path: string, format = "kicad") => ({
  name,
  uri: artifactUri("m", path),
  isDir: false,
  format,
  // The tree filters on the server's kind label, so a fixture without one is invisible.
  kind: format ? FileKind.DESIGN : FileKind.UNSPECIFIED,
});

function mountPicker() {
  const host = document.createElement("div");
  const designsEl = document.createElement("div");
  const treeEl = document.createElement("div");
  host.append(designsEl, treeEl);
  document.body.appendChild(host);
  const onPick = vi.fn();
  const built = comparePickerIsland(host, treeEl, null, onPick, designsEl);
  built.island.activate();
  built.designs?.activate();
  return { host, designsEl, treeEl, onPick, picker: built.picker };
}

// settle lets the tree's listMounts/listDir promises resolve and Solid flush.
const settle = async () => {
  await Promise.resolve();
  await Promise.resolve();
  await new Promise((r) => setTimeout(r, 0));
};

const buttonFor = (el: HTMLElement, label: string) =>
  [...el.querySelectorAll("button")].find((b) => b.textContent?.includes(label));

describe("comparePickerIsland", () => {
  beforeEach(() => {
    document.body.replaceChildren();
    fake.dirs = { "": [file("a.kicad_sch", "a.kicad_sch"), file("b.kicad_sch", "b.kicad_sch")] };
    fake.designs = [];
    fake.revisions = {};
    fake.entries = {};
  });

  it("starts hidden", () => {
    const { host, picker } = mountPicker();
    expect(picker.isOpen()).toBe(false);
    expect(host.classList.contains("on")).toBe(false);
    expect(host.getAttribute("aria-hidden")).toBe("true");
  });

  it("shows on open and hides on close", () => {
    const { host, picker } = mountPicker();
    picker.open({ mount: "m", path: "a.kicad_sch" });
    expect(picker.isOpen()).toBe(true);
    expect(host.classList.contains("on")).toBe(true);
    expect(host.getAttribute("aria-hidden")).toBe("false");

    picker.close();
    expect(picker.isOpen()).toBe(false);
    expect(host.classList.contains("on")).toBe(false);
  });

  // Pushing the open design as the tree's state also drives the tree's auto-reveal, so the picker
  // opens already expanded to that design's folder, so its neighbours (the likely comparison targets,
  // e.g. sibling versions) are one click away with no navigating. Asserted here because it is a
  // behavior the picker gets for free from the tree, which makes it easy to break unknowingly.
  it("opens already expanded to the folder holding the open design", async () => {
    const { treeEl, picker } = mountPicker();
    picker.open({ mount: "m", path: "a.kicad_sch" });
    await settle();

    expect(buttonFor(treeEl, "b.kicad_sch")).toBeDefined();
  });

  it("reports the chosen design and closes", async () => {
    const { host, treeEl, onPick, picker } = mountPicker();
    picker.open({ mount: "m", path: "a.kicad_sch" });
    await settle();

    buttonFor(treeEl, "b.kicad_sch")?.click();

    expect(onPick).toHaveBeenCalledWith({ mount: "m", path: "b.kicad_sch" });
    expect(picker.isOpen()).toBe(false);
    expect(host.classList.contains("on")).toBe(false);
  });

  // Comparing a design against itself is not a comparison. Ignoring rather than closing keeps the
  // click reading as "that one is already open" instead of a silent dismissal.
  it("ignores a click on the design already open, and stays open", async () => {
    const { treeEl, onPick, picker } = mountPicker();
    picker.open({ mount: "m", path: "a.kicad_sch" });
    await settle();

    buttonFor(treeEl, "a.kicad_sch")?.click();

    expect(onPick).not.toHaveBeenCalled();
    expect(picker.isOpen()).toBe(true);
  });

  it("dismisses on a backdrop click but not on a click inside the dialog", () => {
    const { host, treeEl, picker } = mountPicker();
    picker.open(null);

    treeEl.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    expect(picker.isOpen()).toBe(true); // inside the dialog: not a dismissal

    host.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    expect(picker.isOpen()).toBe(false);
  });

  it("dismisses on Escape, and ignores Escape while already closed", () => {
    const { picker } = mountPicker();
    picker.open(null);
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(picker.isOpen()).toBe(false);

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(picker.isOpen()).toBe(false);
  });

  // Opening with no design open is reachable only if the button's disabled guard fails, but the
  // picker must not throw on a null exclude. Nothing is then marked unavailable.
  it("opens with no design to exclude", async () => {
    const { treeEl, onPick, picker } = mountPicker();
    picker.open(null);
    await settle();
    buttonFor(treeEl, "m")?.click();
    await settle();

    buttonFor(treeEl, "a.kicad_sch")?.click();
    expect(onPick).toHaveBeenCalledWith({ mount: "m", path: "a.kicad_sch" });
  });

  // A stored design and each of its revisions are offered beside the files, and a pick compares
  // against the entry each names in the uploads mount (agni issue 1774).
  it("offers the stored designs and their revisions, each picking its entry", async () => {
    fake.designs = [
      { name: "designs/bb", title: "Baseboard", series: "board", entryUri: "mount://uploads/bb/r3/schematic/bb.kicad_sch" },
      { name: "designs/gateway", readOnly: true, entryUri: "mount://tut/designs/gateway/gateway.edn" },
      { name: "designs/draft", entryUri: "" },
    ];
    fake.revisions["designs/bb"] = [
      { revisionId: "r3", refs: ["1.1.3"], current: true },
      { revisionId: "r2", refs: [] },
    ];
    fake.entries["designs/bb@r2"] = "mount://uploads/bb/r2/schematic/bb.kicad_sch";
    const { designsEl, onPick, picker } = mountPicker();
    picker.open({ mount: "m", path: "a.kicad_sch" });
    await settle();

    // A design declared in a mounted folder is in the tree, and a draft opens nothing yet.
    expect([...designsEl.querySelectorAll("li[data-design]")].map((li) => li.getAttribute("data-design"))).toEqual(["designs/bb"]);
    const details = designsEl.querySelector("details") as HTMLDetailsElement;
    details.open = true;
    details.dispatchEvent(new Event("toggle"));
    await settle();
    expect([...designsEl.querySelectorAll("button[data-revision]")].map((b) => b.textContent)).toEqual(["1.1.3 (current)", "r2"]);

    (designsEl.querySelector('button[data-revision="r2"]') as HTMLButtonElement).click();
    await settle();
    expect(onPick).toHaveBeenCalledWith({ mount: "uploads", path: "bb/r2/schematic/bb.kicad_sch" });
    expect(picker.isOpen()).toBe(false);

    picker.open(null);
    buttonFor(designsEl, "Baseboard")?.click();
    expect(onPick).toHaveBeenLastCalledWith({ mount: "uploads", path: "bb/r3/schematic/bb.kicad_sch" });
  });

  it("ignores a stored design that is the one already open", async () => {
    fake.designs = [{ name: "designs/bb", title: "Baseboard", entryUri: "mount://uploads/bb/r3/schematic/bb.kicad_sch" }];
    const { designsEl, onPick, picker } = mountPicker();
    picker.open({ mount: "uploads", path: "bb/r3/schematic/bb.kicad_sch" });
    await settle();
    buttonFor(designsEl, "Baseboard")?.click();
    expect(onPick).not.toHaveBeenCalled();
    expect(picker.isOpen()).toBe(true);
  });

  it("hides the list on a server that keeps no designs", async () => {
    const { designsEl, picker } = mountPicker();
    picker.open(null);
    await settle();
    expect((designsEl.querySelector('[data-testid="compare-designs"]') as HTMLElement).hidden).toBe(true);
  });
});

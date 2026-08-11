// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { artifactUri, uriPath } from "./uri.js";
import { comparePickerIsland } from "./comparepicker.js";

// The picker's tree builds its own client via workspaceClient(); swap in an in-memory workspace,
// same shape as filetree.test.tsx.
const fake = vi.hoisted(() => ({ dirs: {} as Record<string, unknown[]> }));
vi.mock("./api.js", () => ({
  workspaceClient: () => ({
    listMounts: async () => ({ mounts: [{ name: "m", root: "/m" }] }),
    listDir: async ({ uri }: { uri: string }) => ({ entries: fake.dirs[uriPath(uri)] ?? [] }),
  }),
}));

const file = (name: string, path: string, format = "kicad") => ({ name, path, isDir: false, format });

function mountPicker() {
  const host = document.createElement("div");
  const treeEl = document.createElement("div");
  host.appendChild(treeEl);
  document.body.appendChild(host);
  const onPick = vi.fn();
  const built = comparePickerIsland(host, treeEl, null, onPick);
  built.island.activate();
  return { host, treeEl, onPick, picker: built.picker };
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
  // opens already expanded to that design's folder — its neighbours (the likely comparison targets,
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

    expect(onPick).toHaveBeenCalledWith({ uri: artifactUri("m", "b.kicad_sch") });
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
  // picker must not throw on a null exclude — nothing is then marked unavailable.
  it("opens with no design to exclude", async () => {
    const { treeEl, onPick, picker } = mountPicker();
    picker.open(null);
    await settle();
    buttonFor(treeEl, "m")?.click();
    await settle();

    buttonFor(treeEl, "a.kicad_sch")?.click();
    expect(onPick).toHaveBeenCalledWith({ uri: artifactUri("m", "a.kicad_sch") });
  });
});

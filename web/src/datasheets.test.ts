// @vitest-environment jsdom
import { describe, it, expect, vi, beforeAll } from "vitest";
import { loadRecents } from "./recents.js";

// The datasheets page's boot, under test for the first time. The three islands are mocked rather
// than real: the region viewer pulls in pdf.js, which needs a canvas and a worker jsdom does not
// have, and none of that is what this asserts. What it asserts is the BOOT PATH, which is where the
// page has a blind spot the viewer's composition.test.ts already covers for main.ts.
//
// The bug that prompted it: a deep link restored a datasheet by reaching past `open` to
// view.load + tree.setState, which was the same sequence right up until `open` gained recording.
// From then on a datasheet opened by URL never reached the landing page's Recent list, with every
// unit test green. It was found by looking at a screenshot, which is not a test strategy.
const calls = vi.hoisted(() => ({ loaded: [] as string[] }));

// A stand-in island: the lifecycle controller initializes whatever performLocalInit returns, so a
// mock has to satisfy LCMComponent even though it does nothing.
const stubIsland = () => ({
  performLocalInit: () => [],
  setupDependencies: () => {},
  activate: () => {},
  deactivate: () => {},
});
vi.mock("./regionview.js", () => ({
  workbenchIsland: () => ({
    island: stubIsland(),
    view: {
      load: (mount: string, path: string) => calls.loaded.push(`${mount}/${path}`),
      locate: () => {},
    },
  }),
}));
vi.mock("./dstree.js", () => ({
  dsTreeIsland: () => ({ island: stubIsland(), view: { setState: () => {} } }),
}));
vi.mock("./paramspanel.js", () => ({
  paramsPanelIsland: () => ({ island: stubIsland(), view: { setState: () => {} } }),
}));

beforeAll(async () => {
  localStorage.clear();
  document.body.innerHTML = `<div id="ds-tree"></div><div id="ds-view"></div><div id="ds-params"></div>`;
  window.history.replaceState(null, "", "/datasheets/files/ds/vendor/txb0104.pdf");
  await import("./datasheets.js");
  await vi.waitFor(() => expect(calls.loaded.length).toBeGreaterThan(0), 3000);
});

describe("the datasheets page boots from a deep link", () => {
  it("loads the addressed datasheet", () => {
    expect(calls.loaded).toContain("ds/vendor/txb0104.pdf");
  });

  // Arriving by URL is arriving. The viewer counts a deep link as an opening, and a page that
  // disagreed would leave the Recent list quietly dependent on how you got there.
  it("records it for the landing page's Recent list", () => {
    const got = loadRecents();
    expect(got.map((r) => `${r.mount}/${r.path}`)).toContain("ds/vendor/txb0104.pdf");
    expect(got[0].kind).toBe("datasheet"); // routed back to the workbench, not the viewer
  });

  // The restore must not push a history entry for the URL it is replaying, or Back walks through
  // the same datasheet twice.
  it("leaves the URL it replayed alone", () => {
    expect(window.location.pathname).toBe("/datasheets/files/ds/vendor/txb0104.pdf");
  });
});

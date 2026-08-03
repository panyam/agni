// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { VIEWER_PANELS, LAYOUT_KEY, loadLayout, saveLayout, reconcilePanels, adoptPanel, defaultLayout, panelsMenu, openPanel } from "./dock.js";

function memStorage(initial: Record<string, string> = {}): Pick<Storage, "getItem" | "setItem" | "removeItem"> & { data: Map<string, string> } {
  const data = new Map(Object.entries(initial));
  return {
    data,
    getItem: (k) => data.get(k) ?? null,
    setItem: (k, v) => void data.set(k, v),
    removeItem: (k) => void data.delete(k),
  };
}

describe("layout persistence", () => {
  it("returns null when nothing is saved", () => {
    expect(loadLayout(memStorage())).toBeNull();
  });

  it("round-trips a layout, stamped with the panel registry at save time", () => {
    const storage = memStorage();
    const layout = { grid: { root: { type: "branch" } }, panels: { canvas: {} } };
    saveLayout(storage, layout);
    const loaded = loadLayout(storage)!;
    expect(loaded.layout).toEqual(layout);
    expect(loaded.savedPanels).toEqual(VIEWER_PANELS.map((p) => p.id));
  });

  it("accepts a pre-versioning raw save as an unknown registry", () => {
    // Layouts persisted before the version wrapper are bare dockview JSON; they restore
    // with an empty savedPanels, so every non-on-demand panel counts as newly added once.
    const raw = { grid: { root: {} }, panels: { canvas: {} } };
    const storage = memStorage({ [LAYOUT_KEY]: JSON.stringify(raw) });
    const loaded = loadLayout(storage)!;
    expect(loaded.layout).toEqual(raw);
    expect(loaded.savedPanels).toEqual([]);
  });

  it("returns null for a corrupt save instead of throwing", () => {
    const storage = memStorage({ [LAYOUT_KEY]: "{not json" });
    expect(loadLayout(storage)).toBeNull();
  });

  it("swallows storage write failures", () => {
    const storage = {
      getItem: () => null,
      setItem: () => {
        throw new Error("quota");
      },
      removeItem: () => {},
    };
    expect(() => saveLayout(storage, { a: 1 })).not.toThrow();
  });
});

describe("panel registry", () => {
  it("covers exactly the eleven viewer panels with unique ids", () => {
    const ids = VIEWER_PANELS.map((p) => p.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids.sort()).toEqual([
      "canvas",
      "changes",
      "checks",
      "coverage",
      "details",
      "diff",
      "files",
      "overview",
      "parts",
      "query",
      "rules",
    ]);
  });
});

describe("reconcilePanels", () => {
  // A fake DockviewApi recording addPanel calls; `open` seeds the currently open ids.
  function fakeApi(open: string[]) {
    const added: string[] = [];
    return {
      added,
      api: {
        getPanel: (id: string) => (open.includes(id) || added.includes(id) ? { api: { setActive: () => {} } } : undefined),
        addPanel: (opts: { id: string }) => void added.push(opts.id),
      } as never,
    };
  }
  const openIds = VIEWER_PANELS.filter((p) => !p.onDemand).map((p) => p.id);

  it("opens panels added to the registry since the layout was saved", () => {
    const savedWithoutChecks = VIEWER_PANELS.map((p) => p.id).filter((id) => id !== "checks");
    const { api, added } = fakeApi(openIds.filter((id) => id !== "checks"));
    reconcilePanels(api, savedWithoutChecks);
    expect(added).toEqual(["checks"]);
  });

  it("leaves user-closed panels closed (present in the saved registry, absent from the layout)", () => {
    const { api, added } = fakeApi(openIds.filter((id) => id !== "rules")); // user closed rules
    reconcilePanels(api, VIEWER_PANELS.map((p) => p.id)); // registry unchanged since save
    expect(added).toEqual([]);
  });

  it("never opens on-demand panels, even for a pre-versioning save", () => {
    const { api, added } = fakeApi(openIds);
    reconcilePanels(api, []); // legacy save: unknown registry
    expect(added).toEqual([]); // everything default-open already open; diff/changes stay closed
  });

  it("reopens only missing DEFAULT-OPEN panels for a legacy save; secondaries stay closed (WS9-042)", () => {
    // Legacy save (unknown registry) missing a core panel (details) and a secondary (report).
    // Only the core one is reconciled open; the secondary is discoverable in the Panels menu.
    const { api, added } = fakeApi(openIds.filter((id) => id !== "details" && id !== "report"));
    reconcilePanels(api, []);
    expect(added).toEqual(["details"]);
  });

  it("leaves a newly-registered SECONDARY panel closed — menu-only, no auto-open (WS9-042)", () => {
    // 'query' is secondary; simulate it added since the save (absent from savedPanels) and
    // currently closed. reconcile must not open it — contrast the 'checks' (core) case above.
    const savedWithoutQuery = VIEWER_PANELS.map((p) => p.id).filter((id) => id !== "query");
    const { api, added } = fakeApi(openIds.filter((id) => id !== "query"));
    reconcilePanels(api, savedWithoutQuery);
    expect(added).toEqual([]);
  });
});

// dockStub is a minimal DockviewApi tracking which panel ids are open, enough to drive
// defaultLayout, openPanel, and the menu toggle. getPanel returns a handle whose id lets
// removePanel find it; the nested `api` no-ops absorb the deferred sizing calls.
function dockStub(open: string[] = []) {
  const ids = [...open];
  const api = {
    addPanel: (o: { id: string }) => void (ids.includes(o.id) || ids.push(o.id)),
    getPanel: (id: string) => (ids.includes(id) ? { id, api: { setSize() {}, setActive() {} } } : undefined),
    removePanel: (p: { id: string }) => {
      const i = ids.indexOf(p.id);
      if (i >= 0) ids.splice(i, 1);
    },
  };
  return { api: api as never, ids };
}

describe("defaultLayout", () => {
  it("opens exactly the default-open panels, none of the secondaries or on-demand (WS9-042)", () => {
    const { api, ids } = dockStub();
    defaultLayout(api);
    const expected = VIEWER_PANELS.filter((p) => p.defaultOpen).map((p) => p.id).sort();
    expect(ids.slice().sort()).toEqual(expected);
    // Sanity: the lean set drops the reference/secondary panels.
    for (const secondary of ["rules", "report", "expectations", "query", "diff", "changes"]) {
      expect(ids).not.toContain(secondary);
    }
  });
});

describe("panelsMenu toggle", () => {
  function openMenu(open: string[]) {
    const host = document.createElement("div");
    document.body.appendChild(host);
    const { api, ids } = dockStub(open);
    panelsMenu(host, api);
    const btn = host.querySelector<HTMLButtonElement>(".panels-btn")!;
    btn.click(); // rebuild + show the list
    const itemFor = (title: string) =>
      Array.from(host.querySelectorAll<HTMLButtonElement>(".panels-item")).find((b) => b.textContent?.includes(title))!;
    return { host, ids, itemFor };
  }

  it("hides an open panel when its menu item is clicked", () => {
    const { ids, itemFor } = openMenu(["canvas", "checks"]);
    expect(itemFor("Checks").textContent).toContain("✓");
    itemFor("Checks").click();
    expect(ids).not.toContain("checks");
    document.body.replaceChildren();
  });

  it("shows a closed panel when its menu item is clicked", () => {
    const { ids, itemFor } = openMenu(["canvas"]);
    expect(itemFor("Rules").textContent).not.toContain("✓");
    itemFor("Rules").click();
    expect(ids).toContain("rules");
    document.body.replaceChildren();
  });
});

describe("openPanel", () => {
  it("adds the panel by id so the menu can reopen a closed one", () => {
    const { api, ids } = dockStub();
    openPanel(api, { id: "query", title: "Query" });
    expect(ids).toEqual(["query"]);
  });
});

describe("adoptPanel", () => {
  it("adopts the server-rendered hole and parks it back on dispose, preserving identity", () => {
    const park = document.createElement("div");
    const hole = document.createElement("div");
    hole.setAttribute("data-dock-panel", "rules");
    park.appendChild(hole);
    document.body.appendChild(park);

    const renderer = adoptPanel(park, "rules");
    expect(renderer.element).toBe(hole);

    // Simulate dockview mounting the panel, then the user closing it.
    const dock = document.createElement("div");
    document.body.appendChild(dock);
    dock.appendChild(renderer.element);
    expect(park.contains(hole)).toBe(false);
    renderer.dispose?.();
    expect(park.contains(hole)).toBe(true);

    // Reopening resolves the SAME element, so the island mounted in it stays live.
    const reopened = adoptPanel(park, "rules");
    expect(reopened.element).toBe(hole);

    document.body.replaceChildren();
  });

  it("yields an empty element for an unknown panel name instead of crashing", () => {
    const park = document.createElement("div");
    document.body.appendChild(park);
    const renderer = adoptPanel(park, "no-such-panel");
    expect(renderer.element).toBeInstanceOf(HTMLElement);
    expect(renderer.element.childNodes.length).toBe(0);
    document.body.replaceChildren();
  });
});

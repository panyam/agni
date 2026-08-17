// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { VIEWER_PANELS, LAYOUT_KEY, loadLayout, saveLayout, reconcilePanels, prunePanels, adoptPanel, defaultLayout, panelsMenu, openPanel } from "./dock.js";

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
      "overview",
      "parts",
      "query",
      "review",
      "rules",
    ]);
  });

  // Phase 1 demoted Files to secondary and kept it registered only because the old Compare flow
  // needed a tree to click side B in. The picker replaced that flow, so the panel is gone. Pinned
  // separately from the id list because re-adding it there would read as an ordinary new panel.
  it("no longer registers Files (WS9-049 phase 3)", () => {
    expect(VIEWER_PANELS.find((p) => p.id === "files")).toBeUndefined();
  });

  it("leaves Sheets default-open as the work page's navigation surface", () => {
    expect(VIEWER_PANELS.find((p) => p.id === "overview")?.defaultOpen).toBe(true);
  });

  // The secondary tier is gone: the layout tabs the crowding away rather than hiding panels behind
  // a menu, so every registered panel is placed at boot. A future menu-only panel omits the flag,
  // and this assertion is what makes that a deliberate act rather than an oversight.
  it("places every registered panel at boot, with no menu-only tier", () => {
    const menuOnly = VIEWER_PANELS.filter((p) => !p.defaultOpen).map((p) => p.id);
    expect(menuOnly).toEqual([]);
  });
});

describe("prunePanels", () => {
  // A fake DockviewApi exposing the open panels as `panels` and recording removals.
  function fakeApi(open: string[]) {
    const panels = open.map((id) => ({ id }));
    const removed: string[] = [];
    return {
      removed,
      panels,
      api: {
        get panels() {
          return panels;
        },
        removePanel: (p: { id: string }) => {
          removed.push(p.id);
          panels.splice(panels.findIndex((x) => x.id === p.id), 1);
        },
      } as never,
    };
  }

  it("closes a panel the registry no longer has", () => {
    const { api, removed } = fakeApi(["canvas", "checks", "retired-panel"]);
    prunePanels(api);
    expect(removed).toEqual(["retired-panel"]);
  });

  it("leaves every registered panel alone", () => {
    const { api, removed } = fakeApi(VIEWER_PANELS.map((p) => p.id));
    prunePanels(api);
    expect(removed).toEqual([]);
  });

  // This is the WS9-049 phase 3 migration, and the reason prunePanels was added in phase 1. An
  // existing user's saved layout still names "files"; its hole is gone from the template, so
  // without the prune adoptPanel falls back to a blank div and they get an empty "Files" tab.
  it("closes a restored Files panel now that the registry has dropped it", () => {
    const { api, removed } = fakeApi(["overview", "canvas", "details", "checks", "files"]);
    prunePanels(api);
    expect(removed).toEqual(["files"]);
  });

  it("removes several retired panels in one pass without skipping any", () => {
    // Guards the snapshot in prunePanels: removePanel mutates the collection being walked, so
    // iterating it live would skip the entry that slides into the removed one's index.
    const { api, removed } = fakeApi(["gone-a", "gone-b", "canvas"]);
    prunePanels(api);
    expect(removed.sort()).toEqual(["gone-a", "gone-b"]);
  });

  it("tolerates an api that reports no panels", () => {
    expect(() => prunePanels({ removePanel: () => {} } as never)).not.toThrow();
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
  const openIds = VIEWER_PANELS.map((p) => p.id);

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

  // The tier this used to cover (a newly-registered SECONDARY, menu-only) no longer has an instance
  // to test with: every panel is placed at boot. What survives is the rule that matters more — a
  // panel the user closed stays closed, told apart from a new one by the saved registry.
  it("leaves a panel the user closed alone when it was in the saved registry", () => {
    const savedWithQuery = VIEWER_PANELS.map((p) => p.id);
    const { api, added } = fakeApi(openIds.filter((id) => id !== "query"));
    reconcilePanels(api, savedWithQuery);
    expect(added).toEqual([]);
  });
});

// dockStub is a minimal DockviewApi tracking which panel ids are open, enough to drive
// defaultLayout, openPanel, and the menu toggle. getPanel returns a handle whose id lets
// removePanel find it; the nested `api` no-ops absorb the deferred sizing calls.
interface Placement {
  id: string;
  direction?: string;
  reference?: string;
}

function dockStub(open: string[] = []) {
  const ids = [...open];
  const placed: Placement[] = [];
  const api = {
    addPanel: (o: { id: string; position?: { direction?: string; referencePanel?: string } }) => {
      placed.push({ id: o.id, direction: o.position?.direction, reference: o.position?.referencePanel });
      if (!ids.includes(o.id)) ids.push(o.id);
    },
    getPanel: (id: string) =>
      ids.includes(id) ? { id, api: { setActive() {}, group: { api: { setSize() {} } } } } : undefined,
    removePanel: (p: { id: string }) => {
      const i = ids.indexOf(p.id);
      if (i >= 0) ids.splice(i, 1);
    },
  };
  return { api: api as never, ids, placed };
}

describe("defaultLayout", () => {
  // The arrangement is the whole point of this function and nothing asserted it before: the previous
  // test checked WHICH panels opened and not WHERE, so any reshuffle was invisible.
  //
  // Ordering carries meaning here. Each position is relative to the reference panel's GROUP, so
  // query splitting the centre depends on both rails already existing; place it earlier and it
  // becomes a full-width strip under all three columns.
  it("builds the two rails, then splits the centre for query", () => {
    const { api, placed } = dockStub();
    defaultLayout(api);
    const at = (id: string) => placed.find((p) => p.id === id);

    expect(at("canvas")?.direction).toBeUndefined(); // the centre is the anchor

    expect(at("overview")).toMatchObject({ direction: "left", reference: "canvas" });
    expect(at("details")).toMatchObject({ direction: "below", reference: "overview" });
    expect(at("parts")).toMatchObject({ direction: "within", reference: "details" });

    // The east rail is two stacks of two, not one stack of four: a 15% column clips a four-tab
    // strip, and dockview gives no overflow affordance, so the fourth tab becomes unreachable.
    expect(at("checks")).toMatchObject({ direction: "right", reference: "canvas" });
    expect(at("rules")).toMatchObject({ direction: "within", reference: "checks" });
    expect(at("review")).toMatchObject({ direction: "below", reference: "checks" });
    expect(at("coverage")).toMatchObject({ direction: "within", reference: "review" });

    for (const tab of ["diff", "changes"]) {
      expect(at(tab), `${tab} should tab with the canvas`).toMatchObject({ direction: "within", reference: "canvas" });
    }

    const order = placed.map((p) => p.id);
    expect(at("query")).toMatchObject({ direction: "below", reference: "canvas" });
    expect(order.indexOf("query")).toBeGreaterThan(order.indexOf("checks"));
    expect(order.indexOf("query")).toBeGreaterThan(order.indexOf("overview"));
  });

  it("opens every registered panel", () => {
    const { api, ids } = dockStub();
    defaultLayout(api);
    expect(ids.slice().sort()).toEqual(VIEWER_PANELS.map((p) => p.id).sort());
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

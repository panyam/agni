// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { queryPanelIsland } from "./querypanel.jsx";
import { type ExampleItem, type QueryResult, type RelationItem, LocateReason, emptyResult, errorResult, resultFromResponse } from "./query.js";

function mountPanel() {
  const onRun = vi.fn();
  const onLocate = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = queryPanelIsland(el, null, { onRun, onLocate });
  panel.island.activate();
  const push = (s: QueryResult) => panel.view.setState(s);
  const pushRelations = (rels: RelationItem[]) => panel.view.setRelations(rels);
  const pushExamples = (ex: ExampleItem[]) => panel.view.setExamples(ex);
  return { el, panel, onRun, onLocate, push, pushRelations, pushExamples };
}

// a two-row result over a component, a net, and a scalar column, with sheet badges on the entity
// cells — the shape the locate tests drive.
function pushLocatable(push: (s: QueryResult) => void) {
  push(
    resultFromResponse(
      {
        columns: ["r", "n", "v"],
        columnKinds: ["component", "net", ""],
        rows: [
          {
            cells: ["R1", "SDA", "3.3"],
            cites: [],
            cellSheets: [{ sheetIds: ["s2"] }, { sheetIds: ["s1"] }, { sheetIds: [] }],
          },
        ],
      } as never,
      (ids) => ids.map((id) => ({ id, name: id })),
    ),
  );
}

const rel = (over: Partial<RelationItem>): RelationItem => ({ name: "r", args: ["a", "b"], summary: "s", kind: "netlist", detail: "", ...over });

function typeQuery(el: HTMLElement, text: string) {
  const ta = el.querySelector<HTMLTextAreaElement>("textarea.query-text")!;
  ta.value = text;
  ta.dispatchEvent(new Event("input", { bubbles: true }));
}

beforeEach(() => document.body.replaceChildren());

describe("querypanel", () => {
  it("emits onRun with the trimmed query text when Run is clicked", () => {
    const { el, onRun } = mountPanel();
    typeQuery(el, "  component-on-net(?r,?n) => ?r  ");
    el.querySelector<HTMLButtonElement>("button.query-run")!.click();
    expect(onRun).toHaveBeenCalledOnce();
    expect(onRun).toHaveBeenCalledWith("component-on-net(?r,?n) => ?r");
  });

  it("disables Run for an empty query and while a run is loading", () => {
    const { el, push } = mountPanel();
    const btn = () => el.querySelector<HTMLButtonElement>("button.query-run")!;
    expect(btn().disabled).toBe(true); // empty box
    typeQuery(el, "component-on-net(?r,?n) => ?r");
    expect(btn().disabled).toBe(false);
    push(emptyResult(true)); // loading
    expect(btn().disabled).toBe(true);
    expect(btn().textContent).toContain("Running");
  });

  it("renders columns and rows, and reveals a row's provenance on expand", () => {
    const { el, push } = mountPanel();
    push(
      resultFromResponse({
        columns: ["r", "n"],
        rows: [{ cells: ["R1", "SDA"], cites: ["x.edn:SDA"] }],
      } as never),
    );
    const heads = [...el.querySelectorAll("thead th")].map((h) => h.textContent);
    expect(heads).toEqual(["", "r", "n"]); // leading provenance-toggle column
    const cells = [...el.querySelectorAll("tbody .query-row td")].map((c) => c.textContent);
    expect(cells).toEqual(["▸", "R1", "SDA"]); // provenance toggle, then the answer cells
    expect(el.querySelector(".query-count")!.textContent).toContain("1 result");
    // Provenance is hidden until the row toggle is clicked.
    expect(el.querySelector(".query-cites")).toBeNull();
    el.querySelector<HTMLButtonElement>(".query-cite-toggle")!.click();
    expect([...el.querySelectorAll(".query-cites li")].map((l) => l.textContent)).toEqual(["x.edn:SDA"]);
  });

  it("shows 'No results.' when a run matched nothing", () => {
    const { el, push } = mountPanel();
    push(resultFromResponse({ columns: ["r"], rows: [] } as never));
    expect(el.querySelector(".query-empty")!.textContent).toContain("No results");
    expect(el.querySelector(".query-table")).toBeNull();
  });

  it("shows the error message inline and no results table", () => {
    const { el, push } = mountPanel();
    push(errorResult("[invalid_argument] parse error at col 3"));
    expect(el.querySelector(".query-error")!.textContent).toContain("parse error at col 3");
    expect(el.querySelector(".query-table")).toBeNull();
  });

  it("renders relation chips grouped by kind, sorted by name within a group", () => {
    const { el, pushRelations } = mountPanel();
    // Before the catalog arrives, the panel shows the syntax hint, no chips.
    expect(el.querySelector(".query-relchip")).toBeNull();
    pushRelations([
      rel({ name: "net.max_voltage", kind: "netlist" }),
      rel({ name: "component-on-net", kind: "netlist" }),
      rel({ name: "reaches", kind: "predicate" }),
    ]);
    const groups = [...el.querySelectorAll(".query-relgroup")].map((g) => ({
      label: g.querySelector(".query-relgroup-name")!.textContent,
      chips: [...g.querySelectorAll(".query-relchip")].map((c) => c.textContent),
    }));
    // Netlist group before Predicates (kind order); chips alphabetical within the group.
    expect(groups).toEqual([
      { label: "Netlist", chips: ["component-on-net", "net.max_voltage"] },
      { label: "Predicates", chips: ["reaches"] },
    ]);
  });

  it("shows an info button only for a relation with a doc, and opens its Detail on click", () => {
    const { el, pushRelations } = mountPanel();
    pushRelations([
      rel({ name: "net.bus_like", kind: "netlist", detail: "## net.bus_like\n\nA shared node.\n" }),
      rel({ name: "rail", kind: "netlist", detail: "" }),
    ]);
    // The documented relation gets an info affordance; the undocumented one does not.
    const infos = [...el.querySelectorAll(".query-relinfo")];
    expect(infos.length).toBe(1);
    // No detail pane until the info button is clicked.
    expect(el.querySelector(".query-reldetail")).toBeNull();
    (infos[0] as HTMLButtonElement).click();
    const pane = el.querySelector(".query-reldetail");
    expect(pane).not.toBeNull();
    expect(pane!.querySelector(".query-reldetail-name")!.textContent).toBe("net.bus_like");
    expect(pane!.querySelector(".query-reldetail-body")!.innerHTML).toContain("A shared node.");
    // Closing removes the pane.
    (pane!.querySelector(".query-reldetail-close") as HTMLButtonElement).click();
    expect(el.querySelector(".query-reldetail")).toBeNull();
  });

  it("renders component/net cells as locate buttons and scalars as plain text", () => {
    const { el, push } = mountPanel();
    pushLocatable(push);
    const buttons = [...el.querySelectorAll(".query-row .query-locate")].map((b) => b.textContent);
    expect(buttons).toEqual(["R1", "SDA"]); // the component and net cells, not the scalar
    // the scalar cell is plain text in its own cell, with no locate button
    const cells = [...el.querySelectorAll(".query-row td")].map((c) => c.textContent);
    expect(cells).toContain("3.3");
  });

  it("emits onLocate with (kind, subject) when a cell is clicked", () => {
    const { el, push, onLocate } = mountPanel();
    pushLocatable(push);
    el.querySelector<HTMLButtonElement>(".query-row .query-locate")!.click(); // the R1 component cell
    expect(onLocate).toHaveBeenCalledWith("component", "R1", undefined, LocateReason.UNSPECIFIED);
  });

  it("emits onLocate with the sheet id when a cell's badge is clicked", () => {
    const { el, push, onLocate } = mountPanel();
    pushLocatable(push);
    // the net cell (SDA) carries an s1 badge; clicking it navigates to that sheet.
    const netCell = [...el.querySelectorAll(".query-cell-locate")].find((td) => td.textContent?.includes("SDA"))!;
    netCell.querySelector<HTMLElement>(".sheet-badge")!.click();
    expect(onLocate).toHaveBeenCalledWith("net", "SDA", "s1", LocateReason.UNSPECIFIED);
  });

  it("shows the locate note pushed by the presenter, and clears it", () => {
    const { el, push, panel } = mountPanel();
    pushLocatable(push);
    expect(el.querySelector(".query-locate-note")).toBeNull();
    panel.view.setLocateNote("GND is a power rail with no drawn wire.");
    expect(el.querySelector(".query-locate-note")!.textContent).toContain("power rail");
    panel.view.setLocateNote("");
    expect(el.querySelector(".query-locate-note")).toBeNull();
  });

  it("renders example chips and fills+runs the query when one is clicked (WS14-002)", () => {
    const { el, onRun, pushExamples } = mountPanel();
    expect(el.querySelector(".query-example")).toBeNull(); // none until the catalog arrives
    pushExamples([
      { label: "Every part on every net", query: "component-on-net(?r,?n) => ?r, ?n", teaches: "projection" },
      { label: "Rails above 3V", query: "net.max_voltage(?n,?v), ?v > 3 => ?n, ?v", teaches: "filter" },
    ]);
    const chips = [...el.querySelectorAll(".query-example")].map((c) => c.textContent);
    expect(chips).toEqual(["Every part on every net", "Rails above 3V"]);
    el.querySelector<HTMLButtonElement>(".query-example")!.click();
    // Clicking fills the textarea AND runs the query.
    expect(el.querySelector<HTMLTextAreaElement>("textarea.query-text")!.value).toBe("component-on-net(?r,?n) => ?r, ?n");
    expect(onRun).toHaveBeenCalledWith("component-on-net(?r,?n) => ?r, ?n");
  });

  it("keeps the helper chrome in a drawer: closed by default, opened by the handle, closed on textarea click", () => {
    const { el, pushExamples } = mountPanel();
    pushExamples([{ label: "e", query: "component-on-net(?r,?n) => ?r", teaches: "t" }]);
    const drawer = () => el.querySelector(".query-drawer")!;
    // Examples live inside the drawer, and the drawer starts closed.
    expect(el.querySelector(".query-drawer .query-example")).not.toBeNull();
    expect(drawer().classList.contains("open")).toBe(false);
    // The left-edge handle opens it.
    el.querySelector<HTMLButtonElement>(".query-drawer-handle")!.click();
    expect(drawer().classList.contains("open")).toBe(true);
    // Clicking (pointerdown on) the textarea closes it again.
    el.querySelector<HTMLTextAreaElement>("textarea.query-text")!.dispatchEvent(new Event("pointerdown", { bubbles: true }));
    expect(drawer().classList.contains("open")).toBe(false);
  });

  it("running an example closes the drawer so the results are unobscured", () => {
    const { el, pushExamples } = mountPanel();
    pushExamples([{ label: "e", query: "component-on-net(?r,?n) => ?r", teaches: "t" }]);
    el.querySelector<HTMLButtonElement>(".query-drawer-handle")!.click();
    expect(el.querySelector(".query-drawer")!.classList.contains("open")).toBe(true);
    el.querySelector<HTMLButtonElement>(".query-example")!.click();
    expect(el.querySelector(".query-drawer")!.classList.contains("open")).toBe(false);
  });

  it("sorts result rows by a clicked column (numeric-aware), cycling asc → desc → natural", () => {
    const { el, push } = mountPanel();
    push(
      resultFromResponse({
        columns: ["n", "v"],
        rows: [
          { cells: ["A", "10"], cites: [] },
          { cells: ["B", "9"], cites: [] },
          { cells: ["C", "100"], cites: [] },
        ],
      } as never),
    );
    const vColumn = () =>
      [...el.querySelectorAll("tbody tr.query-row")].map((r) => r.querySelectorAll("td")[2].textContent);
    expect(vColumn()).toEqual(["10", "9", "100"]); // natural (server) order
    // The v header is the second sortable column header.
    const vHead = () => [...el.querySelectorAll("thead th.query-sortable")][1] as HTMLElement;
    vHead().click(); // ascending — 9 before 10 before 100, not lexicographic
    expect(vColumn()).toEqual(["9", "10", "100"]);
    expect(vHead().textContent).toContain("▲");
    vHead().click(); // descending
    expect(vColumn()).toEqual(["100", "10", "9"]);
    expect(vHead().textContent).toContain("▼");
    vHead().click(); // back to natural order
    expect(vColumn()).toEqual(["10", "9", "100"]);
    expect(vHead().textContent).not.toContain("▲");
  });

  it("keeps a row's provenance expansion attached to that row after a re-sort", () => {
    const { el, push } = mountPanel();
    push(
      resultFromResponse({
        columns: ["n", "v"],
        rows: [
          { cells: ["A", "10"], cites: ["cite-A"] },
          { cells: ["B", "9"], cites: ["cite-B"] },
        ],
      } as never),
    );
    // Expand the second natural row (B) provenance.
    [...el.querySelectorAll(".query-cite-toggle")][1].dispatchEvent(new MouseEvent("click", { bubbles: true }));
    expect([...el.querySelectorAll(".query-cites li")].map((l) => l.textContent)).toEqual(["cite-B"]);
    // Sort ascending — B moves to the top; its cite must still be the one shown.
    ([...el.querySelectorAll("thead th.query-sortable")][1] as HTMLElement).click();
    expect([...el.querySelectorAll(".query-cites li")].map((l) => l.textContent)).toEqual(["cite-B"]);
  });

  it("inserts a relation template at the caret when its chip is clicked", () => {
    const { el, pushRelations } = mountPanel();
    pushRelations([rel({ name: "component-on-net", args: ["ref_des", "net"] })]);
    const ta = el.querySelector<HTMLTextAreaElement>("textarea.query-text")!;
    // Seed some text and place the caret between the two tokens.
    ta.value = "a,  => ?r";
    ta.dispatchEvent(new Event("input", { bubbles: true }));
    ta.setSelectionRange(3, 3); // after "a, "
    el.querySelector<HTMLButtonElement>(".query-relchip")!.click();
    expect(ta.value).toBe("a, component-on-net(?ref_des, ?net) => ?r");
  });
});

// The results table used to be an auto layout with no per-column widths, so the browser sized each
// column to its widest cell: one long provenance path or net id took most of the panel and the rest
// were unreadable slivers. Columns are equal by default now, and draggable.
describe("resizable result columns", () => {
  const grips = (el: HTMLElement) => [...el.querySelectorAll<HTMLElement>(".query-col-grip")];
  const cols = (el: HTMLElement) => [...el.querySelectorAll<HTMLElement>("colgroup col")];

  function drag(grip: HTMLElement, from: number, to: number): void {
    grip.setPointerCapture = () => {};
    grip.releasePointerCapture = () => {};
    grip.dispatchEvent(new PointerEvent("pointerdown", { clientX: from, bubbles: true, cancelable: true }));
    grip.dispatchEvent(new PointerEvent("pointermove", { clientX: to, bubbles: true }));
    grip.dispatchEvent(new PointerEvent("pointerup", { clientX: to, bubbles: true }));
    // A real browser fires click after a pointerdown/up on the same element, and that click bubbles
    // to the sort handler on the header. jsdom does not synthesize it, so without this line the
    // "does not sort while resizing" case passes whether the grip swallows the click or not.
    grip.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  }

  it("gives every data column a grip and no width until one is dragged", () => {
    const h = mountPanel();
    h.push(resultFromResponse({ columns: ["?r", "?n"], rows: [{ cells: ["U1", "GND"], cites: [] }] } as never));

    expect(grips(h.el)).toHaveLength(2);
    // A colgroup entry per column plus the citation gutter; unset widths mean equal shares.
    expect(cols(h.el)).toHaveLength(3);
    expect(cols(h.el)[1].style.width).toBe("");
  });

  it("applies a dragged width to that column alone", () => {
    const h = mountPanel();
    h.push(resultFromResponse({ columns: ["?r", "?n"], rows: [{ cells: ["U1", "GND"], cites: [] }] } as never));

    drag(grips(h.el)[0], 100, 260);
    expect(cols(h.el)[1].style.width).not.toBe("");
    expect(cols(h.el)[2].style.width).toBe(""); // its neighbour is untouched
  });

  // Dragging an edge must not also re-sort: the grip sits inside the header button's click target.
  it("does not sort while resizing", () => {
    const h = mountPanel();
    h.push(resultFromResponse({ columns: ["?r", "?n"], rows: [{ cells: ["B", "x"], cites: [] }, { cells: ["A", "y"], cites: [] }] } as never));
    const before = h.el.querySelector(".query-sort-ind");

    drag(grips(h.el)[0], 100, 200);
    expect(h.el.querySelector(".query-sort-ind")).toBe(before);
  });
});

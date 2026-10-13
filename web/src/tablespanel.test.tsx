// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { TableSchema } from "./gen/agni/v1/webapi/tables_pb.js";
import { tablesPanelIsland } from "./tablespanel.jsx";
import { type TablesState, emptyTables } from "./tables.js";

// The tests here cover agni issue 898: the panel renders the tables it is handed, asks for an order
// rather than sorting, and locates a cell that names a net or a part.

function mount(state?: Partial<TablesState>) {
  const handlers = { onSource: vi.fn(), onSelect: vi.fn(), onSort: vi.fn(), onSave: vi.fn(), onLocate: vi.fn() };
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = tablesPanelIsland(el, null, handlers);
  panel.island.activate();
  if (state) panel.view.setState({ ...emptyTables(), ...state });
  return { el, ...handlers };
}

const diff = create(TableSchema, {
  name: "diff",
  columns: [{ name: "change_class" }, { name: "net", kind: "net" }, { name: "pin", kind: "pin" }],
  rows: [{ cells: ["renamed", "SDA", "U1.3"] }, { cells: ["new", "", "U2.1"] }],
});

describe("tablesPanel", () => {
  it("renders the active table's columns and rows as sent", () => {
    const { el } = mount({ source: "compare", label: "a to b", tables: [diff], compareOpen: true });
    expect([...el.querySelectorAll(".tables-table th")].map((th) => th.getAttribute("data-column"))).toEqual(["change_class", "net", "pin"]);
    expect([...el.querySelectorAll(".tables-row")].map((tr) => [...tr.querySelectorAll("td")].map((td) => td.textContent))).toEqual([
      ["renamed", "SDA", "U1.3"],
      ["new", "", "U2.1"],
    ]);
    expect(el.querySelector(".tables-count")?.textContent).toBe("2 row(s)");
  });

  it("locates a net cell, and leaves an empty cell and a pin cell as text", () => {
    const { el, onLocate } = mount({ source: "compare", tables: [diff], compareOpen: true });
    const buttons = [...el.querySelectorAll<HTMLButtonElement>(".tables-locate")];
    expect(buttons.map((b) => b.textContent)).toEqual(["SDA"]);
    buttons[0].click();
    expect(onLocate).toHaveBeenCalledWith("net", "SDA");
  });

  it("asks for an order on a header click and marks the order it was given", () => {
    const { el, onSort } = mount({ source: "compare", tables: [diff], orderBy: "-net", compareOpen: true });
    el.querySelector<HTMLElement>('th[data-column="net"]')!.click();
    expect(onSort).toHaveBeenCalledWith("net");
    expect(el.querySelector('th[data-column="net"]')?.textContent).toContain("▼");
  });

  it("offers the comparison's sources only while a comparison is open", () => {
    const sources = (el: HTMLElement) => [...el.querySelectorAll(".tables-source")].map((b) => b.getAttribute("data-source"));
    expect(sources(mount().el)).toEqual(["checks", "query"]);
    document.body.replaceChildren();
    expect(sources(mount({ compareOpen: true }).el)).toEqual(["checks", "query", "compare", "compare-a", "compare-b"]);
  });

  it("asks for the comparison again with near-renames when the box is ticked", () => {
    const { el, onSource } = mount({ source: "compare", tables: [diff], compareOpen: true });
    const box = el.querySelector<HTMLInputElement>(".tables-near-renames")!;
    box.checked = true;
    box.dispatchEvent(new Event("change", { bubbles: true }));
    expect(onSource).toHaveBeenCalledWith("compare", { nearRenames: true, equal: false });
  });

  it("switches tables and saves the one on screen", () => {
    const other = create(TableSchema, { name: "verdicts", columns: [{ name: "rule" }], rows: [] });
    const { el, onSelect, onSave } = mount({ source: "checks", tables: [diff, other] });
    el.querySelector<HTMLButtonElement>('.tables-tab[data-table="verdicts"]')!.click();
    expect(onSelect).toHaveBeenCalledWith(1);
    el.querySelector<HTMLButtonElement>(".tables-save")!.click();
    expect(onSave).toHaveBeenCalled();
  });

  it("locates by a row's own kind where the column names none", () => {
    const mixed = create(TableSchema, {
      name: "diff",
      columns: [{ name: "change_class" }, { name: "subject" }],
      rows: [
        { cells: ["component-added", "R4"], cellKinds: ["", "component"] },
        { cells: ["net-hard", "I2C_SCL"], cellKinds: ["", "net"] },
        { cells: ["note", "x"] },
      ],
    });
    const { el, onLocate } = mount({ source: "compare", tables: [mixed], compareOpen: true });
    const buttons = [...el.querySelectorAll<HTMLButtonElement>(".tables-locate")];
    expect(buttons.map((b) => [b.dataset.kind, b.textContent])).toEqual([
      ["component", "R4"],
      ["net", "I2C_SCL"],
    ]);
    buttons[1].click();
    expect(onLocate).toHaveBeenCalledWith("net", "I2C_SCL");
  });

  it("says why there is nothing to show", () => {
    const { el } = mount({ source: "checks", error: "Run the checks first." });
    expect(el.querySelector(".tables-error")?.textContent).toBe("Run the checks first.");
    expect(el.querySelector(".tables-table")).toBeNull();
  });
});

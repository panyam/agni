// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { propertiesPanelIsland } from "./propertiespanel.jsx";
import type { PropertiesState } from "./properties.js";
import type { EntityQueryItem } from "./query.js";

function mount(state: PropertiesState) {
  const onFocus = vi.fn();
  const onAsk = vi.fn();
  const onTrace = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = propertiesPanelIsland(el, null, { onFocus, onAsk, onTrace });
  panel.island.activate();
  panel.view.setState(state);
  return { el, onFocus, onAsk, onTrace };
}

const QUESTIONS: EntityQueryItem[] = [
  { kind: "component", label: "What nets does it touch?", query: "component.net(?ref, ?net) => ?net", teaches: "projection", binds: ["ref"] },
  { kind: "component", label: "Which test points reach its nets?", query: "q2", teaches: "t", binds: ["ref"] },
];

const component: PropertiesState = {
  crumbs: [
    { label: "gateway", focus: { level: "design" } },
    { label: "Power", focus: { level: "sheet", sheetId: "s1" } },
    { label: "U1", focus: { level: "entity", sel: { kind: "component", ref: "U1" }, sheetId: "s1" } },
  ],
  kind: "component",
  title: "U1",
  rows: [{ label: "mpn", value: "LM1117" }],
  groups: [
    { name: "sheets", refs: [{ label: "Power", sheet: "s1" }] },
    { name: "nets", refs: [{ label: "VIN", target: { kind: "net", net: "VIN" } }] },
    { name: "findings", refs: [], empty: "not checked yet" },
  ],
  datasheet: { kind: "no-corpus" },
  ask: { sel: { kind: "component", ref: "U1" }, questions: QUESTIONS },
};

describe("PropertiesPanel", () => {
  it("renders the breadcrumb, and a crumb moves the selection to its level", () => {
    const { el, onFocus } = mount(component);
    const crumbs = [...el.querySelectorAll<HTMLButtonElement>(".pr-crumb")];
    expect(crumbs.map((c) => c.textContent)).toEqual(["gateway", "Power", "U1"]);
    crumbs[1].click();
    expect(onFocus).toHaveBeenCalledWith({ level: "sheet", sheetId: "s1" });
  });

  it("moves to a referenced entity on the sheet it was reached from", () => {
    const { el, onFocus } = mount(component);
    el.querySelector<HTMLButtonElement>(".pr-ref-go:not(.pr-ref-sheet)")!.click();
    expect(onFocus).toHaveBeenCalledWith({ level: "entity", sel: { kind: "net", net: "VIN" }, sheetId: "s1" });
  });

  it("states an empty group rather than showing nothing", () => {
    const { el } = mount(component);
    expect([...el.querySelectorAll(".pr-empty")].map((e) => e.textContent)).toContain("not checked yet");
  });

  it("says no corpus was asked, which is not the same as a part with no spec", () => {
    const { el } = mount(component);
    expect(el.querySelector(".pr-datasheet")?.textContent).toContain("No datasheet corpus is connected");
    const { el: el2 } = mount({ ...component, datasheet: { kind: "no-spec", mpn: "LM1117" } });
    expect(el2.querySelector(".pr-datasheet")?.textContent).toContain("The corpus has no spec for LM1117");
  });

  it("moves to a sheet the selection is drawn on", () => {
    const { el, onFocus } = mount(component);
    el.querySelector<HTMLButtonElement>(".pr-ref-sheet")!.click();
    expect(onFocus).toHaveBeenCalledWith({ level: "sheet", sheetId: "s1" });
  });

  it("lists one Ask row per served question, and each asks its own question about the selection", () => {
    const { el, onAsk } = mount(component);
    const asks = [...el.querySelectorAll<HTMLButtonElement>(".pr-ask-go")];
    expect(asks.map((a) => a.textContent)).toEqual(["What nets does it touch?", "Which test points reach its nets?"]);
    expect(asks[0].title).toContain("component.net(?ref, ?net) => ?net");
    asks[1].click();
    expect(onAsk).toHaveBeenCalledWith({ kind: "component", ref: "U1" }, QUESTIONS[1]);
  });

  it("offers no Ask row where there is nothing to ask", () => {
    const { el } = mount({ ...component, ask: undefined });
    expect(el.querySelector(".pr-ask-go")).toBeNull();
  });
});

describe("tracing from the selection (agni issue 1133)", () => {
  it("offers a selected part or pin as either end of a trace", () => {
    const pin = { kind: "pin" as const, ref: "U1", pin: "3" };
    const { el, onTrace } = mount({ crumbs: [], kind: "pin", title: "U1.3", rows: [], groups: [], sel: pin });
    el.querySelector<HTMLButtonElement>(".pr-trace-from")!.click();
    expect(onTrace).toHaveBeenCalledWith("from", pin);
    el.querySelector<HTMLButtonElement>(".pr-trace-to")!.click();
    expect(onTrace).toHaveBeenLastCalledWith("to", pin);
  });

  it("offers nothing for a net, which is not an end a trace starts from", () => {
    const { el } = mount({ crumbs: [], kind: "net", title: "SDA", rows: [], groups: [], sel: { kind: "net", net: "SDA" } });
    expect(el.querySelector(".pr-trace")).toBeNull();
  });
});

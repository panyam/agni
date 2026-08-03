// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { rulesPanelIsland } from "./rulespanel.jsx";
import type { RuleItem, RulesState } from "./rules.js";

function rule(name: string, category: string, available = true): RuleItem {
  return {
    name,
    severity: "info",
    summary: `${name} one-liner`,
    impact: `${name} goes wrong`,
    detail: `## ${name}\n\nLong-form **markdown** for ${name}.`,
    reads: [],
    tags: { category, distribution: "open" },
    available,
    unavailableReason: available ? "" : "needs datasheet facts",
  };
}

const catalog: RuleItem[] = [
  rule("single-pin-net", "connectivity"),
  rule("unconnected-component", "connectivity"),
  rule("cap-voltage", "datasheet", false),
];

const state = (selected: string[] = []): RulesState => ({ rules: catalog, selected, fired: {} });

function mountPanel() {
  const onSelectionChange = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = rulesPanelIsland(el, null, { onSelectionChange });
  panel.island.activate();
  panel.view.setState(state());
  return { el, panel, onSelectionChange };
}

const q = <T extends Element>(el: HTMLElement, sel: string): T => {
  const found = el.querySelector<T>(sel);
  if (!found) throw new Error(`no element ${sel}`);
  return found;
};

const setInput = (input: HTMLInputElement, value: string) => {
  input.value = value;
  input.dispatchEvent(new Event("input", { bubbles: true }));
};

const key = (input: HTMLInputElement, k: string) =>
  input.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));

const applyBundle = (el: HTMLElement, name: string) => {
  const sel = q<HTMLSelectElement>(el, ".rules-bundle-pick select");
  sel.value = name;
  sel.dispatchEvent(new Event("change", { bubbles: true }));
};

const savedInStorage = (): unknown[] => JSON.parse(localStorage.getItem("agni.ruleBundles") ?? "[]");

beforeEach(() => {
  document.body.replaceChildren();
  localStorage.clear();
});

describe("rulespanel island bundles", () => {
  it("applying a builtin bundle fires the selection intent with resolved, available rules", () => {
    const { el, onSelectionChange } = mountPanel();
    applyBundle(el, "Open rules");
    expect(onSelectionChange).toHaveBeenCalledWith(["single-pin-net", "unconnected-component"]);
  });

  it("save flow: persists the current selection under the typed name and makes it current", () => {
    const { el, panel } = mountPanel();
    panel.view.setState(state(["single-pin-net"]));

    q<HTMLButtonElement>(el, ".rules-bundle-savebtn").click();
    const name = q<HTMLInputElement>(el, ".rules-bundle-name");
    setInput(name, "  my bundle  ");
    key(name, "Enter");

    expect(savedInStorage()).toEqual([{ name: "my bundle", rules: ["single-pin-net"] }]);
    // The saved bundle is now current: it appears in the picker and can be deleted.
    expect(q<HTMLSelectElement>(el, ".rules-bundle-pick select").value).toBe("my bundle");
    expect(el.querySelector(".rules-bundle-del")).toBeTruthy();
  });

  it("an empty name closes the save flow without persisting", () => {
    const { el } = mountPanel();
    q<HTMLButtonElement>(el, ".rules-bundle-savebtn").click();
    key(q<HTMLInputElement>(el, ".rules-bundle-name"), "Enter");
    expect(savedInStorage()).toEqual([]);
    expect(el.querySelector(".rules-bundle-name")).toBeNull();
  });

  it("escape abandons the save flow", () => {
    const { el } = mountPanel();
    q<HTMLButtonElement>(el, ".rules-bundle-savebtn").click();
    const name = q<HTMLInputElement>(el, ".rules-bundle-name");
    setInput(name, "doomed");
    key(name, "Escape");
    expect(savedInStorage()).toEqual([]);
  });

  it("delete removes the current custom bundle from storage and clears the label", () => {
    const { el, panel } = mountPanel();
    panel.view.setState(state(["single-pin-net"]));
    q<HTMLButtonElement>(el, ".rules-bundle-savebtn").click();
    const name = q<HTMLInputElement>(el, ".rules-bundle-name");
    setInput(name, "doomed");
    key(name, "Enter");
    expect(savedInStorage()).toHaveLength(1);

    q<HTMLButtonElement>(el, ".rules-bundle-del").click();
    expect(savedInStorage()).toEqual([]);
    expect(q<HTMLSelectElement>(el, ".rules-bundle-pick select").value).toBe("");
    // The delete button only exists for a current custom bundle.
    expect(el.querySelector(".rules-bundle-del")).toBeNull();
  });

  it("builtins cannot be deleted and unavailable rules cannot be selected", () => {
    const { el } = mountPanel();
    applyBundle(el, "Open rules");
    expect(el.querySelector(".rules-bundle-del")).toBeNull();

    const boxes = [...el.querySelectorAll<HTMLInputElement>(".rule-list input[type=checkbox]")];
    const unavailable = boxes.find((b) => b.closest("label")?.textContent?.includes("cap-voltage"));
    expect(unavailable?.disabled).toBe(true);
  });
});

describe("rulespanel rule prose (WS9-020)", () => {
  it("shows each rule's summary as visible subtext, not just a tooltip", () => {
    const { el } = mountPanel();
    const summaries = [...el.querySelectorAll(".rule-summary")].map((n) => n.textContent);
    expect(summaries).toContain("single-pin-net one-liner");
    expect(summaries).toHaveLength(catalog.length);
  });

  it("the detail toggle reveals the rendered markdown (with impact) and toggles back closed", () => {
    const { el } = mountPanel();
    expect(el.querySelector(".rule-detail")).toBeNull();

    const toggles = [...el.querySelectorAll<HTMLButtonElement>(".rule-detail-toggle")];
    const toggle = toggles.find((t) => t.closest("li")?.textContent?.includes("single-pin-net"));
    if (!toggle) throw new Error("no detail toggle on single-pin-net");
    toggle.click();

    const detail = q<HTMLElement>(el, ".rule-detail");
    // The markdown is rendered, not shown as source: the `##` heading becomes an h2 and the
    // `**bold**` becomes a strong element.
    expect(detail.querySelector("h2")?.textContent).toBe("single-pin-net");
    expect(detail.querySelector("strong")?.textContent).toBe("markdown");
    expect(detail.textContent).toContain("single-pin-net goes wrong");

    toggle.click();
    expect(el.querySelector(".rule-detail")).toBeNull();
  });
});

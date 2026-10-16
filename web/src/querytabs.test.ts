import { describe, it, expect } from "vitest";
import { type TabStrip, askTitle, blankTab, closeTab, openTab, tabKey, updateTab, activeTab } from "./querytabs.js";

const strip = (...ids: string[]): TabStrip => ({ tabs: ids.map((id) => blankTab(id, id)), active: ids[0] });
const ids = (s: TabStrip) => s.tabs.map((t) => t.id);

describe("query tabs", () => {
  it("opens a tab after the one in front and brings it forward", () => {
    const s = openTab({ ...strip("a", "b", "c"), active: "a" }, blankTab("n", "n"));
    expect(ids(s)).toEqual(["a", "n", "b", "c"]);
    expect(s.active).toBe("n");
  });

  it("never closes the last tab", () => {
    const s = strip("a");
    expect(closeTab(s, "a")).toBe(s);
  });

  it("brings the right-hand neighbour forward when the tab in front closes, or the left one at the end", () => {
    expect(closeTab({ ...strip("a", "b", "c"), active: "b" }, "b").active).toBe("c");
    expect(closeTab({ ...strip("a", "b", "c"), active: "c" }, "c").active).toBe("b");
  });

  it("keeps the tab in front when another closes", () => {
    const s = closeTab({ ...strip("a", "b", "c"), active: "a" }, "c");
    expect(ids(s)).toEqual(["a", "b"]);
    expect(s.active).toBe("a");
  });

  it("changes only the tab named", () => {
    const s = updateTab(strip("a", "b"), "b", { text: "x" });
    expect(s.tabs.map((t) => t.text)).toEqual(["", "x"]);
    expect(activeTab(s).id).toBe("a");
  });

  it("names a tab for the question and what it asks about", () => {
    expect(askTitle({ kind: "component", ref: "J1" }, "Which test points reach its nets?")).toBe("J1: Which test points reach its nets?");
    expect(askTitle({ kind: "pin", ref: "U7", pin: "3" }, "What else is on its net?")).toBe("U7.3: What else is on its net?");
  });

  it("keys a question by its text and bindings, whatever order the bindings came in", () => {
    expect(tabKey("q ", { a: "1", b: "2" })).toBe(tabKey("q", { b: "2", a: "1" }));
    expect(tabKey("q", { a: "1" })).not.toBe(tabKey("q", { a: "2" }));
  });
});

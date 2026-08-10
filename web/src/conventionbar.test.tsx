// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { conventionBarIsland } from "./conventionbar.jsx";
import { type ConventionState, emptyConvention, SERVER_DEFAULT_LABEL } from "./conventions.js";

function mount(over: Partial<ConventionState> = {}) {
  const onSelect = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const bar = conventionBarIsland(el, null, { onSelect });
  bar.island.activate();
  bar.view.setState({ ...emptyConvention(), ...over });
  return { el, onSelect, setState: bar.view.setState };
}

const choices = [
  { ref: "proj/house.yaml", label: "house.yaml" },
  { ref: "proj/acme.yaml", label: "acme.yaml" },
];

describe("conventionBar", () => {
  it("offers the server's convention plus every config beside the design", () => {
    const { el } = mount({ choices });
    const pick = el.querySelector(".convbar-pick") as HTMLSelectElement;
    expect(pick.options).toHaveLength(3);
    expect(pick.options[0].value).toBe("");
    expect(pick.options[0].text).toBe(SERVER_DEFAULT_LABEL);
    expect(Array.from(pick.options).slice(1).map((o) => o.value)).toEqual([
      "proj/house.yaml",
      "proj/acme.yaml",
    ]);
  });

  it("emits the chosen ref, and the empty ref for going back to the server's", () => {
    const { el, onSelect } = mount({ choices });
    const pick = el.querySelector(".convbar-pick") as HTMLSelectElement;
    pick.value = "proj/acme.yaml";
    pick.dispatchEvent(new Event("change"));
    expect(onSelect).toHaveBeenCalledWith("proj/acme.yaml");
    pick.value = "";
    pick.dispatchEvent(new Event("change"));
    expect(onSelect).toHaveBeenCalledWith("");
  });

  // The acceptance criterion from the issue: "It is clear in the UI whether you are seeing the
  // deployment's vocabulary or your own, since a finding that changed because of vocabulary and one
  // that changed because of the design are not the same claim." Asserted on the class, so it is a
  // visible state rather than a value buried in a dropdown.
  it("marks the bar when a request convention is in effect, and not when it is not", () => {
    const { el: plain } = mount({ choices });
    expect(plain.querySelector(".convbar")?.classList.contains("convbar-overridden")).toBe(false);
    expect(plain.querySelector(".convbar-active")?.textContent).toBe(SERVER_DEFAULT_LABEL);

    const { el: over } = mount({ choices, active: "proj/acme.yaml", name: "acme" });
    expect(over.querySelector(".convbar")?.classList.contains("convbar-overridden")).toBe(true);
    expect(over.querySelector(".convbar-active")?.textContent).toBe("acme");
  });

  it("names the convention, which is also the namespace its findings carry", () => {
    const { el } = mount({ choices, active: "proj/acme.yaml", name: "acme" });
    // A finding reading `acme/signal-net-naming` is only interpretable if this says `acme`.
    expect(el.textContent).toContain("acme");
  });

  it("disables the picker while a convention is resolving", () => {
    const { el } = mount({ choices, active: "proj/acme.yaml", busy: true });
    expect((el.querySelector(".convbar-pick") as HTMLSelectElement).disabled).toBe(true);
    expect(el.querySelector(".convbar-active")?.textContent).toBe("resolving…");
  });

  it("flags a resolve failure without claiming the convention is applied", () => {
    const { el } = mount({ choices, active: "", error: 'pattern "^(" does not compile' });
    expect(el.querySelector(".convbar-error")).not.toBeNull();
    expect(el.querySelector(".convbar")?.classList.contains("convbar-overridden")).toBe(false);
    expect(el.querySelector(".convbar-active")?.textContent).toBe(SERVER_DEFAULT_LABEL);
  });
});

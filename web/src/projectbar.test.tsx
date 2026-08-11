// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { projectBarIsland } from "./projectbar.jsx";
import { type ProjectState, emptyProject, NO_PROJECT_LABEL, PLAIN_LABEL } from "./project.js";

function mount(over: Partial<ProjectState> = {}) {
  const onPlain = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const bar = projectBarIsland(el, null, { onPlain });
  bar.island.activate();
  bar.view.setState({ ...emptyProject(), ...over });
  return { el, onPlain, setState: bar.view.setState };
}

const inProject: Partial<ProjectState> = {
  project: "projects/gateway",
  title: "Gateway ECU",
  design: "projects/gateway/designs/gateway",
  entry: "mount://m/d/gateway.edn",
};

describe("projectBar", () => {
  it("names the project whose config produced the answers", () => {
    const { el } = mount(inProject);
    expect(el.querySelector(".projbar-name")?.textContent).toBe("Gateway ECU");
  });

  // Blank reads as "still resolving", and belonging to no project is an ordinary answer rather than a
  // missing one: most files on a mounted folder are in no project at all.
  it("states no-project rather than going blank", () => {
    const { el } = mount({});
    expect(el.querySelector(".projbar-name")?.textContent).toBe(NO_PROJECT_LABEL);
  });

  it("offers the built-in-catalog opt-out for a design in a project, and emits the choice", () => {
    const { el, onPlain } = mount(inProject);
    const box = el.querySelector(".projbar-plain-box") as HTMLInputElement;
    expect(box).not.toBeNull();
    box.checked = true;
    box.dispatchEvent(new Event("change"));
    expect(onPlain).toHaveBeenCalledWith(true);
  });

  // The toggle answers "yours or the engine's" by subtraction. With no project there is nothing to
  // subtract, so offering the control would imply a difference that does not exist.
  it("hides the opt-out for a design that belongs to no project", () => {
    const { el } = mount({});
    expect(el.querySelector(".projbar-plain-box")).toBeNull();
  });

  // The acceptance criterion from the issue: viewing a project design under the built-in catalog is
  // visible on screen, not merely remembered from a control someone touched.
  it("says the built-in catalog is in effect, and styles that state", () => {
    const { el } = mount({ ...inProject, plain: true });
    expect(el.querySelector(".projbar-name")?.textContent).toBe(PLAIN_LABEL);
    expect(el.querySelector(".projbar")?.className).toContain("projbar-overridden");
  });

  // The served loader deliberately does not swap the entry in, so the viewer has to say that the file
  // on screen is not the one analysis reads. Without this the user is looking at a board and reading
  // findings computed from a netlist, with nothing connecting the two.
  it("surfaces that the open file is a companion rather than the design's entry", () => {
    const { el } = mount({ ...inProject, namedIsEntry: false });
    expect(el.querySelector(".projbar-entry")?.textContent).toContain("gateway.edn");
  });

  it("says nothing about the entry when the open file IS the entry", () => {
    const { el } = mount(inProject);
    expect(el.querySelector(".projbar-entry")).toBeNull();
  });

  it("shows a resolution failure inline", () => {
    const { el } = mount({ error: "mount m is not configured" });
    expect(el.querySelector(".projbar-error")?.textContent).toBe("mount m is not configured");
  });
});

// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { tracePanelIsland } from "./tracepanel.jsx";
import { propertiesPanelIsland } from "./propertiespanel.jsx";
import { WalkSchema } from "./gen/agni/v1/webapi/design_pb.js";
import { emptyWalk, walkFromResponse, type WalkRequest } from "./walk.js";

const req: WalkRequest = { start: "R1", kind: "small-parts", hops: 2, through: [] };
const walk = walkFromResponse(
  req,
  create(WalkSchema, {
    nets: [
      { hop: 1, net: { name: "SDA", sheetIds: ["s2"], stubs: [{ refDes: "TP1", pin: "1", class: "test_point" }] }, entered: { refDes: "R1", exitPin: "1" } },
      { hop: 2, net: { name: "GND", sheetIds: ["s2"] }, entered: { refDes: "C1", enterPin: "1", exitPin: "2", fromNet: "SDA" }, ended: "ground" },
    ],
    boundary: [{ refDes: "U3", class: "ic", hop: 1, net: "SDA", reason: "8 connected pins" }],
  }),
);

function mountTrace() {
  const handlers = { onWalk: vi.fn(), onLocate: vi.fn(), onClear: vi.fn() };
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = tracePanelIsland(el, null, { onTrace: vi.fn(), walk: handlers });
  panel.island.activate();
  return { el, handlers, view: panel.view };
}

const click = (el: Element | null) => (el as HTMLElement).click();

describe("the Trace tab's walk", () => {
  it("shows nothing until a walk is asked", () => {
    const { el, view } = mountTrace();
    view.setWalk(emptyWalk());
    expect(el.querySelector(".walk")).toBeNull();
  });

  it("lists the walk by hop, with the names the exercise reads", () => {
    const { el, view } = mountTrace();
    view.setWalk(walk);
    const rows = [...el.querySelectorAll("[data-walk-net]")].map((r) => `${r.getAttribute("data-hop")}:${r.getAttribute("data-walk-net")}`);
    expect(rows).toEqual(["1:SDA", "2:GND"]);
    expect(el.querySelector('[data-walk-net="GND"]')!.textContent).toContain("through C1 pin 1 to pin 2");
    expect(el.querySelector('[data-walk-net="GND"]')!.textContent).toContain("ends here: ground");
    expect(el.querySelector('[data-walk-boundary="U3"]')!.textContent).toContain("8 connected pins");
  });

  it("widens, narrows and crosses by walking again", () => {
    const { el, view, handlers } = mountTrace();
    view.setWalk(walk);
    click(el.querySelector('[data-action="walk-wider"]'));
    expect(handlers.onWalk).toHaveBeenLastCalledWith({ ...req, hops: 3 });
    click(el.querySelector('[data-action="walk-narrower"]'));
    expect(handlers.onWalk).toHaveBeenLastCalledWith({ ...req, hops: 1 });
    click(el.querySelector('[data-walk-boundary="U3"] [data-action="walk-cross"]'));
    expect(handlers.onWalk).toHaveBeenLastCalledWith({ ...req, through: ["U3"] });
  });

  it("locates a net or part on its sheet when clicked", () => {
    const { el, view, handlers } = mountTrace();
    view.setWalk(walk);
    click(el.querySelector('[data-walk-net="SDA"] .walk-net-name'));
    expect(handlers.onLocate).toHaveBeenLastCalledWith("net", "SDA", "s2");
    click(el.querySelector('[data-walk-net="SDA"] .walk-parts .walk-part'));
    expect(handlers.onLocate).toHaveBeenLastCalledWith("component", "TP1", "s2");
  });

  it("folds a hop", () => {
    const { el, view } = mountTrace();
    view.setWalk(walk);
    click(el.querySelector('[data-hop-group="1"] .walk-hop-toggle'));
    expect(el.querySelector('[data-walk-net="SDA"]')).toBeNull();
    expect(el.querySelector('[data-walk-net="GND"]')).not.toBeNull();
  });
});

describe("the walk in Properties", () => {
  it("offers each walk kind on a selected part, once the kinds are listed", () => {
    const onWalk = vi.fn();
    const el = document.createElement("div");
    document.body.appendChild(el);
    const panel = propertiesPanelIsland(el, null, { onFocus: vi.fn(), onAsk: vi.fn(), onTrace: vi.fn(), onWalk });
    panel.island.activate();
    const sel = { kind: "component" as const, ref: "R1" };
    panel.view.setState({ crumbs: [], kind: "component", title: "R1", rows: [], groups: [], sel });
    expect(el.querySelector('[data-action="walk"]')).toBeNull();
    panel.view.setWalkKinds!([
      { name: "series", title: "Series path", summary: "s" },
      { name: "small-parts", title: "Small parts", summary: "crosses small parts" },
    ]);
    const b = el.querySelector('[data-action="walk"][data-walk-kind="small-parts"]') as HTMLButtonElement;
    expect(b.title).toBe("crosses small parts");
    b.click();
    expect(onWalk).toHaveBeenCalledWith("small-parts", sel);
  });
});

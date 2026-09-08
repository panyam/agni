// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { tracePanelIsland } from "./tracepanel.jsx";
import { type TraceState, TraceOutcome, emptyTrace } from "./trace.js";

function mount(state?: TraceState) {
  const onTrace = vi.fn();
  const el = document.createElement("div");
  document.body.appendChild(el);
  const panel = tracePanelIsland(el, null, { onTrace });
  panel.island.activate();
  if (state) panel.view.setState(state);
  return { el, onTrace, view: panel.view };
}

const routed: TraceState = {
  ...emptyTrace(),
  from: { refDes: "U1", pin: "3", pinName: "SDA", net: "SDA" },
  to: { refDes: "J1", pin: "1", pinName: "VBUS", net: "VCC" },
  outcome: TraceOutcome.ROUTED,
  radius: 6,
  crossings: [{ refDes: "R1", cls: "resistor", enterPin: "2", exitPin: "1" }],
  nets: [
    { name: "SDA", stubs: [{ refDes: "TP1", pin: "1", cls: "test_point" }], stubsElided: 0, busLike: false },
    { name: "VCC", stubs: [], stubsElided: 3, busLike: true },
  ],
  ran: true,
};

describe("tracePanel", () => {
  it("says what to do before anything has been asked", () => {
    const { el } = mount();
    expect(el.textContent).toContain("Name two pins");
  });

  it("renders the route with its crossing, its probe point and its rail", () => {
    const { el } = mount(routed);
    expect(el.textContent).toContain("U1.3 (SDA) → R1 → J1.1 (VBUS)");
    expect(el.textContent).toContain("cross R1");
    expect(el.textContent).toContain("pin 2 to pin 1");
    expect(el.querySelector(".trace-stub.probe")?.textContent).toContain("TP1.1");
    expect(el.querySelector(".trace-net.bus-like .trace-net-name")?.textContent).toBe("VCC");
    expect(el.textContent).toContain("and 3 more");
    expect(el.textContent).toContain("1 crossing, 2 nets, searched to a radius of 6");
  });

  // The three outcomes render as three different things, because they are three different kinds of
  // statement. A reader shown "not connected" for a pin that does not exist goes and looks at the
  // board instead of at what they typed.
  it("says a no-route names both nets and what the walk will not cross", () => {
    const { el } = mount({ ...routed, outcome: TraceOutcome.NO_ROUTE, reason: "no series path from SDA to GND within 6 crossings" });
    expect(el.textContent).toContain("No route: U1.3 (SDA) to J1.1 (VBUS)");
    expect(el.textContent).toContain("no series path from SDA to GND");
    expect(el.textContent).toContain("A capacitor is a DC block");
    expect(el.querySelector(".trace-route")).toBeNull();
  });

  it("says an unresolved endpoint is a failed question, not a disconnection", () => {
    const { el } = mount({ ...routed, outcome: TraceOutcome.UNRESOLVED, reason: "no component U99 in this design" });
    expect(el.textContent).toContain("Cannot trace");
    expect(el.textContent).toContain("no component U99");
    expect(el.textContent).toContain("failed question");
    expect(el.textContent).not.toContain("No route");
  });

  it("hands both pins up when the reader asks", () => {
    const { el, onTrace } = mount();
    const inputs = el.querySelectorAll<HTMLInputElement>("input.trace-pin");
    inputs[0].value = " U1.3 ";
    inputs[1].value = "J1.1";
    el.querySelector<HTMLButtonElement>("button.trace-run")!.click();
    expect(onTrace).toHaveBeenCalledWith("U1.3", "J1.1");
  });

  it("disables the button while a walk is running", () => {
    const { el, view } = mount();
    view.setState({ ...emptyTrace(), loading: true, ran: true });
    expect(el.querySelector<HTMLButtonElement>("button.trace-run")!.disabled).toBe(true);
  });
});

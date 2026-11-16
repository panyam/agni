import { describe, expect, it } from "vitest";
import { zoomStep } from "./zoomstep.js";

// The board view and test point Jetson's find-a-part journey met in a laptop's window, where the
// test point came into view at under 8 px.
const view = { x: 994, y: 159, width: 734, height: 457 };

describe("how a journey's reader zooms to a part on the board", () => {
  it("zooms out about the view's middle while the part is out of view", () => {
    expect(zoomStep(view, { x: 1755, y: 122, width: 6.3, height: 6.3 })).toEqual({ x: 1361, y: 387.5, deltaY: 300 });
  });

  it("zooms in about the part itself once it is in view but too small", () => {
    expect(zoomStep(view, { x: 1652.9, y: 191, width: 4.7, height: 4.7 })).toEqual({ x: 1655.25, y: 193.35, deltaY: -300 });
  });

  it("stops once the part is in view and big enough to click", () => {
    expect(zoomStep(view, { x: 1300, y: 300, width: 12, height: 12 })).toBeNull();
  });
});

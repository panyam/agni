import { describe, it, expect } from "vitest";
import { sheetStep, stepLabel } from "./sheetstep.js";

const order = ["s1", "s2", "s3", "s4", "s5", "s6", "board"];

describe("sheetStep", () => {
  it("steps through the sheets that draw the selection in the design's order", () => {
    const drawn = ["s5", "s2", "s4"];
    expect(sheetStep(order, drawn, "s2")).toEqual({ prev: undefined, next: "s4", at: 1, of: 3 });
    expect(sheetStep(order, drawn, "s4")).toEqual({ prev: "s2", next: "s5", at: 2, of: 3 });
    expect(sheetStep(order, drawn, "s5")).toEqual({ prev: "s4", next: undefined, at: 3, of: 3 });
  });

  it("never stops on the board", () => {
    expect(sheetStep(order, ["s3", "board", "s6"], "s6")).toEqual({ prev: "s3", next: undefined, at: 2, of: 2 });
    expect(sheetStep(order, [], "s6").next).toBeUndefined();
  });

  it("goes to the first stop from the board", () => {
    expect(sheetStep(order, ["s3", "board", "s6"], "board")).toEqual({ prev: undefined, next: "s3", at: 0, of: 2 });
  });

  it("steps from a sheet that does not draw the selection to the stops either side of it", () => {
    expect(sheetStep(order, ["s2", "s5"], "s3")).toEqual({ prev: "s2", next: "s5", at: 0, of: 2 });
    expect(sheetStep(order, ["s2", "s5"], "s6")).toEqual({ prev: "s5", next: undefined, at: 0, of: 2 });
  });

  it("steps through every sheet with nothing selected", () => {
    expect(sheetStep(order, [], "s1")).toEqual({ prev: undefined, next: "s2", at: 1, of: 6 });
    expect(sheetStep(order, [], "s3")).toEqual({ prev: "s2", next: "s4", at: 3, of: 6 });
  });

  it("goes to the first sheet from a sheet the design does not list", () => {
    expect(sheetStep(order, ["s4"], "")).toEqual({ prev: undefined, next: "s4", at: 0, of: 1 });
  });
});

describe("stepLabel", () => {
  it("reads the place among the stops, or how many there are", () => {
    expect(stepLabel({ at: 3, of: 6 })).toBe("3 of 6");
    expect(stepLabel({ at: 0, of: 6 })).toBe("6 sheets");
    expect(stepLabel({ at: 0, of: 1 })).toBe("1 sheet");
    expect(stepLabel({ at: 0, of: 0 })).toBe("");
  });
});

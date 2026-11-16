import { describe, expect, it } from "vitest";
import type { Design, Journey } from "./journeys.js";
import { playedSteps, stepsBefore } from "./trysteps.js";

const d = { label: "a test board", find: "R7" } as Design;
const other = { label: "another board", find: "C1" } as Design;
const run = async () => {};
const journey: Journey = {
  name: "find-a-part",
  title: "Find a part",
  steps: [
    { caption: "Open the design", run },
    { caption: (x) => `Search for ${x.find}`, run },
    { caption: "Show the board", skip: (x) => x === other, run },
    { caption: "Click the part on the board", run },
  ],
};

describe("the steps make try plays", () => {
  it("numbers only the steps a design plays, with its own captions", () => {
    expect(playedSteps(journey, d).map((p) => `${p.n}. ${p.caption}`)).toEqual([
      "1. Open the design",
      "2. Search for R7",
      "3. Show the board",
      "4. Click the part on the board",
    ]);
    expect(playedSteps(journey, other).map((p) => `${p.n}:${p.index}`)).toEqual(["1:0", "2:1", "3:3"]);
  });

  it("plays every step without UNTIL", () => {
    expect(stepsBefore(journey, d, "")).toBe(4);
    expect(stepsBefore(journey, other, " ")).toBe(3);
  });

  it("stops before the step UNTIL numbers", () => {
    expect(stepsBefore(journey, d, "1")).toBe(0);
    expect(stepsBefore(journey, d, "3")).toBe(2);
    expect(stepsBefore(journey, other, "3")).toBe(2);
  });

  it("stops before the one step whose caption holds UNTIL's words", () => {
    expect(stepsBefore(journey, d, "search")).toBe(1);
    expect(stepsBefore(journey, d, "CLICK THE PART")).toBe(3);
  });

  it("refuses a step past the end and lists the steps", () => {
    expect(() => stepsBefore(journey, other, "4")).toThrow("UNTIL=4 is past the 3 steps of find-a-part. Its steps are:\n  1. Open the design\n  2. Search for C1\n  3. Click the part on the board");
    expect(() => stepsBefore(journey, d, "0")).toThrow("UNTIL=0 is past the 4 steps");
  });

  it("refuses words no step holds and lists the steps", () => {
    expect(() => stepsBefore(journey, d, "zoom")).toThrow("UNTIL=zoom names no step of find-a-part. Its steps are:\n  1. Open the design\n  2. Search for R7");
  });

  it("refuses words several steps hold and lists those", () => {
    expect(() => stepsBefore(journey, d, "the")).toThrow("UNTIL=the matches 3 steps of find-a-part. Name one by its number:\n  1. Open the design\n  3. Show the board\n  4. Click the part on the board");
  });
});

import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import { ComponentSchema } from "./gen/agni/v1/ir/ir_pb.js";
import type { Inventory } from "./properties.js";
import { PICK_LIMIT, matchRank, pickCandidates, tracePicks } from "./tracepick.js";

function inventory(pinsOf: Record<string, { pin: string; name: string; net: string }[]>, values: Record<string, string> = {}): Inventory {
  return {
    components: new Map(Object.keys(pinsOf).map((ref) => [ref, create(ComponentSchema, { refDes: ref, value: values[ref] ? { input: values[ref] } : undefined })])),
    nets: new Map(),
    pinsOf: new Map(Object.entries(pinsOf)),
    pinName: new Map(),
    testPoints: new Set(),
    buses: new Map(),
    unexpanded: [],
  };
}

const board = tracePicks(
  inventory(
    {
      U13: [{ pin: "1", name: "VCC", net: "3V3" }, { pin: "2", name: "GND", net: "GND" }],
      U1: [{ pin: "3", name: "SDA", net: "I2C_SDA" }, { pin: "4", name: "SCL", net: "I2C_SCL" }, { pin: "10", name: "", net: "NC_10" }],
      R2: [{ pin: "1", name: "", net: "I2C_SDA" }, { pin: "2", name: "", net: "3V3" }],
      TP1: [{ pin: "1", name: "", net: "I2C_SDA" }],
    },
    { R2: "4k7" },
  ),
);

const values = (typed: string) => pickCandidates(board, typed).map((p) => p.value);

describe("matchRank", () => {
  it("ranks exact, then prefix, then the letters in order, and refuses the rest, ignoring case", () => {
    expect(matchRank("u1", "U1")).toBe(0);
    expect(matchRank("u1", "U13")).toBe(1);
    expect(matchRank("u13", "U1.3")).toBe(2);
    expect(matchRank("u31", "U1.3")).toBe(-1);
  });
});

describe("tracePicks", () => {
  it("lists the parts in natural order with their value and pin count, and two real pins as examples", () => {
    expect(board.components).toEqual([
      { ref: "R2", detail: "4k7 · 2 pins" },
      { ref: "TP1", detail: "1 pin" },
      { ref: "U1", detail: "3 pins" },
      { ref: "U13", detail: "2 pins" },
    ]);
    expect(board.examples).toEqual(["R2.1", "U1.3"]);
  });

  it("leaves out a power or flag symbol, which is not a part", () => {
    const picks = tracePicks(inventory({ "#FLG01": [{ pin: "1", name: "", net: "VBUS" }], "#PWR03": [{ pin: "1", name: "", net: "GND" }], R1: [{ pin: "1", name: "", net: "VBUS" }] }));
    expect(picks.components.map((c) => c.ref)).toEqual(["R1"]);
    expect(pickCandidates(picks, "flg")).toEqual([]);
  });

  it("offers nothing, and no example, before the design's inventory is read", () => {
    const empty = tracePicks(null);
    expect(pickCandidates(empty, "u")).toEqual([]);
    expect(empty.examples).toEqual(["", ""]);
  });
});

describe("pickCandidates", () => {
  it("lists the parts when nothing is typed", () => {
    expect(values("")).toEqual(["R2", "TP1", "U1", "U13"]);
  });

  it("finds both U13 and U1.3 from u13, exact before prefix before a gapped match", () => {
    expect(values("u13")).toEqual(["U13", "U13.1", "U13.2", "U1.3"]);
  });

  it("lists a part before its pins on an equal match", () => {
    expect(values("U1")).toEqual(["U1", "U13", "U1.3", "U1.4", "U1.10", "U13.1", "U13.2"]);
  });

  it("finds a pin by the name a datasheet gives it", () => {
    expect(values("sda")).toEqual(["U1.3"]);
    expect(pickCandidates(board, "sda")[0]).toEqual({ value: "U1.3", kind: "pin", detail: "SDA · I2C_SDA" });
  });

  it("lists one part's pins after its ref-des and a dot, in designator order, matched on number or name", () => {
    expect(values("u1.")).toEqual(["U1.3", "U1.4", "U1.10"]);
    expect(values("U1.sc")).toEqual(["U1.4"]);
    expect(values("U1.1")).toEqual(["U1.10"]);
  });

  it(`stops at ${PICK_LIMIT} entries on a large part`, () => {
    const pins = Array.from({ length: 200 }, (_, i) => ({ pin: String(i + 1), name: "", net: `N${i}` }));
    const big = tracePicks(inventory({ U9: pins, U8: pins }));
    expect(pickCandidates(big, "U9.")).toHaveLength(PICK_LIMIT);
    expect(pickCandidates(big, "u")).toHaveLength(PICK_LIMIT);
  });
});

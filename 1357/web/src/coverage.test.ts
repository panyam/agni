import { describe, it, expect } from "vitest";
import { coverageFromResponse, presentCount } from "./coverage.js";
import type { GetInterfaceCoverageResponse } from "./gen/agni/v1/webapi/checks_pb.js";

const resp = {
  interfaces: [
    {
      profile: "SPI_NOR",
      anchorNet: "SPI_CS",
      signals: [
        { name: "CS", net: "SPI_CS", state: "pullup_missing" },
        { name: "SCLK", net: "SPI_SCLK", state: "dangling" },
        { name: "IO0", net: "SPI_IO0", state: "present" },
        { name: "IO2", net: "", state: "missing" },
      ],
    },
  ],
} as unknown as GetInterfaceCoverageResponse;

describe("coverageFromResponse", () => {
  it("maps the wire response to view items (anchorNet -> anchor)", () => {
    const s = coverageFromResponse(resp);
    expect(s.interfaces).toHaveLength(1);
    const i = s.interfaces[0];
    expect([i.profile, i.anchor]).toEqual(["SPI_NOR", "SPI_CS"]);
    expect(i.signals.map((x) => [x.name, x.net, x.state])).toEqual([
      ["CS", "SPI_CS", "pullup_missing"],
      ["SCLK", "SPI_SCLK", "dangling"],
      ["IO0", "SPI_IO0", "present"],
      ["IO2", "", "missing"],
    ]);
  });

  it("presentCount counts only fully-present signals", () => {
    expect(presentCount(coverageFromResponse(resp).interfaces[0])).toBe(1);
  });
});

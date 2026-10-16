import { describe, it, expect, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { GetDesignResponseSchema, type GetDesignResponse } from "./gen/agni/v1/webapi/design_pb.js";
import { INVENTORY_PATHS, type PropertiesState } from "./properties.js";
import { PropertiesPresenter } from "./propertiespresenter.js";

function design(name: string, nets: string[]): GetDesignResponse {
  return create(GetDesignResponseSchema, { name, design: { nets: nets.map((n) => ({ name: n })) } });
}

function harness(getDesign = vi.fn(async (_req: unknown) => design("gateway", ["VIN"]))) {
  const states: PropertiesState[] = [];
  const p = new PropertiesPresenter({ setState: (s) => states.push(s) }, { getDesign, drawn: () => ({ refs: ["U1"], nets: [] }) });
  const last = () => states[states.length - 1];
  return { p, getDesign, last };
}

const sheets = [
  { id: "s1", name: "Power" },
  { id: "s2", name: "IO" },
];

describe("PropertiesPresenter", () => {
  it("reads the design's inventory once per open, under the inventory mask", async () => {
    const { p, getDesign, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s2" });
    await vi.waitFor(() => expect(last().crumbs[0].label).toBe("gateway"));
    expect(getDesign).toHaveBeenCalledTimes(1);
    expect(getDesign.mock.calls[0][0]).toEqual({ uri: "mount://m/d", readMask: { paths: ["name", ...INVENTORY_PATHS] } });
  });

  it("selects the sheet on screen for a click on empty paper, and follows it to the next sheet", () => {
    const { p, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.selectSheet();
    expect(last().crumbs.map((c) => c.label)).toEqual(["d", "Power"]);
    expect(last().groups.find((g) => g.name === "components")?.refs.map((r) => r.label)).toEqual(["U1"]);
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s2" });
    expect(last().crumbs.map((c) => c.label)).toEqual(["d", "IO"]);
  });

  it("keeps an entity selected across a sheet change, since it names a thing rather than a place", () => {
    const { p, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.selectEntity({ kind: "net", net: "VIN" });
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s2" });
    expect(last().title).toBe("VIN");
  });

  it("returns to the design when another design opens, and drops a slow answer for the old one", async () => {
    let release: (r: GetDesignResponse) => void = () => {};
    const slow = new Promise<GetDesignResponse>((r) => (release = r));
    const getDesign = vi.fn().mockReturnValueOnce(slow).mockResolvedValueOnce(design("second", ["B"]));
    const { p, last } = harness(getDesign);
    p.setSheets({ mount: "m", path: "first", sheets, activeId: "s1" });
    p.selectEntity({ kind: "net", net: "VIN" });
    p.setSheets({ mount: "m", path: "second", sheets, activeId: "s1" });
    expect(last().kind).toBe("design");
    await vi.waitFor(() => expect(last().crumbs[0].label).toBe("second"));
    release(design("first", ["A"]));
    await Promise.resolve();
    expect(last().crumbs[0].label).toBe("second");
    expect(last().rows.find((r) => r.label === "nets")?.value).toBe("1");
  });

  it("selects the design on Escape", () => {
    const { p, last } = harness();
    p.setSheets({ mount: "m", path: "d", sheets, activeId: "s1" });
    p.selectEntity({ kind: "component", ref: "U1" });
    p.selectDesign();
    expect(last().kind).toBe("design");
  });
});

import { describe, it, expect } from "vitest";
import { parseDsUrl, dsToUrl, emptyDs, hasDatasheet } from "./dsrouter.js";

describe("dsrouter", () => {
  it("round-trips a datasheet location", () => {
    const loc = { mount: "ds", path: "ti/LM1117.pdf" };
    expect(dsToUrl(loc)).toBe("/datasheets/files/ds/ti/LM1117.pdf");
    expect(parseDsUrl(dsToUrl(loc))).toEqual(loc);
  });

  it("treats bare /datasheets as the empty location", () => {
    expect(parseDsUrl("/datasheets/")).toEqual(emptyDs());
    expect(dsToUrl(emptyDs())).toBe("/datasheets/");
  });

  it("ignores the viewer's /files/ URLs (the two codecs never cross-parse)", () => {
    expect(parseDsUrl("/files/ds/x.kicad_sch")).toEqual(emptyDs());
    expect(hasDatasheet(parseDsUrl("/files/ds/x.kicad_sch"))).toBe(false);
  });

  it("percent-encodes and decodes unsafe path characters", () => {
    const loc = { mount: "d s", path: "a b/c.pdf" };
    const url = dsToUrl(loc);
    expect(url).toBe("/datasheets/files/d%20s/a%20b/c.pdf");
    expect(parseDsUrl(url)).toEqual(loc);
  });

  it("needs a mount and at least one path segment", () => {
    expect(parseDsUrl("/datasheets/files/onlymount")).toEqual(emptyDs());
  });
});

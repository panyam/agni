import { describe, it, expect } from "vitest";
import { parseIndexedNet, sheetIdForNet } from "./netsheetguess.js";

const sheets = [
  { id: "toc", name: "TABLE OF CONTENTS" },
  { id: "ef25a", name: "HPIO -- EFUSE25 [ 00 - 03 ]" },
  { id: "ef25b", name: "HPIO -- EFUSE25 [ 04 - 05 ]" },
  { id: "oph", name: "LPIO -- OPH0750 [ 00 - 03 ]" },
];

describe("parseIndexedNet", () => {
  it("reads EFUSE bank nets", () => {
    expect(parseIndexedNet("EFUSE_25_00")).toEqual({ stem: "EFUSE25", index: 0 });
    expect(parseIndexedNet("EFUSE_25_01")).toEqual({ stem: "EFUSE25", index: 1 });
    expect(parseIndexedNet("EFUSE_25_04")).toEqual({ stem: "EFUSE25", index: 4 });
  });

  it("reads two-part channel nets", () => {
    expect(parseIndexedNet("OPH0750_02")).toEqual({ stem: "OPH0750", index: 2 });
  });
});

describe("sheetIdForNet", () => {
  it("maps every EFUSE_25_0x net to the bracketed sheet, not the TOC", () => {
    expect(sheetIdForNet("EFUSE_25_00", sheets)).toBe("ef25a");
    expect(sheetIdForNet("EFUSE_25_01", sheets)).toBe("ef25a");
    expect(sheetIdForNet("EFUSE_25_03", sheets)).toBe("ef25a");
    expect(sheetIdForNet("EFUSE_25_04", sheets)).toBe("ef25b");
  });

  it("maps OPH0750 channels by range", () => {
    expect(sheetIdForNet("OPH0750_00", sheets)).toBe("oph");
  });
});

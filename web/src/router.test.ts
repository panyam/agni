import { describe, it, expect } from "vitest";
import { emptyLocation, hasDir, hasFile, locationToUrl, parseUrl, type ViewerLocation } from "./router.js";

function loc(over: Partial<ViewerLocation> = {}): ViewerLocation {
  return { ...emptyLocation(), ...over };
}

describe("router", () => {
  it("builds a resourceful /files/ URL with the sheet and knobs in the query", () => {
    const url = locationToUrl(loc({ mount: "corpus", path: "boards/board.eds", sheet: "root", mode: "svg", layout: "grid", symbols: true }));
    expect(url).toBe("/files/corpus/boards/board.eds?sheet=root&mode=svg&layout=grid&sym=1");
  });

  it("omits absent knobs and keeps the nested path", () => {
    expect(locationToUrl(loc({ mount: "m", path: "a/b/c.kicad_sch" }))).toBe("/files/m/a/b/c.kicad_sch");
  });

  it("collapses to / when no file is open", () => {
    expect(locationToUrl(emptyLocation())).toBe("/");
  });

  it("round-trips a full location through build then parse", () => {
    const l = loc({ mount: "corpus", path: "boards/board.eds", sheet: "root", mode: "native", layout: "faithful", symbols: true });
    const url = locationToUrl(l);
    const q = url.indexOf("?");
    const parsed = parseUrl(url.slice(0, q), url.slice(q));
    expect(parsed).toEqual(l);
  });

  it("splits mount from the rest of the path and decodes segments", () => {
    const parsed = parseUrl("/files/my%20mount/deep/dir/board.eds", "");
    expect(parsed.mount).toBe("my mount");
    expect(parsed.path).toBe("deep/dir/board.eds");
  });

  it("treats a non-/files/ path as the empty location", () => {
    expect(parseUrl("/", "")).toEqual(emptyLocation());
    expect(parseUrl("/static/app.js", "")).toEqual(emptyLocation());
  });

  it("needs both a mount and a path segment, else empty", () => {
    expect(hasFile(parseUrl("/files/onlymount", ""))).toBe(false);
    expect(hasFile(parseUrl("/files/m/board.eds", ""))).toBe(true);
  });

  it("rejects a garbage ?mode= rather than passing it through", () => {
    expect(parseUrl("/files/m/board.eds", "?mode=bogus").mode).toBe("");
    expect(parseUrl("/files/m/board.eds", "?mode=webgl").mode).toBe("webgl");
  });

  it("reads sym only when it is exactly 1", () => {
    expect(parseUrl("/files/m/b.eds", "?sym=1").symbols).toBe(true);
    expect(parseUrl("/files/m/b.eds", "?sym=0").symbols).toBe(false);
    expect(parseUrl("/files/m/b.eds", "").symbols).toBe(false);
  });

  it("marks a folder with a trailing slash and carries no view knobs", () => {
    expect(locationToUrl(loc({ mount: "corpus", path: "boards/rev2", isDir: true, sheet: "root", mode: "svg" }))).toBe("/files/corpus/boards/rev2/");
  });

  it("addresses a mount root as /files/<mount>/", () => {
    expect(locationToUrl(loc({ mount: "corpus", isDir: true }))).toBe("/files/corpus/");
  });

  it("round-trips a folder through build then parse", () => {
    const l = loc({ mount: "corpus", path: "boards/rev2", isDir: true });
    expect(parseUrl(locationToUrl(l), "")).toEqual(l);
  });

  it("round-trips a mount-root folder", () => {
    const l = loc({ mount: "corpus", isDir: true });
    expect(parseUrl(locationToUrl(l), "")).toEqual(l);
  });

  it("classifies folder vs file by the trailing slash", () => {
    expect(hasDir(parseUrl("/files/corpus/boards/", ""))).toBe(true);
    expect(hasFile(parseUrl("/files/corpus/boards/", ""))).toBe(false);
    expect(hasDir(parseUrl("/files/corpus/boards/b.eds", ""))).toBe(false);
    expect(hasFile(parseUrl("/files/corpus/boards/b.eds", ""))).toBe(true);
  });

  it("drops a folder query string (folders pin no sheet or knobs)", () => {
    const parsed = parseUrl("/files/corpus/boards/", "?sheet=root&mode=svg");
    expect(parsed.sheet).toBe("");
    expect(parsed.mode).toBe("");
  });
});

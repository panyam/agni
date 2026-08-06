import { describe, it, expect } from "vitest";
import { emptyLocation, hasDir, hasFile, locationToUrl, parseUrl, type ViewerLocation } from "./router.js";

function loc(over: Partial<ViewerLocation> = {}): ViewerLocation {
  return { ...emptyLocation(), ...over };
}

describe("router", () => {
  it("builds a resourceful /designs/ URL with the sheet and knobs in the query", () => {
    const url = locationToUrl(loc({ mount: "corpus", path: "boards/board.eds", sheet: "root", mode: "svg", layout: "grid", symbols: true }));
    expect(url).toBe("/designs/corpus/boards/board.eds/view?sheet=root&mode=svg&layout=grid&sym=1");
  });

  it("omits absent knobs and keeps the nested path", () => {
    expect(locationToUrl(loc({ mount: "m", path: "a/b/c.kicad_sch" }))).toBe("/designs/m/a/b/c.kicad_sch/view");
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
    const parsed = parseUrl("/designs/my%20mount/deep/dir/board.eds/view", "");
    expect(parsed.mount).toBe("my mount");
    expect(parsed.path).toBe("deep/dir/board.eds");
  });

  it("treats a non-/designs/ path as the empty location", () => {
    expect(parseUrl("/", "")).toEqual(emptyLocation());
    expect(parseUrl("/static/app.js", "")).toEqual(emptyLocation());
  });

  it("no longer parses the retired /files/ space (the server redirects it)", () => {
    expect(parseUrl("/files/corpus/boards/board.eds", "")).toEqual(emptyLocation());
  });

  it("needs both a mount and a path segment, else empty", () => {
    expect(hasFile(parseUrl("/designs/onlymount/view", ""))).toBe(false);
    expect(hasFile(parseUrl("/designs/m/board.eds/view", ""))).toBe(true);
  });

  it("needs the trailing /view to address a design", () => {
    expect(hasFile(parseUrl("/designs/m/board.eds", ""))).toBe(false);
    expect(parseUrl("/designs/m/board.eds", "")).toEqual(emptyLocation());
  });

  it("rejects a garbage ?mode= rather than passing it through", () => {
    expect(parseUrl("/designs/m/board.eds/view", "?mode=bogus").mode).toBe("");
    expect(parseUrl("/designs/m/board.eds/view", "?mode=webgl").mode).toBe("webgl");
  });

  it("reads sym only when it is exactly 1", () => {
    expect(parseUrl("/designs/m/b.eds/view", "?sym=1").symbols).toBe(true);
    expect(parseUrl("/designs/m/b.eds/view", "?sym=0").symbols).toBe(false);
    expect(parseUrl("/designs/m/b.eds/view", "").symbols).toBe(false);
  });

  it("marks a folder with a trailing slash and carries no view knobs", () => {
    expect(locationToUrl(loc({ mount: "corpus", path: "boards/rev2", isDir: true, sheet: "root", mode: "svg" }))).toBe("/designs/corpus/boards/rev2/");
  });

  it("addresses a mount root as /designs/<mount>/", () => {
    expect(locationToUrl(loc({ mount: "corpus", isDir: true }))).toBe("/designs/corpus/");
  });

  it("round-trips a folder through build then parse", () => {
    const l = loc({ mount: "corpus", path: "boards/rev2", isDir: true });
    expect(parseUrl(locationToUrl(l), "")).toEqual(l);
  });

  it("round-trips a mount-root folder", () => {
    const l = loc({ mount: "corpus", isDir: true });
    expect(parseUrl(locationToUrl(l), "")).toEqual(l);
  });

  it("classifies folder vs design by the trailing slash and /view", () => {
    expect(hasDir(parseUrl("/designs/corpus/boards/", ""))).toBe(true);
    expect(hasFile(parseUrl("/designs/corpus/boards/", ""))).toBe(false);
    expect(hasDir(parseUrl("/designs/corpus/boards/b.eds/view", ""))).toBe(false);
    expect(hasFile(parseUrl("/designs/corpus/boards/b.eds/view", ""))).toBe(true);
  });

  it("addresses a folder literally named view, not the design under it", () => {
    // The two markers cannot collide: /view ends a design, a trailing slash ends a folder.
    expect(hasDir(parseUrl("/designs/corpus/view/", ""))).toBe(true);
    expect(parseUrl("/designs/corpus/view/", "").path).toBe("view");
  });

  it("drops a folder query string (folders pin no sheet or knobs)", () => {
    const parsed = parseUrl("/designs/corpus/boards/", "?sheet=root&mode=svg");
    expect(parsed.sheet).toBe("");
    expect(parsed.mode).toBe("");
  });
});

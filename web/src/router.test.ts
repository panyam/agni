import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it, expect } from "vitest";
import { againstTarget, bindsFromLink, dropPageUrl, keepTiming, emptyLocation, hasDir, hasFile, locationToUrl, pageFlattens, pageOf, parseUrl, setBase, withFlatten, setStaticHost, subjectFromParam, subjectParam, swapPage, type ViewerLocation } from "./router.js";

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

  it("carries the rule alongside the verdict it qualifies", () => {
    const url = locationToUrl(loc({ mount: "m", path: "b.edn", verdict: "i2c-pull-up:net:SDA", rule: "i2c-pull-up" }));
    expect(url).toBe("/designs/m/b.edn/view?verdict=i2c-pull-up%3Anet%3ASDA&rule=i2c-pull-up");
  });

  it("drops a rule with no verdict to qualify", () => {
    const url = locationToUrl(loc({ mount: "m", path: "b.edn", rule: "i2c-pull-up" }));
    expect(url).not.toContain("rule=");
  });

  it("reads the rule back out of the query", () => {
    const parsed = parseUrl("/designs/m/b.edn/view", "?verdict=i2c-pull-up%3Anet%3ASDA&rule=i2c-pull-up");
    expect(parsed.rule).toBe("i2c-pull-up");
  });

  it("reads a rule-less link as one that names no rule", () => {
    const parsed = parseUrl("/designs/m/b.edn/view", "?verdict=i2c-pull-up%3Anet%3ASDA");
    expect(parsed.rule).toBe("");
  });

  it("carries the link's revision hash alongside the verdict it qualifies", () => {
    const url = locationToUrl(loc({ mount: "m", path: "b.edn", verdict: "i2c-pull-up:net:SDA", hash: "sha256:abc" }));
    expect(url).toBe("/designs/m/b.edn/view?verdict=i2c-pull-up%3Anet%3ASDA&hash=sha256%3Aabc");
  });

  // The hash is provenance FOR a verdict, so it must not outlive one in the address bar. Every path
  // that replaces the highlight drops the verdict, and a hash left behind would keep asserting a
  // revision that nothing on screen still depends on.
  it("drops the hash when no verdict is focused", () => {
    expect(locationToUrl(loc({ mount: "m", path: "b.edn", hash: "sha256:abc" }))).toBe("/designs/m/b.edn/view");
  });

  it("round-trips a verdict link with its hash", () => {
    const l = loc({ mount: "m", path: "b.edn", verdict: "clearance:net:VBUS", hash: "sha256:deadbeef" });
    const url = locationToUrl(l);
    const q = url.indexOf("?");
    expect(parseUrl(url.slice(0, q), url.slice(q))).toEqual(l);
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

  it("carries flatten through every address the viewer writes (agni issue 1102)", () => {
    const url = locationToUrl(loc({ mount: "m", path: "b.edn", sheet: "root", flatten: true }));
    expect(url).toBe("/designs/m/b.edn/view?sheet=root&flatten=1");
    expect(parseUrl("/designs/m/b.edn/view", "?sheet=root&flatten=1").flatten).toBe(true);
    expect(parseUrl("/designs/m/b.edn/view", "?flatten=0").flatten).toBe(false);
    expect(parseUrl("/designs/m/b.edn/view", "").flatten).toBe(false);
    expect(pageFlattens("?flatten=1&timing=1")).toBe(true);
    expect(pageFlattens("?timing=1")).toBe(false);
    expect(withFlatten("/designs/m/b.edn/view?sheet=root")).toBe("/designs/m/b.edn/view?sheet=root&flatten=1");
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

describe("a trace in the URL", () => {
  // The question round-trips, which is what makes a link from the CLI and a link the panel put in
  // the address bar the same kind of thing.
  it("round-trips the pins and the radius", () => {
    const loc = { ...emptyLocation(), mount: "m", path: "b.edn", trace: "U1.3,J1.1", traceHops: 3 };
    const back = parseUrl(...splitHref(locationToUrl(loc)));
    expect(back.trace).toBe("U1.3,J1.1");
    expect(back.traceHops).toBe(3);
  });

  // The radius is only in the URL when it was pinned, so an ordinary link stays short.
  it("omits the radius when the question used the default", () => {
    const url = locationToUrl({ ...emptyLocation(), mount: "m", path: "b.edn", trace: "U1.3,J1.1", traceHops: 0 });
    expect(url).toContain("trace=U1.3%2CJ1.1");
    expect(url).not.toContain("hops=");
  });

  // A radius means nothing without the question it qualifies, the same rule `hash` follows for a
  // verdict.
  it("drops a radius with no trace beside it", () => {
    const url = locationToUrl({ ...emptyLocation(), mount: "m", path: "b.edn", trace: "", traceHops: 4 });
    expect(url).not.toContain("hops=");
  });

  // A mangled radius falls back to the default rather than to whatever Number() made of it, because
  // clamping would ask a question nobody wrote.
  it("ignores a radius that is not a positive integer", () => {
    for (const bad of ["0", "-2", "2.5", "lots", ""]) {
      expect(parseUrl("/designs/m/b.edn/view", `?trace=U1.3,J1.1&hops=${bad}`).traceHops).toBe(0);
    }
  });
});

describe("a comparison in the URL (agni issue 1126)", () => {
  // The link core/report.DiffURL writes escapes the slashes, and the router reads it back whole.
  it("reads the design to compare against from the link agni diff --server prints", () => {
    const loc = parseUrl("/designs/tut/designs/gateway/gateway.edn/view", "?against=tut%2Fdesigns%2Fgateway%2Fgateway-rev-c.edn");
    expect(loc.path).toBe("designs/gateway/gateway.edn");
    expect(againstTarget(loc.against)).toEqual({ mount: "tut", path: "designs/gateway/gateway-rev-c.edn" });
  });

  it("round-trips through build then parse", () => {
    const back = parseUrl(...splitHref(locationToUrl(loc({ mount: "m", path: "b.edn", against: "m/c.edn" }))));
    expect(back.against).toBe("m/c.edn");
  });

  // A diff that paired near-renames (agni diff --rename-approx) says so, and the link carries it only
  // beside a design to compare against, since alone it asks nothing.
  it("reads rename_approx beside against, and drops it alone", () => {
    expect(parseUrl("/designs/m/b.edn/view", "?against=m%2Fc.edn&rename_approx=1").renameApprox).toBe(true);
    expect(parseUrl("/designs/m/b.edn/view", "?against=m%2Fc.edn").renameApprox).toBe(false);
    expect(parseUrl("/designs/m/b.edn/view", "?rename_approx=1").renameApprox).toBe(false);
    const back = parseUrl(...splitHref(locationToUrl(loc({ mount: "m", path: "b.edn", against: "m/c.edn", renameApprox: true }))));
    expect(back.renameApprox).toBe(true);
    expect(locationToUrl(loc({ mount: "m", path: "b.edn", renameApprox: true }))).not.toContain("rename_approx");
  });

  it("reads a value naming no file in a mount as no design", () => {
    for (const bad of ["", "m", "m/", "/c.edn"]) expect(againstTarget(bad), bad).toBeNull();
  });
});

// splitHref splits a built URL back into the pathname+search pair parseUrl takes.
function splitHref(url: string): [string, string] {
  const i = url.indexOf("?");
  return i < 0 ? [url, ""] : [url.slice(0, i), url.slice(i)];
}

describe("the router under a base path, on a static host (agni issue 856)", () => {
  it("builds and reads every URL under the base, and reads <...>/view/ as the design", () => {
    setBase("/agni/demo/");
    setStaticHost(true);
    try {
      const loc = parseUrl("/agni/demo/designs/rb/board.kicad_sch/view/", "?sheet=root");
      expect(hasFile(loc)).toBe(true);
      expect(loc.mount).toBe("rb");
      expect(loc.path).toBe("board.kicad_sch");
      expect(loc.sheet).toBe("root");
      expect(locationToUrl(loc)).toBe("/agni/demo/designs/rb/board.kicad_sch/view?sheet=root");
      expect(hasFile(parseUrl("/designs/rb/board.kicad_sch/view", ""))).toBe(false);
      expect(locationToUrl(parseUrl("/agni/demo/", ""))).toBe("/agni/demo/");
    } finally {
      setBase("/");
      setStaticHost(false);
    }
  });
});

// agni issue 1072: a design's Load page sits beside its viewer, at the same address with the other
// verb, and carries the same query.
describe("the Load page's address", () => {
  it("reads a /load address as the design it names, with its query", () => {
    const l = parseUrl("/designs/m/boards/b.edn/load", "?verdict=v1&rule=r");
    expect(hasFile(l)).toBe(true);
    expect([l.mount, l.path, l.verdict, l.rule]).toEqual(["m", "boards/b.edn", "v1", "r"]);
    expect(locationToUrl(l, "load")).toBe("/designs/m/boards/b.edn/load?verdict=v1&rule=r");
    expect(pageOf("/designs/m/boards/b.edn/load")).toBe("load");
    expect(pageOf("/designs/m/boards/b.edn/view")).toBe("view");
    expect(pageOf("/designs/m/boards/")).toBe("");
  });

  it("swaps between the two pages keeping the query whole", () => {
    expect(swapPage("/designs/m/b.edn/view", "?timing=1&verdict=v", "load")).toBe("/designs/m/b.edn/load?timing=1&verdict=v");
    expect(swapPage("/agni/demo/designs/m/b.edn/load/", "?sheet=s", "view")).toBe("/agni/demo/designs/m/b.edn/view/?sheet=s");
  });

  // A drop opened from a page asked with ?timing keeps it, so the Load page and the viewer report
  // their timings as a seed's do.
  it("carries ?timing onto the address a drop opens, and adds nothing without it", () => {
    expect(keepTiming("/designs/local/drop-a/g/load?engine=wasm", "?engine=wasm&timing=1")).toBe("/designs/local/drop-a/g/load?engine=wasm&timing=1");
    expect(keepTiming("/designs/local/load/?d=drop-a%2Fg", "?timing=memory")).toBe("/designs/local/load/?d=drop-a%2Fg&timing=memory");
    expect(keepTiming("/designs/local/drop-a/g/load", "?engine=wasm")).toBe("/designs/local/drop-a/g/load");
  });

  it("names the drop page with a trailing slash on every host", () => {
    expect(dropPageUrl()).toBe("/designs/local/load/");
    expect(hasFile(parseUrl("/designs/local/load/", ""))).toBe(false);
  });

  // A static host serves only the pages `agni site` wrote, so a dropped design's path rides in the
  // query of the browser mount's one page.
  it("puts a dropped design's path in the query on a static host, and reads it back", () => {
    setBase("/agni/demo/");
    setStaticHost(true);
    try {
      const l = loc({ mount: "local", path: "drop-abc/gateway.zip", sheet: "s1" });
      const view = locationToUrl(l);
      expect(view).toBe("/agni/demo/designs/local/view/?d=drop-abc%2Fgateway.zip&sheet=s1");
      expect(locationToUrl(l, "load")).toBe("/agni/demo/designs/local/load/?d=drop-abc%2Fgateway.zip&sheet=s1");
      const q = view.indexOf("?");
      expect(parseUrl(view.slice(0, q), view.slice(q))).toEqual(l);
      expect(parseUrl("/agni/demo/designs/local/load/", "?d=drop-abc/gateway.zip").path).toBe("drop-abc/gateway.zip");
      // A served mount keeps its path in the address.
      expect(locationToUrl(loc({ mount: "rb", path: "b.kicad_sch" }), "load")).toBe("/agni/demo/designs/rb/b.kicad_sch/load");
    } finally {
      setBase("/");
      setStaticHost(false);
    }
  });

  it("keeps a dropped design's path in the address on a server", () => {
    expect(locationToUrl(loc({ mount: "local", path: "drop-abc/g.zip" }))).toBe("/designs/local/drop-abc/g.zip/view");
  });
});

// agni issue 634: a query link carries the question, its title and its bindings, or one query of a
// project set by name, and reads back as it was written.
describe("query links", () => {
  const reparse = (u: string) => {
    const at = u.indexOf("?");
    return parseUrl(u.slice(0, at), u.slice(at));
  };
  it("round-trips a query with its title and bindings", () => {
    const loc = { ...emptyLocation(), mount: "m", path: "b.edn", query: "component.net(?r, ?n), ?n = ?net => ?r", title: "On ground", binds: ["net=GND", 't="3"'] };
    const back = reparse(locationToUrl(loc));
    expect([back.query, back.title, back.binds]).toEqual([loc.query, loc.title, loc.binds]);
  });

  it("round-trips a set member by name, and drops a set without its query", () => {
    const back = reparse(locationToUrl({ ...emptyLocation(), mount: "m", path: "b.edn", set: "power-review", q: "Rails" }));
    expect([back.set, back.q]).toEqual(["power-review", "Rails"]);
    expect(parseUrl("/designs/m/b.edn/view", "?set=power-review").set).toBe("");
  });

  it("reads bindings as --bind spells them, a quoted value as text without its quotes", () => {
    expect(bindsFromLink(["net=GND", 't="3"', "=x", "v=3.3"])).toEqual({ net: "GND", t: "3", v: "3.3" });
  });

  // agni issue 1121: a variable bound to a list repeats its name, one bind per value.
  it("reads a name given more than once as a list in link order, and one given once as a value", () => {
    expect(bindsFromLink(["net=I2C_SDA", "r=R1", 'net="I2C_SCL"'])).toEqual({ net: ["I2C_SDA", "I2C_SCL"], r: "R1" });
  });

  it("round-trips every value of a list binding, in order", () => {
    const loc = { ...emptyLocation(), mount: "m", path: "b.edn", query: "component.net(?r, ?net) => ?r", binds: ["net=I2C_SDA", "net=I2C_SCL"] };
    const url = locationToUrl(loc);
    expect(url).toContain("bind=net%3DI2C_SDA&bind=net%3DI2C_SCL");
    const back = reparse(url);
    expect(back.binds).toEqual(loc.binds);
    expect(bindsFromLink(back.binds)).toEqual({ net: ["I2C_SDA", "I2C_SCL"] });
  });
});

// agni issue 897: a verdict link names its subjects in check.SubjectParam's spelling, which the Go
// side writes and both sides read, held to one answer by the twin fixture.
describe("a verdict link's subjects", () => {
  it("reads the twin's subjects as the Go side does, and refuses what it refuses", () => {
    const twin = JSON.parse(readFileSync(join(process.cwd(), "../core/check/testdata/subject_param_twin.json"), "utf8")) as {
      cases: { param: string; kind: string; ref: string; pin: string }[];
      bad: string[];
    };
    expect(twin.cases.length).toBeGreaterThanOrEqual(5);
    for (const c of twin.cases) {
      expect(subjectFromParam(c.param), c.param).toEqual({ kind: c.kind, ref: c.ref, pin: c.pin });
      expect(subjectParam({ kind: c.kind, subject: c.ref, pin: c.pin }), c.param).toBe(c.param);
    }
    for (const b of twin.bad) expect(subjectFromParam(b), b).toBeNull();
  });

  it("carries the subjects with their verdict, and none without one", () => {
    const l = loc({ mount: "m", path: "b.edn", verdict: "v", rule: "r", subjects: ["net:SDA", "pin:U7.3"] });
    const url = locationToUrl(l);
    expect(url).toBe("/designs/m/b.edn/view?verdict=v&rule=r&subject=net%3ASDA&subject=pin%3AU7.3");
    const q = url.indexOf("?");
    expect(parseUrl(url.slice(0, q), url.slice(q)).subjects).toEqual(["net:SDA", "pin:U7.3"]);
    expect(locationToUrl(loc({ mount: "m", path: "b.edn", subjects: ["net:SDA"] }))).not.toContain("subject=");
    expect(parseUrl("/designs/m/b.edn/view", "?subject=net:SDA").subjects).toEqual([]);
    // A spelling no scope covers is dropped rather than asked.
    expect(parseUrl("/designs/m/b.edn/view", "?verdict=v&subject=bus:D&subject=net:SDA").subjects).toEqual(["net:SDA"]);
  });
});

import type { Bindings } from "./query.js";
import type { RenderMode } from "./viewer.js";

// ViewerLocation is the URL-addressable state of the viewer: which file is open (mount+path),
// which sheet within it, and the view knobs (renderer mode, layout axis, faithful-symbol toggle),
// so a refresh or a shared link reopens where you were.
//
// A location can also address a *folder* (isDir), so a refresh reopens the tree expanded to it. A
// folder carries no sheet or view knobs and is marked in the URL by a trailing slash (see
// locationToUrl).
export interface ViewerLocation {
  mount: string;
  path: string;
  // isDir marks a folder (mount + optional sub-path). It opens no design and pins no view.
  isDir: boolean;
  sheet: string;
  // mode is a RenderMode or "" when the URL does not pin one (the presenter keeps its default).
  mode: RenderMode | "";
  layout: string;
  symbols: boolean;
  // flatten reads the design's hierarchical netlist whole (Snapshot.flatten_hierarchy, agni issue
  // 1102). It is a property of the page rather than of one question, since every snapshot the page
  // ingests carries it (pageFlattens), so it rides every address the viewer writes.
  flatten: boolean;
  // verdict is the id of the focused verdict ("i2c-pull-up:net:SDA"), "" when none.
  //
  // SELECTION STATE, like `sheet`, and not the resource form. A server-resolved `GET /checks/<id>`
  // would need a fixed-arity design identity, and `/designs/<mount>/<path...>/` has a variable-length
  // path, so a two-segment verb suffix cannot be told from a folder without the trailing slash.
  verdict: string;
  // hash is the design revision the LINK was computed against ("sha256:<hex>"), "" when it named
  // none. It qualifies `verdict` ("this verdict was concluded about these bytes") and means nothing
  // without one.
  //
  // PROVENANCE the viewer checks and reports on, never one it mints, since a hash is worth comparing
  // only because whoever wrote the link put it there.
  hash: string;
  // rule is the rule name behind `verdict`, "" when the link named none. Like `hash`, it means
  // nothing without the verdict.
  //
  // A HINT rather than part of the address, so the viewer resolves the verdict by running one rule
  // instead of the catalog (1.95s against 21.78s on a 3980-component board, paid per click). The
  // verdict id stays the identity, so a link with no rule, including every saved report, still
  // resolves by the full run.
  //
  // Not derived from the id, deliberately. VerdictID is generated and never parsed, because its
  // escaping is what lets a subject ref carry its own colons and commas.
  rule: string;
  // subjects are the verdict's nets, parts and pins in check.SubjectParam's spelling (`net:SDA`,
  // `pin:U7.3`), empty when the link named none (agni issue 897). Like `rule`, a HINT: the viewer
  // asks that one rule about these subjects alone rather than running it over the whole design, and a
  // link without them resolves by the whole rule. They mean nothing without the verdict.
  subjects: string[];
  // trace is a pin-to-pin question the viewer should ask on arrival, "U1.3,J1.1", "" when none.
  //
  // It carries the QUESTION rather than an answer, so unlike `verdict` it needs no `hash`. Two pin
  // names are re-asked against whatever the design is now, and a pin that has since gone is already
  // an outcome the panel states, so a staleness banner has nothing to add.
  trace: string;
  // traceHops is the search radius the question was asked at, 0 when the URL names none (the server
  // default applies).
  //
  // It means nothing without `trace`. A link that dropped it would re-ask a NARROWER question
  // wider, so a run that said "no route within 2 crossings" could link to a route.
  traceHops: number;
  // query is a datalog question the viewer should ask on arrival, in a query tab, "" when none (agni
  // issue 634). Like `trace` it carries the QUESTION, so it needs no `hash`. title names the tab and
  // binds are its bound values as `--bind` spells them (`n=GND`, `t="3"`), so the link asks the
  // question it was minted for rather than a wider one. A variable bound to a list repeats its name,
  // one bind per value in order, and the whole list is kept back into the URL. Both mean nothing
  // without `query`.
  query: string;
  title: string;
  binds: string[];
  // set and q open one query of the design's project query sets by name (agni issue 634), so a
  // link to a long query stays short and follows the set's current text. Both or neither.
  set: string;
  q: string;
  // against is a design to compare the open one against on arrival, as "<mount>/<path>", "" when
  // none (agni issue 1126). It starts the Compare the picker starts, so `agni diff --server` and an
  // MCP diff answer can link to a comparison. Read on arrival and not written back, like `query`,
  // since the comparison is the reader's own from then on.
  against: string;
  // renameApprox asks that comparison's tables to pair near-renames, as the diff behind the link did
  // (`rename_approx=1`, from `agni diff --rename-approx` or the MCP tool's rename_approx).
  renameApprox: boolean;
}

// base is the path the app is served under, "/" on `agni serve` and "/agni/demo/" on the static
// demo (agni issue 856). The page writes it into the shell (data-base on .app) and the boot code
// passes it to setBase before anything builds or reads a URL.
let base = "/";

// setBase sets the path every URL the router builds or reads sits under. It always ends in "/".
export function setBase(b: string): void {
  base = b.endsWith("/") ? b : `${b}/`;
}

// staticHost marks a page served as plain files (agni issue 856). Such a host serves a design's
// page as <...>/view/index.html, at an address ending "/view/", and browses no folders, so on it
// that address is the design rather than a folder named "view".
let staticHost = false;

// setStaticHost turns on reading "<...>/view/" as a design, for a page served as plain files.
export function setStaticHost(on: boolean): void {
  staticHost = on;
}

// appBase is the path the app is served under, for code building URLs outside the router.
export function appBase(): string {
  return base;
}

function designsPrefix(): string {
  return `${base}designs/`;
}

// VIEW_SEGMENT terminates a design's work-page URL, telling a FILE location from a FOLDER one
// without depending on the path's shape. A folder ends in "/" and a design ends in "/view". The
// segment carries no state; it reserves room for per-verb routes (/checks, /query) without
// migrating the URL space again.
const VIEW_SEGMENT = "view";

// LOAD_SEGMENT ends a design's Load page, the page that reads a design into the engine's store
// before the viewer opens it (agni issue 1072). It carries the same query string as /view, which it
// hands on when the design is read.
const LOAD_SEGMENT = "load";

// A DesignPage is one of the two pages a design's URL can end in.
export type DesignPage = typeof VIEW_SEGMENT | typeof LOAD_SEGMENT;

// DROP_PARAM carries a dropped design's path within the browser mount on a static host. Such a host
// serves only the pages `agni site` wrote, so a design under designs/local/<drop>/... has no page of
// its own there, and its path rides in the query of the browser mount's one page instead.
const DROP_PARAM = "d";

// BROWSER_MOUNT is the mount dropped files go into, the same name as wasm/drop.ts's.
const BROWSER_MOUNT = "local";

// emptyLocation is the "nothing open" location ("/"): no file, no folder, no view knobs.
export function emptyLocation(): ViewerLocation {
  return { mount: "", path: "", isDir: false, sheet: "", mode: "", layout: "", symbols: false, flatten: false, verdict: "", hash: "", rule: "", subjects: [], trace: "", traceHops: 0, query: "", title: "", binds: [], set: "", q: "", against: "", renameApprox: false };
}

// hasFile reports whether a location names a file to open (mount and path both set, and it is
// not a folder).
export function hasFile(loc: ViewerLocation): boolean {
  return !loc.isDir && loc.mount !== "" && loc.path !== "";
}

// hasDir reports whether a location names a folder to reveal (a mount, with an optional sub-path;
// the mount root is path ""). It and hasFile are mutually exclusive.
export function hasDir(loc: ViewerLocation): boolean {
  return loc.isDir && loc.mount !== "";
}

// isRenderMode narrows an untrusted URL value to a RenderMode, so a hand-edited ?mode= can't
// reach the presenter as garbage (it collapses to "" and the presenter keeps its default).
function isRenderMode(s: string | null): s is RenderMode {
  return s === "webgl" || s === "svg" || s === "native";
}

// locationToUrl renders a location as a root-relative URL (pathname + search). A design's work
// page lives at /designs/<mount>/<path...>/view, with the sheet and view knobs in the query string
// because sheet ids and layout names are not guaranteed path-safe. A folder is the same path with a
// trailing slash (/designs/<mount>/ for a mount root) and no query. With neither it is "/".
export function locationToUrl(loc: ViewerLocation, page: DesignPage = VIEW_SEGMENT): string {
  if (hasDir(loc)) {
    const segs = [loc.mount, ...loc.path.split("/")].filter((s) => s !== "").map(encodeURIComponent);
    return designsPrefix() + segs.join("/") + "/";
  }
  if (!hasFile(loc)) return base;
  const segs = [loc.mount, ...loc.path.split("/")].filter((s) => s !== "").map(encodeURIComponent);
  const params = new URLSearchParams();
  if (loc.sheet) params.set("sheet", loc.sheet);
  if (loc.mode) params.set("mode", loc.mode);
  if (loc.layout) params.set("layout", loc.layout);
  if (loc.symbols) params.set("sym", "1");
  if (loc.flatten) params.set(FLATTEN_PARAM, "1");
  if (loc.verdict) params.set("verdict", loc.verdict);
  // Only alongside its verdict, or the address bar keeps asserting a provenance nothing on screen
  // depends on.
  if (loc.verdict && loc.hash) params.set("hash", loc.hash);
  // Only alongside its verdict, or the next run gets scoped to a rule nothing on screen asked about.
  if (loc.verdict && loc.rule) params.set("rule", loc.rule);
  if (loc.verdict) for (const sub of loc.subjects) params.append("subject", sub);
  if (loc.trace) params.set("trace", loc.trace);
  // Only alongside its trace, and only when it is not the server default.
  if (loc.trace && loc.traceHops > 0) params.set("hops", String(loc.traceHops));
  if (loc.query) {
    params.set("query", loc.query);
    if (loc.title) params.set("title", loc.title);
    for (const b of loc.binds) params.append("bind", b);
  }
  if (loc.set && loc.q) {
    params.set("set", loc.set);
    params.set("q", loc.q);
  }
  if (loc.against) params.set("against", loc.against);
  if (loc.against && loc.renameApprox) params.set("rename_approx", "1");
  if (staticHost && loc.mount === BROWSER_MOUNT) {
    const all = new URLSearchParams([[DROP_PARAM, loc.path], ...params]);
    return `${designsPrefix()}${BROWSER_MOUNT}/${page}/?${all.toString()}`;
  }
  const q = params.toString();
  return designsPrefix() + segs.join("/") + "/" + page + (q ? `?${q}` : "");
}

// FLATTEN_PARAM is the query parameter asking for a design's hierarchical netlist read whole.
export const FLATTEN_PARAM = "flatten";

// pageFlattens reports whether the page at search asks for hierarchical netlists read whole. The Load
// page and the viewer share the address's query, so both ingest the same way.
export function pageFlattens(search: string): boolean {
  return new URLSearchParams(search).get(FLATTEN_PARAM) === "1";
}

// withFlatten is url with the page asking for its design read whole, every other parameter kept.
export function withFlatten(url: string): string {
  const u = new URL(url, "http://x");
  u.searchParams.set(FLATTEN_PARAM, "1");
  return u.pathname + u.search;
}

// keepTiming carries the address's ?timing onto url, for a page that moves the reader to another of
// its own pages. The Load page hands its query to the viewer whole, so a drop opened with ?timing
// keeps it through both (agni issue 914's timings, lost on a drop's way to its Load page until this).
export function keepTiming(url: string, search: string): string {
  const t = new URLSearchParams(search).get("timing");
  if (t === null) return url;
  const u = new URL(url, "http://x");
  u.searchParams.set("timing", t);
  return u.pathname + u.search;
}

// dropPageUrl is where files are dropped or picked, the browser mount's Load page with no design. It
// ends in a slash on every host, since a static host serves it as a folder's index.html.
export function dropPageUrl(): string {
  return `${designsPrefix()}${BROWSER_MOUNT}/${LOAD_SEGMENT}/`;
}

// swapPage is the address of the other page of the design at pathname+search, the query kept whole
// (a static host's dropped design names its path there).
export function swapPage(pathname: string, search: string, page: DesignPage): string {
  const slash = pathname.endsWith("/");
  const p = slash ? pathname.slice(0, -1) : pathname;
  const at = p.lastIndexOf("/");
  return `${p.slice(0, at + 1)}${page}${slash ? "/" : ""}${search}`;
}

// pageOf says which of a design's two pages a pathname is, or "" for one that is neither.
export function pageOf(pathname: string): DesignPage | "" {
  const p = pathname.endsWith("/") ? pathname.slice(0, -1) : pathname;
  if (p.endsWith(`/${VIEW_SEGMENT}`)) return VIEW_SEGMENT;
  if (p.endsWith(`/${LOAD_SEGMENT}`)) return LOAD_SEGMENT;
  return "";
}

// parseUrl reads a location back out of a pathname+search pair (as read from window.location). The
// first path segment is the mount and the rest, rejoined, is the path within it. A trailing slash
// marks a folder (mount root is /designs/<mount>/, path ""), and a trailing /view marks a design,
// which needs at least one path segment and carries the knobs from the query. Anything else yields
// the empty location.
export function parseUrl(pathname: string, search: string): ViewerLocation {
  const loc = emptyLocation();
  if (!pathname.startsWith(designsPrefix())) return loc;
  let rest = pathname.slice(designsPrefix().length);
  if (staticHost && (rest.endsWith(`/${VIEW_SEGMENT}/`) || rest.endsWith(`/${LOAD_SEGMENT}/`))) rest = rest.slice(0, -1);
  const isDir = rest.endsWith("/");
  const segs = rest
    .split("/")
    .filter((s) => s !== "")
    .map(decodeURIComponent);
  if (isDir) {
    if (segs.length < 1) return loc; // need at least a mount
    loc.mount = segs[0];
    loc.path = segs.slice(1).join("/");
    loc.isDir = true;
    return loc; // folders carry no sheet or view knobs
  }
  const last = segs[segs.length - 1];
  if (last !== VIEW_SEGMENT && last !== LOAD_SEGMENT) return loc; // not a design URL
  segs.pop();
  const params = new URLSearchParams(search);
  // A dropped design on a static host names its path in the query (see DROP_PARAM).
  if (staticHost && segs.length === 1 && segs[0] === BROWSER_MOUNT && params.get(DROP_PARAM)) segs.push(...(params.get(DROP_PARAM) ?? "").split("/"));
  if (segs.length < 2) return loc; // a design needs a mount and at least one path segment
  loc.mount = segs[0];
  loc.path = segs.slice(1).join("/");
  loc.sheet = params.get("sheet") ?? "";
  const mode = params.get("mode");
  loc.mode = isRenderMode(mode) ? mode : "";
  loc.layout = params.get("layout") ?? "";
  loc.symbols = params.get("sym") === "1";
  loc.flatten = params.get(FLATTEN_PARAM) === "1";
  loc.verdict = params.get("verdict") ?? "";
  loc.hash = params.get("hash") ?? "";
  loc.rule = params.get("rule") ?? "";
  loc.subjects = loc.verdict ? params.getAll("subject").filter((sub) => subjectFromParam(sub) !== null) : [];
  loc.trace = params.get("trace") ?? "";
  // A hops that is not a positive integer is DROPPED rather than clamped, so a mangled radius falls
  // back to the server default instead of asking a question nobody wrote.
  const hops = Number(params.get("hops"));
  loc.traceHops = Number.isInteger(hops) && hops > 0 ? hops : 0;
  loc.query = params.get("query") ?? "";
  loc.title = loc.query ? (params.get("title") ?? "") : "";
  loc.binds = loc.query ? params.getAll("bind") : [];
  const set = params.get("set") ?? "";
  const member = params.get("q") ?? "";
  loc.set = set && member ? set : "";
  loc.q = set && member ? member : "";
  loc.against = params.get("against") ?? "";
  loc.renameApprox = loc.against !== "" && params.get("rename_approx") === "1";
  return loc;
}

// againstTarget reads an `against` value as the design it names, the mount up to the first slash and
// the path after it, or null for one naming no file in a mount.
export function againstTarget(against: string): { mount: string; path: string } | null {
  const at = against.indexOf("/");
  if (at <= 0 || at === against.length - 1) return null;
  return { mount: against.slice(0, at), path: against.slice(at + 1) };
}

// currentLocation reads the browser's current URL into a ViewerLocation.
export function currentLocation(): ViewerLocation {
  return parseUrl(window.location.pathname, window.location.search);
}

// A SubjectRef is one subject a scoped check is asked about, as the wire's checks.Subject spells it.
export interface SubjectRef {
  kind: "net" | "component" | "pin";
  ref: string;
  pin: string;
}

// subjectFromParam reads check.SubjectParam's spelling, `net:NAME`, `component:REF` or `pin:REF.PIN`,
// or answers null for one naming nothing a scope covers. The kind ends at the first colon and a pin
// splits at its last dot, as on the Go side, which core/check/testdata/subject_param_twin.json holds
// both readers to.
export function subjectFromParam(s: string): SubjectRef | null {
  const at = s.indexOf(":");
  if (at < 0) return null;
  const kind = s.slice(0, at);
  const ref = s.slice(at + 1);
  if (!ref) return null;
  if (kind === "net" || kind === "component") return { kind, ref, pin: "" };
  if (kind !== "pin") return null;
  const dot = ref.lastIndexOf(".");
  if (dot <= 0 || dot === ref.length - 1) return null;
  return { kind: "pin", ref: ref.slice(0, dot), pin: ref.slice(dot + 1) };
}

// subjectParam spells a subject for a link, "" for a kind no scope covers.
export function subjectParam(s: { kind: string; subject: string; pin: string }): string {
  if (s.kind === "net" || s.kind === "component") return `${s.kind}:${s.subject}`;
  if (s.kind === "pin" && s.pin) return `pin:${s.subject}.${s.pin}`;
  return "";
}

// bindsFromLink reads a link's `bind` values, each `name=value` as `--bind` spells it, into the query
// panel's chips. A quoted value is text with its quotes dropped, as ParseBinding reads it; a pair with
// no name is skipped. A name given more than once is bound to the list of its values in link order
// (agni issue 1121), and a name given once stays a single value.
export function bindsFromLink(binds: string[]): Bindings {
  const seen = new Map<string, string[]>();
  for (const b of binds) {
    const at = b.indexOf("=");
    const name = (at < 0 ? "" : b.slice(0, at)).trim().replace(/^\?/, "");
    if (!name) continue;
    let value = b.slice(at + 1);
    if (value.length >= 2 && value.startsWith('"') && value.endsWith('"')) value = value.slice(1, -1);
    seen.set(name, [...(seen.get(name) ?? []), value]);
  }
  const out: Bindings = {};
  for (const [name, values] of seen) out[name] = values.length === 1 ? values[0] : values;
  return out;
}

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

// emptyLocation is the "nothing open" location ("/"): no file, no folder, no view knobs.
export function emptyLocation(): ViewerLocation {
  return { mount: "", path: "", isDir: false, sheet: "", mode: "", layout: "", symbols: false, verdict: "", hash: "", rule: "", trace: "", traceHops: 0 };
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
export function locationToUrl(loc: ViewerLocation): string {
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
  if (loc.verdict) params.set("verdict", loc.verdict);
  // Only alongside its verdict, or the address bar keeps asserting a provenance nothing on screen
  // depends on.
  if (loc.verdict && loc.hash) params.set("hash", loc.hash);
  // Only alongside its verdict, or the next run gets scoped to a rule nothing on screen asked about.
  if (loc.verdict && loc.rule) params.set("rule", loc.rule);
  if (loc.trace) params.set("trace", loc.trace);
  // Only alongside its trace, and only when it is not the server default.
  if (loc.trace && loc.traceHops > 0) params.set("hops", String(loc.traceHops));
  const q = params.toString();
  return designsPrefix() + segs.join("/") + "/" + VIEW_SEGMENT + (q ? `?${q}` : "");
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
  if (staticHost && rest.endsWith(`/${VIEW_SEGMENT}/`)) rest = rest.slice(0, -1);
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
  if (segs[segs.length - 1] !== VIEW_SEGMENT) return loc; // not a design URL
  segs.pop();
  if (segs.length < 2) return loc; // a design needs a mount and at least one path segment
  loc.mount = segs[0];
  loc.path = segs.slice(1).join("/");
  const params = new URLSearchParams(search);
  loc.sheet = params.get("sheet") ?? "";
  const mode = params.get("mode");
  loc.mode = isRenderMode(mode) ? mode : "";
  loc.layout = params.get("layout") ?? "";
  loc.symbols = params.get("sym") === "1";
  loc.verdict = params.get("verdict") ?? "";
  loc.hash = params.get("hash") ?? "";
  loc.rule = params.get("rule") ?? "";
  loc.trace = params.get("trace") ?? "";
  // A hops that is not a positive integer is DROPPED rather than clamped, so a mangled radius falls
  // back to the server default instead of asking a question nobody wrote.
  const hops = Number(params.get("hops"));
  loc.traceHops = Number.isInteger(hops) && hops > 0 ? hops : 0;
  return loc;
}

// currentLocation reads the browser's current URL into a ViewerLocation.
export function currentLocation(): ViewerLocation {
  return parseUrl(window.location.pathname, window.location.search);
}

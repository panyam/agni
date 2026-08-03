import type { RenderMode } from "./viewer.js";

// ViewerLocation is the URL-addressable state of the viewer: which file is open (mount+path),
// which sheet within it, and the view knobs (renderer mode, layout axis, faithful-symbol
// toggle). It is the bridge between the presenter's state and the browser URL, so a refresh or
// a shared link reopens exactly where you were instead of dropping back to the empty "/" shell.
//
// A location can also address a *folder* rather than a file (isDir): selecting a folder in the
// tree is a valid, shareable place too, so a refresh reopens the tree expanded to it. A folder
// carries no sheet or view knobs (it renders nothing); it is distinguished on the wire by a
// trailing slash (see locationToUrl).
export interface ViewerLocation {
  mount: string;
  path: string;
  // isDir marks the location as a folder (mount + optional sub-path) rather than a file. A folder
  // is addressable so the tree can reopen expanded to it, but it opens no design and pins no view.
  isDir: boolean;
  sheet: string;
  // mode is a RenderMode or "" when the URL does not pin one (the presenter keeps its default).
  mode: RenderMode | "";
  layout: string;
  symbols: boolean;
}

const FILES_PREFIX = "/files/";

// emptyLocation is the "nothing open" location ("/"): no file, no folder, no view knobs.
export function emptyLocation(): ViewerLocation {
  return { mount: "", path: "", isDir: false, sheet: "", mode: "", layout: "", symbols: false };
}

// hasFile reports whether a location names a file to open (mount and path both set, and it is
// not a folder).
export function hasFile(loc: ViewerLocation): boolean {
  return !loc.isDir && loc.mount !== "" && loc.path !== "";
}

// hasDir reports whether a location names a folder to reveal (a mount, with an optional sub-path;
// the mount root is path ""). Its counterpart to hasFile — the two are mutually exclusive.
export function hasDir(loc: ViewerLocation): boolean {
  return loc.isDir && loc.mount !== "";
}

// isRenderMode narrows an untrusted URL value to a RenderMode, so a hand-edited ?mode= can't
// reach the presenter as garbage (it collapses to "" and the presenter keeps its default).
function isRenderMode(s: string | null): s is RenderMode {
  return s === "webgl" || s === "svg" || s === "native";
}

// locationToUrl renders a location as a root-relative URL (pathname + search). A file lives at
// /files/<mount>/<path...> so the address is resourceful (a design has a stable, shareable URL);
// the sheet and view knobs ride in the query string because sheet ids and layout names are
// opaque strings that are not guaranteed path-safe. A folder is the same path with a trailing
// slash (/files/<mount>/<dir>/, or /files/<mount>/ for a mount root) and no query, mirroring the
// classic "directories end in /" convention. With neither it collapses to "/".
export function locationToUrl(loc: ViewerLocation): string {
  if (hasDir(loc)) {
    const segs = [loc.mount, ...loc.path.split("/")].filter((s) => s !== "").map(encodeURIComponent);
    return FILES_PREFIX + segs.join("/") + "/";
  }
  if (!hasFile(loc)) return "/";
  const segs = [loc.mount, ...loc.path.split("/")].filter((s) => s !== "").map(encodeURIComponent);
  const params = new URLSearchParams();
  if (loc.sheet) params.set("sheet", loc.sheet);
  if (loc.mode) params.set("mode", loc.mode);
  if (loc.layout) params.set("layout", loc.layout);
  if (loc.symbols) params.set("sym", "1");
  const q = params.toString();
  return FILES_PREFIX + segs.join("/") + (q ? `?${q}` : "");
}

// parseUrl reads a location back out of a pathname+search pair (as read from window.location). A
// path that is not under /files/ (or that lacks a mount) yields the empty location. The first
// path segment is the mount; the rest, rejoined, is the path within it. A trailing slash marks a
// folder (mount root is /files/<mount>/, path ""); otherwise it is a file, which needs at least
// one path segment and carries the sheet/view knobs from the query.
export function parseUrl(pathname: string, search: string): ViewerLocation {
  const loc = emptyLocation();
  if (!pathname.startsWith(FILES_PREFIX)) return loc;
  const rest = pathname.slice(FILES_PREFIX.length);
  const isDir = rest.endsWith("/"); // the trailing slash is the folder marker
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
  if (segs.length < 2) return loc; // a file needs a mount and at least one path segment
  loc.mount = segs[0];
  loc.path = segs.slice(1).join("/");
  const params = new URLSearchParams(search);
  loc.sheet = params.get("sheet") ?? "";
  const mode = params.get("mode");
  loc.mode = isRenderMode(mode) ? mode : "";
  loc.layout = params.get("layout") ?? "";
  loc.symbols = params.get("sym") === "1";
  return loc;
}

// currentLocation reads the browser's current URL into a ViewerLocation.
export function currentLocation(): ViewerLocation {
  return parseUrl(window.location.pathname, window.location.search);
}

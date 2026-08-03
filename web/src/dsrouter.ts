// The datasheets workbench's URL <-> state codec (WS13-006), the analogue of router.ts for the
// viewer. It is a pure functions module with no page imports, so it stays testable and the viewer
// router never has to know about the datasheets space. A datasheet lives at
// /datasheets/files/<mount>/<path...>, mirroring the viewer's /files/<mount>/<path...> so the two
// pages address resources the same way. parseDsUrl returns the empty location for any path outside
// the datasheets space (including the viewer's /files/), so the two codecs never mis-parse each
// other's URLs.

// DsLocation is the URL-addressable state of the workbench: which datasheet is open. Empty
// mount+path is the bare /datasheets landing (no datasheet selected).
export interface DsLocation {
  mount: string;
  path: string;
}

const DS_FILES_PREFIX = "/datasheets/files/";

// emptyDs is the no-datasheet-open location (the /datasheets landing).
export function emptyDs(): DsLocation {
  return { mount: "", path: "" };
}

// hasDatasheet reports whether a location names a datasheet to open (both mount and path set).
export function hasDatasheet(loc: DsLocation): boolean {
  return loc.mount !== "" && loc.path !== "";
}

// dsToUrl renders a location as a root-relative URL. A datasheet lives at
// /datasheets/files/<mount>/<path...>, each segment percent-encoded so paths with spaces or other
// unsafe characters round-trip. The empty location is the /datasheets landing.
export function dsToUrl(loc: DsLocation): string {
  if (!hasDatasheet(loc)) return "/datasheets/";
  const segs = [loc.mount, ...loc.path.split("/")].filter((s) => s !== "").map(encodeURIComponent);
  return DS_FILES_PREFIX + segs.join("/");
}

// parseDsUrl reads a location back out of a pathname. A path not under /datasheets/files/ (the
// bare /datasheets landing, or any viewer URL) yields the empty location. The first segment is the
// mount; the rest, rejoined, is the mount-relative datasheet path (a datasheet needs both).
export function parseDsUrl(pathname: string): DsLocation {
  const loc = emptyDs();
  if (!pathname.startsWith(DS_FILES_PREFIX)) return loc;
  const rest = pathname.slice(DS_FILES_PREFIX.length);
  const segs = rest.split("/").filter((s) => s !== "").map(decodeURIComponent);
  if (segs.length < 2) return loc; // a datasheet needs a mount and at least one path segment
  loc.mount = segs[0];
  loc.path = segs.slice(1).join("/");
  return loc;
}

// currentDs reads the workbench location from the browser's address bar.
export function currentDs(): DsLocation {
  return parseDsUrl(window.location.pathname);
}

// Artifact URIs, browser side (agni issue 177).
//
// The wire names an artifact with one string, `mount://<mount>/<path>`. The viewer keeps a mount and
// a path as separate pieces of its own state, and deliberately keeps doing so: they are separate in
// the URL the user can bookmark (`/designs/<mount>/<path...>/view`), and the file tree navigates by
// path within a mount. This module is the seam between the two, so the assembly happens in one place
// rather than at each of the call sites that address an artifact.
//
// Nothing here validates. Containment is decided by the server when it parses the URI, and a browser
// that pre-judged it would either duplicate that rule or, worse, disagree with it.

const PREFIX = "mount://";

// artifactUri assembles the wire name for an artifact from the mount and mount-relative path the
// viewer holds. An empty path names the mount root, which is what a directory listing of a whole
// mount asks for.
export function artifactUri(mount: string, path: string): string {
  const clean = path.replace(/^\/+/, "").replace(/\/+$/, "");
  return clean === "" ? PREFIX + mount : `${PREFIX}${mount}/${clean}`;
}

// uriMount reads the authority back out, "" when the value is not an artifact URI.
export function uriMount(uri: string): string {
  if (!uri.startsWith(PREFIX)) return "";
  const rest = uri.slice(PREFIX.length);
  const slash = rest.indexOf("/");
  return slash < 0 ? rest : rest.slice(0, slash);
}

// uriPath reads the mount-relative path back out, "" for a mount root. A value that is not an
// artifact URI is returned unchanged, so a stored document written before this still renders.
export function uriPath(uri: string): string {
  if (!uri.startsWith(PREFIX)) return uri;
  const rest = uri.slice(PREFIX.length);
  const slash = rest.indexOf("/");
  return slash < 0 ? "" : rest.slice(slash + 1);
}

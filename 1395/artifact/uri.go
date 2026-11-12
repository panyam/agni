// Package artifact defines how the engine NAMES a stored artifact. An artifact is a design, a board
// export, a datasheet, a checklist, or anything else the injected Loader resolves to bytes.
//
// A name is one string, in one field.
//
//	mount://boards/designs/gateway/gateway.edn
//
// It replaces a `(mount, path)` pair that 24 request messages carried as two fields (#179).
//
// # Why the authority is always `mount`
//
// Not `s3://` or `db://`. The authority is the mount NAME, a key in a server-defined namespace, and
// the deployment decides what it resolves to. A per-store scheme would put the storage kind in the
// client's hands. The Loader and ProjectStore ports exist to prevent that.
//
// # Why a URI rather than a path
//
// The mount is the containment boundary and `..` cannot escape it. Related files resolve against
// each other through one specified operation, such as a KiCad schematic naming its sub-sheets or a
// symbol library referenced from inside a file. A URI path is always slash-separated.
// `formats.Loader` warns about a platform-separator split, and that split cannot break a sibling
// lookup on Windows.
//
// Grouping is NOT expressed here. A design's relationship to its board companion is DECLARED in a
// `design.yaml`, not encoded in a path or inferred from a stem.
package artifact

import (
	"fmt"
	"path"
	"strings"
)

// Scheme is the only scheme an artifact URI uses. See the package doc for why it is fixed.
const Scheme = "mount"

const prefix = Scheme + "://"

// URI is a parsed artifact URI. It holds its mount and the mount-relative path within that mount.
//
// A parsed URI is a CONTAINED one. Parse rejects anything that escapes its mount, so callers can
// pass a value of this type around without re-checking containment.
type URI struct {
	// Mount is the authority, a key in a server-defined namespace and never a host path.
	Mount string
	// Path is the mount-relative path, cleaned, slash-separated, with no leading slash. The mount
	// ROOT is the empty string. A directory listing of a whole mount asks for that path.
	Path string
}

// New builds a URI from a mount and a mount-relative path, applying the same containment rules as
// Parse. Code holding the two halves calls it, such as an adapter reading a config flag or a test.
// That code then cannot assemble a string that Parse would reject.
func New(mount, p string) (URI, error) {
	if mount == "" {
		return URI{}, fmt.Errorf("artifact uri: mount is required")
	}
	if strings.ContainsAny(mount, "/\\") {
		return URI{}, fmt.Errorf("artifact uri: mount %q must not contain a separator", mount)
	}
	clean, err := cleanPath(p)
	if err != nil {
		return URI{}, err
	}
	return URI{Mount: mount, Path: clean}, nil
}

// Parse reads an artifact URI.
//
// Parse refuses a bare path, an absolute path, a different scheme, and a path escaping its mount.
// Each is an error rather than a best-effort reading, because this is the containment boundary. A
// permissive parse here is a path traversal everywhere above it. The error names the broken rule,
// since these arrive from a wire and a client needs to fix its own call.
func Parse(s string) (URI, error) {
	rest, ok := strings.CutPrefix(s, prefix)
	if !ok {
		if other, _, found := strings.Cut(s, "://"); found {
			return URI{}, fmt.Errorf("artifact uri %q: scheme %q is not supported, want %q", s, other, Scheme)
		}
		return URI{}, fmt.Errorf("artifact uri %q: missing %q prefix (an artifact is named %s<mount>/<path>, never a bare or host path)", s, prefix, prefix)
	}
	mount, p, _ := strings.Cut(rest, "/")
	return New(mount, p)
}

// String renders the URI. It round-trips with Parse.
func (u URI) String() string {
	if u.Path == "" {
		return prefix + u.Mount
	}
	return prefix + u.Mount + "/" + u.Path
}

// IsZero reports whether the URI names nothing. An absent optional field reads as a zero URI.
func (u URI) IsZero() bool { return u.Mount == "" && u.Path == "" }

// Resolve returns the URI naming `rel` interpreted RELATIVE to this one, the way a browser resolves
// a relative href against the page it is on.
//
// A reader calls this when a schematic names its sub-sheets or a symbol library. A reference is
// resolved against this URI's DIRECTORY, so resolving "sub/sheet1.kicad_sch" against
// ".../designs/gateway/gateway.kicad_sch" yields ".../designs/gateway/sub/sheet1.kicad_sch".
//
// A result outside the mount is an error, so a file naming "../../etc/passwd" cannot leave the
// mount it was read from.
func (u URI) Resolve(rel string) (URI, error) {
	switch {
	case strings.HasPrefix(rel, prefix):
		// A full URI reference replaces everything, mount included. Containment is still enforced,
		// because Parse enforces it.
		return Parse(rel)
	case strings.HasPrefix(rel, "/"):
		// An absolute-path reference resolves against the AUTHORITY rather than the current
		// directory. RFC 3986 gives that rule, and a browser applies it to a leading-slash href.
		// The authority IS the mount, so this names something else in the same mount.
		//
		// This deviates from RFC 3986 on purpose. Its remove_dot_segments SILENTLY DISCARDS a
		// leading `..` on an absolute path, so "/../x" would normalize to "/x". Here it is an error
		// instead, the same as a relative reference walking above the root. Silently reinterpreting
		// a traversal attempt as a valid path hides the bug.
		return New(u.Mount, strings.TrimPrefix(rel, "/"))
	default:
		return New(u.Mount, path.Join(u.Dir().Path, rel))
	}
}

// Dir returns the URI of the directory containing this one, or the mount root for a top-level entry.
func (u URI) Dir() URI {
	if u.Path == "" {
		return u
	}
	d := path.Dir(u.Path)
	if d == "." {
		d = ""
	}
	return URI{Mount: u.Mount, Path: d}
}

// Base returns the final path element, the way path.Base does, and "" for a mount root.
func (u URI) Base() string {
	if u.Path == "" {
		return ""
	}
	return path.Base(u.Path)
}

// Join returns the URI of a child of this one.
func (u URI) Join(elem ...string) (URI, error) {
	return New(u.Mount, path.Join(append([]string{u.Path}, elem...)...))
}

// cleanPath normalizes a mount-relative path and enforces containment. Cleaning happens BEFORE the
// escape check, so "a/../../b" is caught by what it resolves to rather than how it was spelled.
func cleanPath(p string) (string, error) {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("artifact uri: path %q must be relative to the mount, not absolute", p)
	}
	if p == "" {
		return "", nil
	}
	clean := path.Clean(p)
	switch {
	case clean == "." || clean == "/":
		return "", nil
	case clean == ".." || strings.HasPrefix(clean, "../"):
		return "", fmt.Errorf("artifact uri: path %q escapes its mount", p)
	}
	return strings.TrimPrefix(clean, "./"), nil
}

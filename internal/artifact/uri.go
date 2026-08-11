// Package artifact defines how the engine NAMES a stored artifact: a design, a board export, a
// datasheet, a checklist, anything the injected Loader resolves to bytes.
//
// One string, one field:
//
//	mount://boards/designs/gateway/gateway.edn
//
// It replaces the `(mount, path)` PAIR every request used to carry. The pair worked, and the loader
// was indifferent to which form it got, but nothing above the loader was: two fields that mean one
// thing must travel together, and 24 request messages repeated that pairing by hand.
//
// # Why the authority is always `mount`
//
// Not `s3://` or `db://`. The authority is the mount NAME, which is a key in a server-defined
// namespace, and what that key resolves to is the deployment's business. A per-store scheme would
// put the storage kind in the client's hands, which is precisely the indirection the Loader and
// ProjectStore ports exist to keep. The scheme is fixed so that the URI carries exactly the
// information the pair carried, and no more.
//
// # Why a URI rather than a path
//
// Containment is unchanged: the mount is still the boundary and `..` still cannot escape it. What
// changes is that relative resolution between related files becomes a specified operation instead of
// a hand-rolled one. A KiCad schematic naming its sub-sheets, a symbol library referenced from
// inside a file: those are resolved RELATIVE to the file being read, and a URI path is always
// slash-separated, so the platform-separator split that `formats.Loader` warns about ("invisible on
// unix, where the two agree, and breaks every sibling lookup on Windows") has nowhere to hide.
//
// Grouping is NOT expressed here. A design's relationship to its board companion is DECLARED in a
// `design.yaml`, not encoded in a path or inferred from a stem. Addressing and grouping are separate
// jobs and this type only does the first.
package artifact

import (
	"fmt"
	"path"
	"strings"
)

// Scheme is the only scheme an artifact URI uses. See the package doc for why it is fixed.
const Scheme = "mount"

const prefix = Scheme + "://"

// URI is a parsed artifact URI: the mount it lives in, and the mount-relative path within it.
//
// A parsed URI is a CONTAINED one. Parse rejects anything that escapes its mount, so a value of this
// type cannot name a location outside the mount it claims — which is what lets callers pass it
// around without re-checking, and what makes the boundary a property of the type rather than a step
// somebody remembers.
type URI struct {
	// Mount is the authority: a key in a server-defined namespace, never a host path.
	Mount string
	// Path is the mount-relative path, cleaned, slash-separated, with no leading slash. The mount
	// ROOT is the empty string, which is what a directory listing of a whole mount asks for.
	Path string
}

// New builds a URI from a mount and a mount-relative path, applying the same containment rules as
// Parse. It exists so code holding the two halves (an adapter reading a config flag, a test) cannot
// assemble a string that Parse would then reject.
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
// It is strict on purpose. A bare path, an absolute path, a different scheme, or a path escaping its
// mount are all errors rather than best-effort readings, because this is the containment boundary:
// a permissive parse here is a path traversal everywhere above it. The error says which rule was
// broken, since these arrive from a wire and a client needs to fix its own call.
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

// IsZero reports whether the URI names nothing, which is how an absent optional field reads.
func (u URI) IsZero() bool { return u.Mount == "" && u.Path == "" }

// Resolve returns the URI naming `rel` interpreted RELATIVE to this one, the way a browser resolves
// a relative href against the page it is on.
//
// This is the operation a reader performs when a schematic names its sub-sheets or a symbol library:
// the reference is written relative to the file that contains it, and only the reader's own position
// says what it means. A reference is resolved against this URI's DIRECTORY, so resolving
// "sub/sheet1.kicad_sch" against ".../designs/gateway/gateway.kicad_sch" yields
// ".../designs/gateway/sub/sheet1.kicad_sch".
//
// The result stays inside the mount or it is an error: a file that names "../../etc/passwd" is a
// file trying to read outside the boundary its own mount established, and being inside a design is
// not authority to leave it.
func (u URI) Resolve(rel string) (URI, error) {
	switch {
	case strings.HasPrefix(rel, prefix):
		// A full URI reference replaces everything, mount included. Containment is still enforced,
		// because Parse enforces it.
		return Parse(rel)
	case strings.HasPrefix(rel, "/"):
		// An absolute-path reference resolves against the AUTHORITY rather than the current
		// directory, which is RFC 3986's rule and the one a browser applies to a leading-slash href.
		// The authority IS the mount, so this names something else in the same mount.
		//
		// One deliberate deviation: RFC 3986's remove_dot_segments SILENTLY DISCARDS a leading `..`
		// on an absolute path, so "/../x" would normalize to "/x". Here it is an error instead, the
		// same as a relative reference walking above the root. Both spellings are a reference trying
		// to leave the mount, and quietly reinterpreting one of them as a different valid path is
		// exactly what makes a traversal bug hard to see.
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
// escape check, so "a/../../b" is caught: the check is on what the path resolves to, never on how it
// was spelled.
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

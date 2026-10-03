package profiles

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"

	"github.com/panyam/agni/core/check"
)

// LoadDir loads every *.yaml / *.yml profile in dir (non-recursive), sorted by filename for
// deterministic order. It is the file-facing entry point for overlay/customer profiles (the CLI
// --profile-path flag); the os dependency lives here, isolated from the WASM-clean Parse/Load path.
// An empty or missing dir is an error (a caller that wants "optional" checks existence first).
func LoadDir(dir string) ([]Profile, error) {
	// Checked here rather than by wrapping LoadFS's error, so only an unreadable directory names the
	// flag and a malformed profile keeps its own message.
	if _, err := os.ReadDir(dir); err != nil {
		return nil, fmt.Errorf("profiles: reading --profile-path %q: %w", dir, err)
	}
	return LoadFS(os.DirFS(dir), ".")
}

// LoadFS is LoadDir over dir inside fsys, for a host whose profiles are not on the host filesystem
// (a project mounted in memory in the wasm engine).
func LoadFS(fsys fs.FS, dir string) ([]Profile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ext := path.Ext(e.Name()); ext == ".yaml" || ext == ".yml" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []Profile
	for _, name := range names {
		f, err := fsys.Open(path.Join(dir, name))
		if err != nil {
			return nil, err
		}
		p, err := Load(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("profiles: %s: %w", name, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// Source compiles the given profiles into a named check.RuleSource so the CLI can splice overlay
// profiles into the catalog via check.CatalogWith, the same way --conventions does. The name is the
// namespace prefix and must differ from the built-in "profile" source (CatalogWith rejects a
// duplicate source name); the CLI uses "profile-overlay" so an overlay rule is distinguishable from
// a built-in one by its namespace.
// It SUPERSEDES rather than augments, so an overlay profile carrying the name of a built-in one
// replaces that built-in's rules in the composed catalog (WS3-056). Augmenting was WS3-054's v0 and it
// produces false failures, not just duplicates. A naming map that re-binds some roles and leaves others
// at core naming lets the CORE profile still anchor and clear its in-use gate, so it reports every
// re-bound role as a missing signal at severity error while the overlay reads the same design clean.
// Re-binding the ANCHOR role hides the effect, because the core profile stops anchoring and goes quiet,
// so the closer a house convention sits to the core one, the more false failures augmenting generates.
//
// Supersession is keyed on the profile NAME and scoped to the built-in "profile" source. Same
// interface name means same interface, so the overlay's reading of it wins. Scoped to the built-ins,
// two overlay sources that both define an interface do not delete each other, which under a
// source-agnostic rule would depend on composition order.
func Source(name string, ps []Profile) check.RuleSource {
	var rules []*check.Rule
	var supersedes []check.Facets
	for _, p := range ps {
		rules = append(rules, Compile(p)...)
		if _, isBuiltin := ByName(p.Name); isBuiltin {
			supersedes = append(supersedes, check.Facets{
				Tags: map[string][]string{
					TagProfile:      {p.Name},
					check.KeySource: {BuiltinSourceName},
				},
			})
		}
	}
	if len(supersedes) == 0 {
		return check.NewSource(name, rules)
	}
	return check.NewSupersedingSource(name, rules, supersedes...)
}

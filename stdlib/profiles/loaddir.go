package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/panyam/agni/core/check"
)

// LoadDir loads every *.yaml / *.yml profile in dir (non-recursive), sorted by filename for
// deterministic order. It is the file-facing entry point for overlay/customer profiles (the CLI
// --profile-path flag); the os dependency lives here, isolated from the WASM-clean Parse/Load path.
// An empty or missing dir is an error (a caller that wants "optional" checks existence first).
func LoadDir(dir string) ([]Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("profiles: reading --profile-path %q: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ext := filepath.Ext(e.Name()); ext == ".yaml" || ext == ".yml" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []Profile
	for _, name := range names {
		f, err := os.Open(filepath.Join(dir, name))
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
func Source(name string, ps []Profile) check.RuleSource {
	var rules []*check.Rule
	for _, p := range ps {
		rules = append(rules, Compile(p)...)
	}
	return check.NewSource(name, rules)
}

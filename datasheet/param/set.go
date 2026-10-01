package param

import (
	"fmt"
	"io/fs"
	"strings"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// ParamProvider is the one call the model makes to reach a part's seeded datasheet spec, given
// its MPN. A part's ratings are the same for every design that uses the MPN, so the source is
// pluggable rather than tied to one on-disk corpus. Backends are ParamSet (a textproto corpus
// loaded from a directory) and ProviderFunc (an in-memory mock), and a shared datasheet service
// would slot in behind this one method with no rule or model change.
type ParamProvider interface {
	// Lookup returns the spec seeded for an MPN, or nil when the part is unseeded or unknown.
	// A nil return keeps datasheet-backed rules silent by construction (skip, never false-pass).
	Lookup(mpn string) *parampb.PartSpec
}

// ProviderFunc adapts a plain lookup function to ParamProvider (the http.HandlerFunc pattern),
// for in-memory mocks and tests: param.ProviderFunc(func(mpn string) *param.PartSpec { ... }).
type ProviderFunc func(mpn string) *parampb.PartSpec

// Lookup calls the wrapped function.
func (f ProviderFunc) Lookup(mpn string) *parampb.PartSpec { return f(mpn) }

// ParamSet is a seeded parameter corpus: PartSpecs indexed by upper-cased MPN, and the
// directory-backed ParamProvider. The spec-side half of the WS10-003 validation join (design
// component -> PartSpec). A nil or empty ParamSet is valid and means nothing is seeded, so every
// Lookup misses.
type ParamSet map[string]*parampb.PartSpec

// Lookup returns the spec seeded for an MPN, or nil. Matching is case-insensitive
// because vendor and BOM casing of the same MPN routinely differ. There is no other
// normalization (no suffix stripping, no package-code fuzzing), because a near-miss MPN
// is a different part until a human says otherwise.
func (s ParamSet) Lookup(mpn string) *parampb.PartSpec {
	if s == nil || mpn == "" {
		return nil
	}
	return s[strings.ToUpper(mpn)]
}

// LoadSet walks fsys for *.textproto PartSpecs, validates each, and indexes them by
// MPN. It is all-or-nothing. Any file that fails to parse or Validate, and any two
// files claiming the same MPN, fail the whole load with the offending file named, so a
// bad spec never silently shrinks the corpus.
func LoadSet(fsys fs.FS) (ParamSet, error) {
	set := ParamSet{}
	from := map[string]string{}
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".textproto") {
			return nil
		}
		f, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		spec, err := Load(f)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := Validate(spec); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		key := strings.ToUpper(spec.Mpn)
		if prev, dup := from[key]; dup {
			return fmt.Errorf("%s: duplicate spec for mpn %q (already loaded from %s)", path, spec.Mpn, prev)
		}
		set[key], from[key] = spec, path
		return nil
	})
	if err != nil {
		return nil, err
	}
	return set, nil
}

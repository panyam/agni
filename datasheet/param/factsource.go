package param

import (
	"sort"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// FactSource is the read-all sibling of ParamProvider.Lookup: it yields the WHOLE seeded spec library, not a
// single MPN. It exists so the datalog surface can query the spec library as a fact base (`param(?mpn, ...)`
// across every seeded part) instead of only the parts joined to one design. A directory-backed
// ParamSet implements it; a future service backend implements it over its store. Kept separate from
// ParamProvider because a keyed remote Lookup is cheap while an enumerate-all may not be — a backend
// opts into library-wide datalog by implementing this.
type FactSource interface {
	// AllSpecs returns every seeded PartSpec, ordered by MPN for deterministic query output.
	AllSpecs() []*parampb.PartSpec
}

// AllSpecs returns the corpus's PartSpecs sorted by upper-cased MPN key (the map's index), so a
// library-wide query prints in a stable order.
func (s ParamSet) AllSpecs() []*parampb.PartSpec {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*parampb.PartSpec, 0, len(keys))
	for _, k := range keys {
		out = append(out, s[k])
	}
	return out
}

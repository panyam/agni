package param

import (
	"sort"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// FactSource is the read-all sibling of ParamProvider.Lookup. It yields the WHOLE seeded spec library,
// so datalog can query `param.max(?mpn, ...)` across every seeded part rather than only the parts
// joined to one design. It is separate from ParamProvider because a keyed remote Lookup is cheap and
// an enumerate-all may not be, so a backend opts into library-wide datalog by implementing this.
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

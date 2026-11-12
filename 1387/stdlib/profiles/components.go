package profiles

import (
	"github.com/panyam/agni/core/check"
)

// Components returns the RefDes of the parts on interface p's nets, the component analogue of Nets.
// A per-interface datasheet ask joins on the component because an interface chip sits on both its
// signal nets and its power rails. So a component-subject finding (e.g. rail-nominal-out-of-recommended
// on the memory die) is kept when the part is on one of the interface's nets, even though the rail
// carries no interface suffix. It shares Nets' single walk (scope), so the host-beats-convention
// precedence is the same.
//
// An empty result means present but unmatched, a clean pass. Absence is InUse's job (see present.go).
func Components(m check.Model, p Profile) map[string]bool {
	_, comps := scope(m, p)
	return comps
}

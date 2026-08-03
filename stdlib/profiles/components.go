package profiles

import (
	"github.com/panyam/agni/core/check"
)

// Components returns the set of component RefDes that belong to interface p on this design — the parts
// on the interface's nets. It is the component analogue of Nets, and the component IS the join a
// per-interface datasheet ask needs: an interface chip sits on both its signal nets and its power
// rails, so a component-subject finding (e.g. rail-nominal-out-of-recommended on the memory die) is
// kept for the interface when the flagged part is on one of the interface's nets, even though the rail
// net itself carries no interface suffix. It shares Nets' single design walk (via scope), so it
// inherits the same host-beats-convention precedence and costs one linear pass, not two.
//
// Presence is a SEPARATE concern (Present): an absent interface is marked not-applicable before any
// filtering, so an empty result here means "present but no part matched", a clean pass for the ask.
func Components(m check.Model, p Profile) map[string]bool {
	_, comps := scope(m, p)
	return comps
}

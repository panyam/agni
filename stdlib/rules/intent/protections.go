package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
)

// Protection kinds.
const (
	ProtectionOVP       = "ovp"       // a TVS or zener clamps the rail
	ProtectionDischarge = "discharge" // a resistor bridges the rail to ground (a bleeder)
)

// protectionRule builds the rule for ONE protection kind, covering every declared protection of that
// kind. One rule per kind (intent/protection-ovp, intent/protection-discharge) so distinct review items
// bind independently. For each declared rail it probes the design topology on that EXACT net (not the
// rail-role heuristic, which misses names like VBATT01): ovp requires a TVS/zener among the rail's
// components; discharge requires a resistor that also touches a ground net. A declared rail with no
// matching device fails (a KindNet finding on the rail). Iterates the DECLARATION and probes the
// design — never derives the protected-rail set from the netlist.
func protectionRule(kind string, ps []Protection) *check.Rule {
	return &check.Rule{
		Name:     "protection-" + kind,
		Severity: "warning",
		Summary:  fmt.Sprintf("a rail the design intent declares needs %s protection has none", kind),
		Impact:   "a power rail the design was intended to protect (OV clamp / discharge path) lacks the protection device",
		Reads:    []string{"component-on-net", "component.class", "net.ground"},
		Tags:     intentTags(),
		Eval: func(m check.Model) []check.Finding {
			var out []check.Finding
			for _, p := range ps {
				if p.Kind != kind {
					continue
				}
				if protected(m, p) {
					continue
				}
				out = append(out, check.Finding{
					Kind:    check.KindNet,
					Subject: p.Rail,
					Message: fmt.Sprintf("rail %q declares %s protection, but no %s device is on it", p.Rail, kind, protectionDevice(kind)),
				})
			}
			return out
		},
	}
}

// protected reports whether the declared protection is realized on its rail net.
func protected(m check.Model, p Protection) bool {
	rail := componentsOnNet(m, p.Rail)
	switch p.Kind {
	case ProtectionOVP:
		for ref := range rail {
			if m.HasClass(ref, check.ComponentClass("tvs")) || m.HasClass(ref, check.ComponentClass("zener")) {
				return true
			}
		}
	case ProtectionDischarge:
		gnd := groundRefs(m)
		for ref := range rail {
			if gnd[ref] && m.HasClass(ref, check.ComponentClass("resistor")) {
				return true
			}
		}
	}
	return false
}

// componentsOnNet returns the ref-des set connected to the net named name (empty if the net is absent).
func componentsOnNet(m check.Model, name string) map[string]bool {
	out := map[string]bool{}
	for _, n := range m.Nets() {
		if n.GetName() != name {
			continue
		}
		for _, c := range n.GetConnections() {
			out[c.GetComponentRef()] = true
		}
	}
	return out
}

// groundRefs returns the ref-des set that touches any ground net (name-derived, the same predicate the
// net.ground fact uses), so a discharge check can ask "does this resistor also reach ground".
func groundRefs(m check.Model) map[string]bool {
	out := map[string]bool{}
	for _, n := range m.Nets() {
		if !classify.ActiveRoleVocab().IsGround(n.GetName()) {
			continue
		}
		for _, c := range n.GetConnections() {
			out[c.GetComponentRef()] = true
		}
	}
	return out
}

// protectionDevice names the device a kind looks for, for the finding message.
func protectionDevice(kind string) string {
	if kind == ProtectionDischarge {
		return "bleeder resistor (rail-to-ground)"
	}
	return "TVS/zener clamp"
}

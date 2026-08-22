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
// kind, named intent/protection-<kind> (protection-ovp, protection-discharge). For each declared rail
// it probes the design topology on that EXACT net (not the rail-role heuristic, which misses names like
// VBATT01): ovp requires a TVS/zener among the rail's components; discharge requires a resistor that
// also touches a ground net. A declared rail with no matching device fails (a KindNet finding on the
// rail).
func protectionRule(kind string, ps []Protection) *check.Rule {
	return &check.Rule{
		Name:                "protection-" + kind,
		Severity:            "warning",
		Summary:             fmt.Sprintf("a rail the design intent declares needs %s protection has none", kind),
		Detail:              intentDoc("protection-" + kind),
		Impact:              "a power rail the design was intended to protect (OV clamp / discharge path) lacks the protection device",
		Remedy:              intentRemedy("protection-" + kind),
		Reads:               []string{"component-on-net", "component.class", "net.ground"},
		Tags:                intentTags(),
		Eval:                func(m check.Model) []check.Verdict { return protectionVerdicts(m, ps, kind) },
		StatesConsideredSet: true,
	}
}

// protectionVerdicts decides every declared protection OF THIS KIND. A declaration of another kind
// belongs to a sibling rule and yields no verdict here, since these families compile to one rule per
// kind and a pass about someone else's declaration would claim a check that never happened.
//
// The pass is what a protection declaration is for: "the OV clamp this rail was intended to have is on
// it" is the sentence the intent file was written to get back, and the rule said nothing at all when
// the answer was yes.
func protectionVerdicts(m check.Model, ps []Protection, kind string) []check.Verdict {
	var out []check.Verdict
	for _, p := range ps {
		if p.Kind != kind {
			continue // another kind's declaration: not a subject of this rule
		}
		v := check.Verdict{Subjects: []check.Entity{check.NetNameEntity(p.Rail)}}
		if protected(m, p) {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("a %s device sits on rail %q, as declared", protectionDevice(kind), p.Rail),
				Terms:     []check.WitnessTerm{{Label: "declared protection", Value: kind}},
			}
		} else {
			v.Outcome = check.Fail
			msg := fmt.Sprintf("rail %q declares %s protection, but no %s device is on it", p.Rail, kind, protectionDevice(kind))
			v.Witness = &check.Witness{Statement: fmt.Sprintf("no %s device is on rail %q", protectionDevice(kind), p.Rail)}
			v.Finding = &check.Finding{Subject: check.NetNameEntity(p.Rail), Message: msg}
		}
		out = append(out, v)
	}
	return out
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

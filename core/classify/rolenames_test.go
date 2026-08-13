package classify

import (
	"reflect"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestStampNetRoles: the ingestion pass stamps each net's role SET from the active naming lexicon — a
// rail, a ground, a feedback node, a plain signal (no roles), and a rail-NAMED feedback node that
// carries BOTH roles (precedence is the consumer's call, so the stamp records both).
func TestStampNetRoles(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "+3V3"}, {Name: "GND"}, {Name: "AMP_FB"}, {Name: "SDA"}, {Name: "VCC1V2_FB"},
	}}
	StampNetRoles(d)
	want := map[string][]string{
		"+3V3":      {NetRoleRail},
		"GND":       {NetRoleGround},
		"AMP_FB":    {NetRoleFeedback}, // no rail prefix, only the _FB feedback suffix
		"SDA":       nil,
		"VCC1V2_FB": {NetRoleRail, NetRoleFeedback}, // a rail-NAMED feedback node carries both
	}
	for _, n := range d.Nets {
		if got := RoleTokens(n); !reflect.DeepEqual(got, want[n.Name]) {
			t.Errorf("roles(%q) = %v, want %v", n.Name, got, want[n.Name])
		}
	}
}

// TestStampNetRolesIdempotent: a re-stamp (a re-read of the same design) overwrites rather than
// accumulates, so the set stays correct.
func TestStampNetRolesIdempotent(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{{Name: "GND"}}}
	StampNetRoles(d)
	StampNetRoles(d)
	if got := RoleTokens(d.Nets[0]); !reflect.DeepEqual(got, []string{NetRoleGround}) {
		t.Errorf("re-stamp roles = %v, want [ground]", got)
	}
}

// TestStampNetRolesHonorsActiveVocab: a --conventions role override installed via SetActiveRoleVocab
// takes effect at stamp time, so a project's house rail name is stamped as a rail.
func TestStampNetRolesHonorsActiveVocab(t *testing.T) {
	defer SetActiveRoleVocab(nil)
	v, err := BuildRoleVocab(RoleVocabConfig{Rail: VocabPatterns{Patterns: []string{`^HV_`}}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	SetActiveRoleVocab(v)
	d := &ir.Design{Nets: []*ir.Net{{Name: "HV_BUS"}}}
	StampNetRoles(d)
	if got := RoleTokens(d.Nets[0]); !reflect.DeepEqual(got, []string{NetRoleRail}) {
		t.Errorf("HV_BUS roles = %v, want [rail] under the extended vocab", got)
	}
}

// TestStampNetRolesDeclared covers WS1-051: a role the SOURCE declared (translated to this
// vocabulary by the reader that understood the format) is unioned with what the naming lexicon
// infers, never replaced by it and never duplicating it. The declared role leads, because it is
// evidence rather than inference.
func TestStampNetRolesDeclared(t *testing.T) {
	declared := func(name, role string) *ir.Net {
		return &ir.Net{Name: name, Attributes: map[string]string{AttrDeclaredRole: role}}
	}
	d := &ir.Design{Nets: []*ir.Net{
		declared("N$17", NetRoleGround),       // opaque name: only the source knows
		declared("GND", NetRoleGround),        // agrees with the name; must not double up
		declared("VCC1V2_FB", NetRoleRail),    // agrees on rail, name adds feedback
		declared("MYSTERY_FB", NetRoleGround), // disagreeing sources UNION, neither wins
		{Name: "SDA"},                         // no declaration, no name match
	}}
	StampNetRoles(d)
	want := map[string][]string{
		"N$17":       {NetRoleGround},
		"GND":        {NetRoleGround},
		"VCC1V2_FB":  {NetRoleRail, NetRoleFeedback},
		"MYSTERY_FB": {NetRoleGround, NetRoleFeedback},
		"SDA":        nil,
	}
	for _, n := range d.Nets {
		if got := RoleTokens(n); !reflect.DeepEqual(got, want[n.Name]) {
			t.Errorf("roles(%q) = %v, want %v", n.Name, got, want[n.Name])
		}
	}
}

// sourceOf returns the evidence recorded for a role on a net, for the assertions below.
func sourceOf(n *ir.Net, role string) ir.RoleSource {
	for _, r := range n.GetRoles() {
		if r.GetRole() == role {
			return r.GetSource()
		}
	}
	return ir.RoleSource_ROLE_SOURCE_UNSPECIFIED
}

// The point of carrying provenance: a role read off the NAME and a role the SOURCE FILE stated are
// no longer indistinguishable. Both were already unioned into the same set (WS1-051); before this
// the consumer had no way to tell which one spoke.
func TestStampNetRolesRecordsItsEvidence(t *testing.T) {
	declared := &ir.Net{Name: "N$17", Attributes: map[string]string{AttrDeclaredRole: NetRoleGround}}
	named := &ir.Net{Name: "+3V3"}
	d := &ir.Design{Nets: []*ir.Net{declared, named}}
	StampNetRoles(d)

	if got := sourceOf(declared, NetRoleGround); got != ir.RoleSource_ROLE_SOURCE_DECLARED {
		t.Errorf("a role the source file stated is DECLARED, got %v", got)
	}
	if got := sourceOf(named, NetRoleRail); got != ir.RoleSource_ROLE_SOURCE_CONVENTION {
		t.Errorf("a role read from the net name is CONVENTION, got %v", got)
	}
}

// A role both channels establish keeps the STRONGER evidence. Recording the weaker of two true
// sources would understate what is known, and it is the one way this dedup can lose information.
func TestStampNetRolesKeepsTheStrongerEvidence(t *testing.T) {
	// Named GND (the convention matches) AND declared ground by the source format.
	both := &ir.Net{Name: "GND", Attributes: map[string]string{AttrDeclaredRole: NetRoleGround}}
	StampNetRoles(&ir.Design{Nets: []*ir.Net{both}})

	if n := len(both.GetRoles()); n != 1 {
		t.Fatalf("one role established twice is still one role, got %d: %v", n, both.GetRoles())
	}
	if got := sourceOf(both, NetRoleGround); got != ir.RoleSource_ROLE_SOURCE_DECLARED {
		t.Errorf("declared beats convention for the same role, got %v", got)
	}
}

// Adding provenance must not change WHICH roles a net carries. This is the behaviour-preservation
// half: the token set is exactly what it was before the field grew a source.
func TestStampNetRolesTokensUnchanged(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{{Name: "VCC1V2_FB"}}}
	StampNetRoles(d)
	if got := RoleTokens(d.Nets[0]); !reflect.DeepEqual(got, []string{NetRoleRail, NetRoleFeedback}) {
		t.Errorf("roles = %v, want [rail feedback] exactly as before", got)
	}
}

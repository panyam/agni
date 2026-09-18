package classify

import (
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
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
	want := map[string][]ir.Role{
		"+3V3":      {ir.Role_ROLE_RAIL},
		"GND":       {ir.Role_ROLE_GROUND},
		"AMP_FB":    {ir.Role_ROLE_FEEDBACK}, // no rail prefix, only the _FB feedback suffix
		"SDA":       nil,
		"VCC1V2_FB": {ir.Role_ROLE_RAIL, ir.Role_ROLE_FEEDBACK}, // a rail-NAMED feedback node carries both
	}
	for _, n := range d.Nets {
		if got := NetRoles(n); !reflect.DeepEqual(got, want[n.Name]) {
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
	if got := NetRoles(d.Nets[0]); !reflect.DeepEqual(got, []ir.Role{ir.Role_ROLE_GROUND}) {
		t.Errorf("re-stamp roles = %v, want [ground]", got)
	}
}

// TestStampNetRolesHonorsActiveVocab: a --conventions role override installed via SetActiveRoleVocab
// takes effect at stamp time, so a project's house rail name is stamped as a rail.
func TestStampNetRolesHonorsActiveVocab(t *testing.T) {
	defer SetActiveRoleVocab(nil)
	v, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{Rail: &configpb.VocabPatterns{Patterns: []string{`^HV_`}}}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	SetActiveRoleVocab(v)
	d := &ir.Design{Nets: []*ir.Net{{Name: "HV_BUS"}}}
	StampNetRoles(d)
	if got := NetRoles(d.Nets[0]); !reflect.DeepEqual(got, []ir.Role{ir.Role_ROLE_RAIL}) {
		t.Errorf("HV_BUS roles = %v, want [rail] under the extended vocab", got)
	}
}

// TestStampNetRolesDeclared covers WS1-051: a role the SOURCE declared (translated to this
// vocabulary by the reader that understood the format) is unioned with what the naming lexicon
// infers, never replaced by it and never duplicating it. The declared role leads, because it is
// evidence rather than inference.
func TestStampNetRolesDeclared(t *testing.T) {
	declared := func(name string, role ir.Role) *ir.Net {
		return &ir.Net{Name: name, Attributes: map[string]string{AttrDeclaredRole: RoleToken(role)}}
	}
	d := &ir.Design{Nets: []*ir.Net{
		declared("N$17", ir.Role_ROLE_GROUND),       // opaque name: only the source knows
		declared("GND", ir.Role_ROLE_GROUND),        // agrees with the name; must not double up
		declared("VCC1V2_FB", ir.Role_ROLE_RAIL),    // agrees on rail, name adds feedback
		declared("MYSTERY_FB", ir.Role_ROLE_GROUND), // disagreeing sources UNION, neither wins
		{Name: "SDA"}, // no declaration, no name match
	}}
	StampNetRoles(d)
	want := map[string][]ir.Role{
		"N$17":       {ir.Role_ROLE_GROUND},
		"GND":        {ir.Role_ROLE_GROUND},
		"VCC1V2_FB":  {ir.Role_ROLE_RAIL, ir.Role_ROLE_FEEDBACK},
		"MYSTERY_FB": {ir.Role_ROLE_GROUND, ir.Role_ROLE_FEEDBACK},
		"SDA":        nil,
	}
	for _, n := range d.Nets {
		if got := NetRoles(n); !reflect.DeepEqual(got, want[n.Name]) {
			t.Errorf("roles(%q) = %v, want %v", n.Name, got, want[n.Name])
		}
	}
}

// sourceOf returns the evidence recorded for a role on a net, for the assertions below.
func sourceOf(n *ir.Net, role ir.Role) ir.RoleSource {
	for _, r := range n.GetRoles() {
		if r.GetRoleKind() == role {
			return r.GetSource()
		}
	}
	return ir.RoleSource_ROLE_SOURCE_UNSPECIFIED
}

// The point of carrying provenance: a role read off the NAME and a role the SOURCE FILE stated are
// no longer indistinguishable. Both were already unioned into the same set (WS1-051); before this
// the consumer had no way to tell which one spoke.
func TestStampNetRolesRecordsItsEvidence(t *testing.T) {
	declared := &ir.Net{Name: "N$17", Attributes: map[string]string{AttrDeclaredRole: RoleToken(ir.Role_ROLE_GROUND)}}
	named := &ir.Net{Name: "+3V3"}
	d := &ir.Design{Nets: []*ir.Net{declared, named}}
	StampNetRoles(d)

	if got := sourceOf(declared, ir.Role_ROLE_GROUND); got != ir.RoleSource_ROLE_SOURCE_DECLARED {
		t.Errorf("a role the source file stated is DECLARED, got %v", got)
	}
	if got := sourceOf(named, ir.Role_ROLE_RAIL); got != ir.RoleSource_ROLE_SOURCE_CONVENTION {
		t.Errorf("a role read from the net name is CONVENTION, got %v", got)
	}
}

// A role both channels establish keeps the STRONGER evidence. Recording the weaker of two true
// sources would understate what is known, and it is the one way this dedup can lose information.
func TestStampNetRolesKeepsTheStrongerEvidence(t *testing.T) {
	// Named GND (the convention matches) AND declared ground by the source format.
	both := &ir.Net{Name: "GND", Attributes: map[string]string{AttrDeclaredRole: RoleToken(ir.Role_ROLE_GROUND)}}
	StampNetRoles(&ir.Design{Nets: []*ir.Net{both}})

	if n := len(both.GetRoles()); n != 1 {
		t.Fatalf("one role established twice is still one role, got %d: %v", n, both.GetRoles())
	}
	if got := sourceOf(both, ir.Role_ROLE_GROUND); got != ir.RoleSource_ROLE_SOURCE_DECLARED {
		t.Errorf("declared beats convention for the same role, got %v", got)
	}
}

// Adding provenance must not change WHICH roles a net carries. This is the behaviour-preservation
// half: the token set is exactly what it was before the field grew a source.
func TestStampNetRolesTokensUnchanged(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{{Name: "VCC1V2_FB"}}}
	StampNetRoles(d)
	if got := NetRoles(d.Nets[0]); !reflect.DeepEqual(got, []ir.Role{ir.Role_ROLE_RAIL, ir.Role_ROLE_FEEDBACK}) {
		t.Errorf("roles = %v, want [rail feedback] exactly as before", got)
	}
}

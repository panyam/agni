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
		if got := n.Roles; !reflect.DeepEqual(got, want[n.Name]) {
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
	if got := d.Nets[0].Roles; !reflect.DeepEqual(got, []string{NetRoleGround}) {
		t.Errorf("re-stamp roles = %v, want [ground]", got)
	}
}

// TestStampNetRolesHonorsActiveVocab: a --conventions role override installed via SetActiveRoleVocab
// takes effect at stamp time, so a project's house rail name is stamped as a rail.
func TestStampNetRolesHonorsActiveVocab(t *testing.T) {
	defer SetActiveRoleVocab(nil)
	v, err := BuildRoleVocab(VocabPatterns{Patterns: []string{`^HV_`}}, VocabPatterns{}, VocabPatterns{}, VocabPatterns{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	SetActiveRoleVocab(v)
	d := &ir.Design{Nets: []*ir.Net{{Name: "HV_BUS"}}}
	StampNetRoles(d)
	if got := d.Nets[0].Roles; !reflect.DeepEqual(got, []string{NetRoleRail}) {
		t.Errorf("HV_BUS roles = %v, want [rail] under the extended vocab", got)
	}
}

package classify

import (
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"reflect"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestSwitchingRoleVocabulary: the switching vocabulary matches a regulator's power-stage nodes and
// nothing else. The negative half carries the weight here, because a vocabulary that matched every
// net would satisfy the positive half alone and read as working.
func TestSwitchingRoleVocabulary(t *testing.T) {
	v := DefaultRoleVocab()
	for _, name := range []string{"12V_SW", "3V3_PHASE", "5V_BOOT", "VCC_LX", "/DCDC/12V_SW"} {
		if !v.IsSwitching(name) {
			t.Errorf("IsSwitching(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"+3V3", "GND", "SDA", "12V_FB", "SW_EN", "BOOTSEL", "12V_SWITCHED"} {
		if v.IsSwitching(name) {
			t.Errorf("IsSwitching(%q) = true, want false", name)
		}
	}
}

// TestStampNetRolesStampsSwitching: a switch node is stamped with BOTH roles, the same way a
// rail-named feedback node is. The stamp still records every match; what changed with agni 680 is
// that rail-versus-regulator-internal precedence is settled in Model.IsRailNet rather than by each
// consumer, and that is asserted in core/check.
func TestStampNetRolesStampsSwitching(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "12V_SW"}, {Name: "AMP_PHASE"}, {Name: "+5V"},
	}}
	StampNetRoles(d)
	want := map[string][]string{
		"12V_SW":    {NetRoleRail, NetRoleSwitching}, // a rail-NAMED switch node carries both
		"AMP_PHASE": {NetRoleSwitching},              // no rail prefix, only the _PHASE suffix
		"+5V":       {NetRoleRail},
	}
	for _, n := range d.Nets {
		if got := RoleTokens(n); !reflect.DeepEqual(got, want[n.Name]) {
			t.Errorf("roles(%q) = %v, want %v", n.Name, got, want[n.Name])
		}
	}
}

// TestSwitchingVocabularyIsProjectExtensible: the role is lexicon config like every other, so a house
// convention the built-in patterns miss is reachable without patching the engine. This is the
// property agni 677 says a component CLASS does not have.
func TestSwitchingVocabularyIsProjectExtensible(t *testing.T) {
	v, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{Switching: &configpb.VocabPatterns{Patterns: []string{`_HSD$`}}}})
	if err != nil {
		t.Fatalf("BuildRoleVocab: %v", err)
	}
	if !v.IsSwitching("12V_HSD") {
		t.Error("an added pattern must match")
	}
	if !v.IsSwitching("12V_SW") {
		t.Error("an added pattern must not displace the defaults")
	}
}

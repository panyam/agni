package relations

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func rolesDesign() *ir.Design {
	return &ir.Design{Nets: []*ir.Net{
		{Name: "12V_OUT", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "12V_FB", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "12V_SW", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "12V_MODE1", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "12V_VDRV", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "GND", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "SDA", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "SCL", Attributes: map[string]string{"external": "true"}, Prov: &ir.Provenance{SourceFile: "t"}},
	}}
}

// TestNetRoleProjectsEveryStampedRole: the point of the relation. A role the engine acts on must be
// nameable in a query, and before agni 691 control and gate_drive were stamped and projected nowhere.
func TestNetRoleProjectsEveryStampedRole(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(rolesDesign())))

	got := map[string]map[string]bool{}
	for _, f := range byRel[RelNetRole] {
		if got[f.Subject] == nil {
			got[f.Subject] = map[string]bool{}
		}
		got[f.Subject][f.Value] = true
	}

	for _, c := range []struct{ net, role string }{
		{"12V_OUT", check.NetRoleRail},
		{"12V_FB", check.NetRoleFeedback},
		{"12V_SW", check.NetRoleSwitching},
		{"12V_MODE1", check.NetRoleControl},
		{"12V_VDRV", check.NetRoleGateDrive},
		{"GND", check.NetRoleGround},
	} {
		if !got[c.net][c.role] {
			t.Errorf("net.role(%s, %s) missing; got %v", c.net, c.role, got[c.net])
		}
	}
	// A net can carry several: 12V_SW is rail by prefix and switching by suffix.
	if !got["12V_SW"][check.NetRoleRail] {
		t.Errorf("net.role must emit EVERY role, not the first: 12V_SW = %v", got["12V_SW"])
	}
	// The negative half. A plain signal carries none.
	if len(got["SDA"]) != 0 {
		t.Errorf("net.role(SDA) = %v, want no rows", got["SDA"])
	}
}

// TestEveryRoleInTheVocabularyIsProjectable: the ratchet. classify.AllNetRoles is what the projector
// iterates, so a role added to the token list and forgotten here is invisible to every query, which is
// exactly how control and gate_drive spent a release unqueryable.
func TestEveryRoleInTheVocabularyIsProjectable(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(rolesDesign())))
	seen := map[string]bool{}
	for _, f := range byRel[RelNetRole] {
		seen[f.Value] = true
	}
	for _, role := range classify.AllNetRoles() {
		if !seen[role] {
			t.Errorf("role %q is in AllNetRoles but no fixture net projects it; either the fixture or the projector is short", role)
		}
	}
}

// TestNetAttrProjectsDeclaredAttributes: the twin of component.attr, and the half nets never had.
func TestNetAttrProjectsDeclaredAttributes(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(rolesDesign())))

	var found bool
	for _, f := range byRel[RelNetAttr] {
		if f.Subject == "SCL" && f.Object == "external" && f.Value == "true" {
			found = true
		}
	}
	if !found {
		t.Errorf("net.attr(SCL, external, true) missing: %+v", byRel[RelNetAttr])
	}
	// Declared, not derived: a role must not leak into the attribute relation.
	for _, f := range byRel[RelNetAttr] {
		if f.Object == "role" {
			t.Errorf("net.attr carries a role (%+v); roles are net.role, attributes are what the file declared", f)
		}
	}
}

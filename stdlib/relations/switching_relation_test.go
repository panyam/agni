package relations

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func switchingRelationDesign() *ir.Design {
	return &ir.Design{Nets: []*ir.Net{
		{Name: "12V_OUT", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "12V_SW", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "12V_BOOT", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "12V_PHASE", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "VCC_LX", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "12V_FB", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "SDA", Prov: &ir.Provenance{SourceFile: "t"}},
	}}
}

// TestSwitchingRelationProjectsThePowerStageNodes: the relation names exactly the nets carrying the
// switching role, so "a rail that is not a power-stage node" becomes writable in datalog. Until this
// existed only the feedback half of that pair could be said.
func TestSwitchingRelationProjectsThePowerStageNodes(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(switchingRelationDesign())))

	got := map[string]bool{}
	for _, f := range byRel[RelSwitching] {
		got[f.Subject] = true
	}
	for _, want := range []string{"12V_SW", "12V_BOOT", "12V_PHASE", "VCC_LX"} {
		if !got[want] {
			t.Errorf("switching(%s) missing: %+v", want, byRel[RelSwitching])
		}
	}
	// The negative half. A relation matching every net would satisfy the positive half alone.
	for _, never := range []string{"12V_OUT", "12V_FB", "SDA"} {
		if got[never] {
			t.Errorf("switching(%s) present, want absent", never)
		}
	}
}

// TestRailExcludesBothRegulatorInternalRoles: `rail` and the two "not a rail" relations cannot
// overlap. This is the cross-relation assertion agni 684 was filed on the belief that it would FAIL,
// because railFacts asks Model.IsPowerRail (a name function) while net.nominal_voltage asks
// IsRailNet. It passes, because IsPowerRail delegates to IsRailNet for the name path and so inherits
// the agni 683 exclusion. Kept as a ratchet: the two answers agreed by delegation rather than by
// anything asserting it, and a future change to either could part them silently.
func TestRailExcludesBothRegulatorInternalRoles(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(switchingRelationDesign())))

	rails := map[string]bool{}
	for _, f := range byRel[RelRail] {
		rails[f.Subject] = true
	}
	for _, rel := range []string{RelSwitching, RelFeedback} {
		for _, f := range byRel[rel] {
			if rails[f.Subject] {
				t.Errorf("%s is in both rail and %s; a rail-named net that is not a rail must be in exactly one", f.Subject, rel)
			}
		}
	}
	// The positive control: rail is not simply empty.
	if !rails["12V_OUT"] {
		t.Errorf("rail(12V_OUT) missing, so the assertion above proves nothing: %+v", byRel[RelRail])
	}
}

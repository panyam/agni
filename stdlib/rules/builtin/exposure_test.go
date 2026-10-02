package builtin

import (
	"context"
	"reflect"
	"testing"

	"github.com/panyam/agni/core/check"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// exposureDesign has two plain connectors. J1 is the cable entry and J2 the one a design declares
// internal, and each exposure rule has a net that reaches only J2, plus one net reaching both.
func exposureDesign() *ir.Design {
	load := &ir.PartType{Name: "LOAD", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
	}}
	comps := []*ir.Component{
		{RefDes: "J1", Attributes: map[string]string{"Value": "Mezzanine 40-pin"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "J2", Attributes: map[string]string{"Value": "Mezzanine 80-pin"}, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "U2", Sections: []*ir.ComponentSection{{PartRef: "LOAD", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		{RefDes: "D1", Attributes: map[string]string{"Description": "DIODE ZENER 18V 1W"}, Prov: &ir.Provenance{SourceFile: "t"}},
	}
	return protDesign(comps, []*ir.PartType{load}, []*ir.Net{
		tnet("EXT_SIG", "J1.1", "U1.1"),
		tnet("INT_SIG", "J2.1", "U1.2"),
		tnet("INT_ZENER", "J2.2", "U1.3", "D1.1"),
		tnet("BOTH", "J2.3", "J1.2", "U1.4"),
		tnet("INT_VIN", "J2.4", "U2.1"),
	})
}

func internalIntent(refs ...string) *configpb.DesignIntent {
	di := &configpb.DesignIntent{Components: map[string]*configpb.ComponentIntent{}}
	for _, r := range refs {
		di.Components[r] = &configpb.ComponentIntent{Exposure: "internal"}
	}
	return di
}

func exposureFindings(m check.Model) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, f := range check.RunBackground(m, rules) {
		if out[f.Rule] == nil {
			out[f.Rule] = map[string]bool{}
		}
		out[f.Rule][check.EntityRef(f.Subject)] = true
	}
	return out
}

// Whether a connector faces the field comes from the design's intent (agni issue 831). The same two
// mezzanine connectors are both exposed until the design declares one internal, and then the four
// exposure rules leave the nets reaching only that one alone. A net that also reaches the cable entry
// stays exposed.
func TestExposureFollowsDeclaredIntent(t *testing.T) {
	cases := []struct {
		rule string
		net  string
	}{
		{"esd-protection", "INT_SIG"},
		{"esd-clamp-not-tvs", "INT_ZENER"},
		{"input-protection", "INT_VIN"},
		{"reverse-blocking-absent", "INT_VIN"},
	}

	undeclared := exposureFindings(check.NewModel(exposureDesign()))
	for _, c := range cases {
		if !undeclared[c.rule][c.net] {
			t.Errorf("with no declaration, %s should flag %s, since an undeclared connector is exposed; got %v", c.rule, c.net, undeclared[c.rule])
		}
	}

	declared := exposureFindings(check.NewModel(exposureDesign(), check.WithIntent(internalIntent("J2"))))
	for _, c := range cases {
		if declared[c.rule][c.net] {
			t.Errorf("with J2 declared internal, %s still flags %s", c.rule, c.net)
		}
	}
	for _, net := range []string{"EXT_SIG", "BOTH"} {
		if !declared["esd-protection"][net] {
			t.Errorf("%s reaches the undeclared J1, so esd-protection should still flag it; got %v", net, declared["esd-protection"])
		}
	}
}

// The declarative twins of the exposure rules read the same declaration as their Go rules, so the
// two agree on a model built with intent as well as without.
func TestExposureTwinsAgreeUnderIntent(t *testing.T) {
	byName := rulesByName()
	m := check.NewModel(exposureDesign(), check.WithIntent(internalIntent("J2")))
	compared := 0
	for _, name := range []string{"esd-protection", "esd-clamp-not-tvs", "input-protection"} {
		spec := specs[name]
		if spec == nil {
			t.Errorf("%s has no declarative twin", name)
			continue
		}
		got, want := spec.Eval(context.Background(), m), byName[name].Findings(context.Background(), m)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: spec findings diverge under intent\n spec: %+v\n   go: %+v", name, got, want)
		}
		compared++
	}
	if compared != 3 {
		t.Fatalf("compared %d twins, want 3", compared)
	}
}

package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A declaration that names a connector the design has passes, and one naming a part the design lacks
// or a part that is not a connector fails, since either would change nothing while reading as a
// declaration that took effect (agni issue 831).
func TestExposureDeclared(t *testing.T) {
	decl := declOf(t, `
name: carrier
intent:
  components:
    J1:  {exposure: internal}
    J9:  {exposure: internal}
    R1:  {exposure: internal}
    J2:  {exposure: external}
`)
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "J1", DeviceClasses: classify.Tags("connector")},
		{RefDes: "J2", DeviceClasses: classify.Tags("connector")},
		{RefDes: "R1", DeviceClasses: classify.Tags("resistor")},
	}}
	got := map[string]check.Outcome{}
	for _, v := range check.RunVerdictsBackground(check.NewModel(d), Compile(decl)) {
		if v.Rule == RuleExposureDeclared {
			got[check.EntityRef(v.Subjects[0])] = v.Outcome
		}
	}
	want := map[string]check.Outcome{"J1": check.Pass, "J2": check.Pass, "J9": check.Fail, "R1": check.Fail}
	if len(got) != len(want) {
		t.Fatalf("want a verdict per declared component, got %v", got)
	}
	for ref, o := range want {
		if got[ref] != o {
			t.Errorf("%s: outcome %v, want %v", ref, got[ref], o)
		}
	}
}

func TestExposureValidation(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"J1: {exposure: sideways}", "must be internal or external"},
		{"J1: {}", "declares nothing"},
	} {
		_, err := Parse([]byte("name: x\nintent:\n  components:\n    " + tc.body + "\n"))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error containing %q", tc.body, err, tc.want)
		}
	}
	// components alone is a declaration, not an empty one.
	if _, err := Parse([]byte("name: x\nintent:\n  components:\n    J1: {exposure: internal}\n")); err != nil {
		t.Errorf("a declaration carrying only components should load: %v", err)
	}
}

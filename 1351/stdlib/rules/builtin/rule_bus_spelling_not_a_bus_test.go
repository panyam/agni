package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func rangeLabelDesign(supplied bool, labels ...*ir.RangeLabel) *ir.Design {
	d := &ir.Design{InputDiagnostics: &ir.InputDiagnostics{RangeLabels: labels}}
	if supplied {
		d.InputDiagnostics.Supplied = []string{"range_labels"}
	}
	return d
}

// A label in the format's bus syntax passes and a colon-spelled one fails, so the rule partitions the
// labels the reader saw. One name on two sheets is one verdict.
func TestBusSpellingNotABusPartitionsTheLabels(t *testing.T) {
	m := check.NewModel(rangeLabelDesign(true,
		&ir.RangeLabel{Label: "ADDR[0..3]", ReadsAsBus: true},
		&ir.RangeLabel{Label: "DATA[7:0]", Prov: &ir.Provenance{SourceFile: "a.kicad_sch"}},
		&ir.RangeLabel{Label: "DATA[7:0]", Prov: &ir.Provenance{SourceFile: "b.kicad_sch"}},
	))
	vs := busSpellingNotABus.Eval(context.Background(), m)
	if len(vs) != 2 {
		t.Fatalf("verdicts = %d, want 2 (one per name)", len(vs))
	}
	got := map[string]check.Outcome{}
	for _, v := range vs {
		got[check.EntityRef(v.Subjects[0])] = v.Outcome
	}
	if got["ADDR[0..3]"] != check.Pass || got["DATA[7:0]"] != check.Fail {
		t.Errorf("outcomes = %v, want ADDR[0..3] pass and DATA[7:0] fail", got)
	}
	fs := busSpellingNotABus.Findings(context.Background(), m)
	if len(fs) != 1 || check.EntityRef(fs[0].Subject) != "DATA[7:0]" || fs[0].Prov.GetSourceFile() != "a.kicad_sch" {
		t.Errorf("findings = %v, want one on DATA[7:0] at a.kicad_sch", fs)
	}
}

// A reader that never recorded range labels has checked nothing, so the rule must not run there and
// read as a pass.
func TestBusSpellingNotABusNeedsTheReaderToHaveLooked(t *testing.T) {
	if ok, reason := check.Available(busSpellingNotABus, check.NewModel(rangeLabelDesign(false))); ok || reason == "" {
		t.Errorf("a read that never declared range_labels: available=%v reason=%q, want not-applicable with a reason", ok, reason)
	}
	if ok, _ := check.Available(busSpellingNotABus, check.NewModel(rangeLabelDesign(true))); !ok {
		t.Error("a read that declared range_labels: want available")
	}
}

package check

import (
	"context"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A verdict about an unfitted part, named directly or through a pin, is not considered, and one about
// a fitted part keeps its outcome (agni issue 1082).
func TestOnFittedPartsJudgesNoUnfittedPart(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1"}, {RefDes: "U2", Attributes: map[string]string{"dnp": "yes"}},
	}}
	f := Finding{Message: "over its rating"}
	r := OnFittedParts(&Rule{Name: "t", Eval: func(context.Context, Model) []Verdict {
		return []Verdict{
			{Subjects: []Entity{ComponentEntity("U1")}, Outcome: Fail, Finding: &f},
			{Subjects: []Entity{ComponentEntity("U2")}, Outcome: Fail, Finding: &f},
			{Subjects: []Entity{PinEntity("U2", "3")}, Outcome: Pass},
		}
	}})
	vs := r.Eval(context.Background(), NewModel(d))
	if vs[0].Outcome != Fail || vs[0].Finding == nil {
		t.Errorf("fitted U1 = %v, want its fail kept", vs[0].Outcome)
	}
	for i, v := range vs[1:] {
		if v.Outcome != NotConsidered || v.Finding != nil || v.Reason == "" {
			t.Errorf("verdict %d about unfitted U2 = %v %q, want not-considered with a reason", i+1, v.Outcome, v.Reason)
		}
	}
	if OnFittedParts(r); !r.JudgesFittedParts() {
		t.Error("a marked rule does not report it")
	}
}

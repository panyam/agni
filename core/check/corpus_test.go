package check

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Every datasheet citation names the corpus its spec came from, on findings and verdicts alike, so a
// verdict resting on a limit transcribed outside the project says so (agni issue 749). The text
// rendering marks only the shared case.
func TestCitationsNameTheirCorpus(t *testing.T) {
	spec := func(mpn string) *parampb.PartSpec {
		return &parampb.PartSpec{Mpn: mpn, Docs: []*parampb.SourceDoc{{Id: "ds", Title: mpn + " Rev A"}},
			Parameters: []*parampb.Parameter{{Symbol: "VIN", Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 3}}}}
	}
	project, shared := param.ParamSet{"LDO": spec("LDO")}, param.ParamSet{"BUCK": spec("BUCK"), "LDO": spec("LDO")}
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U1", Mpn: "BUCK"}, {RefDes: "U2", Mpn: "LDO"}}}
	m := NewModel(d, WithParamProvider(param.Layered{{Name: param.CorpusProject, Provider: project}, {Name: param.CorpusShared, Provider: shared}}))

	rule := &Rule{
		Name:                "cites",
		StatesConsideredSet: true,
		Eval: func(_ context.Context, m Model) []Verdict {
			var out []Verdict
			for _, ref := range []string{"U1", "U2"} {
				s := m.PartSpec(ref)
				out = append(out, Verdict{
					Subjects: []Entity{ComponentEntity(ref)},
					Outcome:  Fail,
					Witness:  &Witness{Statement: ref + " over its limit", Datasheet: []*DatasheetCitation{DatasheetCitationOf(s, s.GetParameters()[0])}},
					Finding:  &Finding{Subject: ComponentEntity(ref), Message: "over", DatasheetProv: []*DatasheetCitation{DatasheetCitationOf(s, s.GetParameters()[0])}},
				})
			}
			return out
		},
	}
	want := map[string]string{"U1": param.CorpusShared, "U2": param.CorpusProject}

	vs := RunVerdictsBackground(m, []*Rule{rule})
	if len(vs) != 2 {
		t.Fatalf("got %d verdicts, want 2", len(vs))
	}
	for _, v := range vs {
		ref := v.Subjects[0].Ref
		if got := v.Witness.Datasheet[0].Corpus; got != want[ref] {
			t.Errorf("%s verdict citation corpus = %q, want %q", ref, got, want[ref])
		}
		marked := strings.Contains(v.Witness.Render(), "(shared corpus)")
		if marked != (want[ref] == param.CorpusShared) {
			t.Errorf("%s witness renders %q; only a shared citation is marked", ref, v.Witness.Render())
		}
	}
	fs := RunBackground(m, []*Rule{rule})
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2", len(fs))
	}
	for _, f := range fs {
		if got := f.DatasheetProv[0].Corpus; got != want[f.Subject.Ref] {
			t.Errorf("%s finding citation corpus = %q, want %q", f.Subject.Ref, got, want[f.Subject.Ref])
		}
	}
}

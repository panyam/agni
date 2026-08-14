package param

import (
	"errors"
	"strings"
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func specWithDocTitle(mpn, title string) *parampb.PartSpec {
	return &parampb.PartSpec{Mpn: mpn, Docs: []*parampb.SourceDoc{{Id: "ds", Title: title}}}
}

// A title that repeats the MPN is not a missing answer, it is a wrong one: it reads like a citation,
// and it is the same string before and after a reissue, so nobody re-checks it (agni issue 290).
func TestDocTitleRepeatingThePartIsFlagged(t *testing.T) {
	for _, title := range []string{"LM1117", "lm1117", "  LM1117  "} {
		spec := specWithDocTitle("LM1117", title)
		got := errors.Join(completenessProblems(spec)...)
		if got == nil || !strings.Contains(got.Error(), "the part, not the document") {
			t.Errorf("title %q: completeness = %v, want it flagged", title, got)
		}
		if err := errors.Join(structuralProblems(spec)...); err != nil {
			t.Errorf("title %q: must not be structural, a draft is allowed to be here: %v", title, err)
		}
	}
}

// Absent is the honest state a first-pass derivation is in, and derive treats any Validate failure
// as its own bug, so reporting it here would make every derived spec fail. The refusal is carried by
// the run manifest's gap list instead, and the citation says "revision unrecorded" at the point of use.
func TestAbsentDocTitleIsNotAProblem(t *testing.T) {
	spec := specWithDocTitle("LM1117", "")
	if got := errors.Join(completenessProblems(spec)...); got != nil && strings.Contains(got.Error(), "the part, not the document") {
		t.Errorf("an absent title must not be reported as a wrong one: %v", got)
	}
	if err := Validate(spec); err != nil && strings.Contains(err.Error(), "title") {
		t.Errorf("Validate must accept a spec that has not stated its document identity yet: %v", err)
	}
}

func TestRealDocTitlePasses(t *testing.T) {
	spec := specWithDocTitle("LM1117", "SNOS412Q - REVISED JANUARY 2023")
	if got := errors.Join(completenessProblems(spec)...); got != nil && strings.Contains(got.Error(), "the part, not the document") {
		t.Errorf("a real document identity must not be flagged: %v", got)
	}
}

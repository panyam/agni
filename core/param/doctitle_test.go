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

// A title can repeat the MPN. That is a wrong answer rather than a missing one. It reads like a
// citation and does not change when the document is reissued, so nobody re-checks it (agni issue 290).
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

// A first-pass derivation cannot state the title yet, so absent is its accurate state. derive treats
// any Validate failure as its own bug, so reporting an absent title here would make every derived spec
// fail. The run manifest's gap list records the refusal instead, and the citation says "revision
// unrecorded" at the point of use.
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

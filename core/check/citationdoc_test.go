package check

import (
	"strings"
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func specTitled(title string) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn:  "LM1117",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: title}},
		Parameters: []*parampb.Parameter{{
			Symbol: "VIN",
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 4, TableOrFigure: "Absolute Maximum Ratings",
				Method: "hand", Confidence: 1,
			},
		}},
	}
}

// The citation is the instruction to go and check a value, and page numbers move between revisions.
// One that cannot name a revision has to say so rather than reading like a complete answer.
func TestCitationSaysWhenTheRevisionIsUnrecorded(t *testing.T) {
	spec := specTitled("")
	got := Citation(spec, spec.Parameters[0])

	if !strings.Contains(got, "LM1117") {
		t.Errorf("a reader still needs to know which part: %q", got)
	}
	if !strings.Contains(got, "revision unrecorded") {
		t.Errorf("citation must state that it cannot name the revision: %q", got)
	}
	// The old behaviour borrowed the part name and presented it as the document's own.
	if strings.Contains(got, `datasheet "LM1117"`) {
		t.Errorf("part name must not be presented as the document's identity: %q", got)
	}
}

func TestCitationPrefersTheRealDocumentIdentity(t *testing.T) {
	spec := specTitled("SNOS412Q - REVISED JANUARY 2023")
	got := Citation(spec, spec.Parameters[0])

	if !strings.Contains(got, `datasheet "SNOS412Q - REVISED JANUARY 2023"`) {
		t.Errorf("a recorded identity must be cited as-is: %q", got)
	}
	if strings.Contains(got, "unrecorded") {
		t.Errorf("nothing is missing here: %q", got)
	}
}

// With neither an identity nor a part there is nothing to name, and the citation must not go blank.
func TestCitationWithNothingToNameStaysExplicit(t *testing.T) {
	spec := specTitled("")
	spec.Mpn = ""
	if got := Citation(spec, spec.Parameters[0]); !strings.Contains(got, "unknown source") {
		t.Errorf("citation must never be silently blank: %q", got)
	}
}

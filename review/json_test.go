package review

import (
	"strings"
	"testing"

	"github.com/panyam/agni/check"
)

// RenderJSON carries a finding's structured datasheet citation as its own `datasheet` object
// (doc/page/section/method/confidence), so a report renderer shows the source and can flag a low
// confidence without parsing the message. A finding with no datasheet backing omits the key.
func TestRenderJSONDatasheetProv(t *testing.T) {
	rep := Report{
		Manifest: "m", Design: "d",
		Areas: []AreaResult{{
			Area: Area{Name: "power"},
			Items: []ItemResult{{
				Item:    Item{ID: "18", Title: "regulator output ratings"},
				Outcome: Fail,
				Findings: []check.Finding{
					{
						Rule: "review/18", Severity: "warning", Kind: check.KindComponent, Subject: "U7000",
						Message: "IOUT below requirement",
						DatasheetProv: &check.DatasheetCitation{
							Doc: "LMR60410-Q1 (SNAS870B Rev. B)", DocRef: "snas870b", Page: 5,
							Section: "6.3 Recommended Operating Conditions", Method: "hand", Confidence: 1.0,
						},
					},
					{Rule: "review/other", Severity: "warning", Kind: check.KindComponent, Subject: "U9", Message: "no datasheet backing"},
				},
			}},
		}},
	}
	out, err := RenderJSON(rep)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"datasheet":`, `"doc": "LMR60410-Q1 (SNAS870B Rev. B)"`, `"page": 5`,
		`"section": "6.3 Recommended Operating Conditions"`, `"confidence": 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("datasheet-backed finding is missing %q in:\n%s", want, out)
		}
	}
	// The second finding has no citation, so exactly one `datasheet` object appears.
	if n := strings.Count(out, `"datasheet":`); n != 1 {
		t.Fatalf("want exactly 1 datasheet object (only the backed finding), got %d", n)
	}
}

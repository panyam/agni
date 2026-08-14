package derive

import (
	"strings"
	"testing"

	derivepb "github.com/panyam/agni/gen/go/agni/v1/derive"
)

func gapsOfKind(m *derivepb.RunManifest, kind string) []*derivepb.Gap {
	var out []*derivepb.Gap
	for _, g := range m.GetGaps() {
		if g.GetKind() == kind {
			out = append(out, g)
		}
	}
	return out
}

// A refusal that is not recorded is indistinguishable from a success. The gap is what makes leaving
// the field empty affordable rather than just lossy.
func TestUnidentifiedDocumentIsGappedWithItsEvidence(t *testing.T) {
	d := loadDocFixture(t, "bss138-raw-docir.textproto")
	recipes, patches := loadArtifacts(t)

	spec, manifest, err := Run(d, recipes, patches, Identity{MPN: "BSS138", Manufacturer: "onsemi"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.GetDocs()[0].GetTitle() != "" {
		t.Errorf("derive asserted a document identity it cannot read: %q", spec.GetDocs()[0].GetTitle())
	}

	gaps := gapsOfKind(manifest, "unidentified-document")
	if len(gaps) != 1 {
		t.Fatalf("want exactly one unidentified-document gap, got %d", len(gaps))
	}
	detail := gaps[0].GetDetail()
	// The prose is the point: a reader decides from what the cover page says, without reopening it.
	if !strings.Contains(detail, "Opening prose:") {
		t.Errorf("gap carries no evidence to decide from: %q", detail)
	}
	// The doc-IR title is named as what it is, so the next reader does not re-make the assignment.
	if !strings.Contains(detail, "BSS138") {
		t.Errorf("gap should name the doc-IR's own title as the part rather than the document: %q", detail)
	}
}

// The evidence has to be real page-one prose, not a placeholder, or the gap is a flag with extra
// words. This is the positive control on identityEvidence: a document whose first page carries text
// must produce some of it.
func TestIdentityEvidenceCarriesRealPageOneText(t *testing.T) {
	d := loadDocFixture(t, "bss138-raw-docir.textproto")

	var want string
	for _, pg := range d.GetPages() {
		if pg.GetNumber() != 1 {
			continue
		}
		for _, tb := range pg.GetTextBlocks() {
			if strings.TrimSpace(tb.GetText()) != "" {
				want = strings.TrimSpace(tb.GetText())
				break
			}
		}
		break
	}
	if want == "" {
		t.Skip("fixture has no page-one text, so this control proves nothing")
	}
	if got := identityEvidence(d); !strings.Contains(got, want) {
		t.Errorf("evidence %q does not contain the document's first text block %q", got, want)
	}
}

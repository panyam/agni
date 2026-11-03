package derive

import (
	"fmt"
	"strings"

	derivepb "github.com/panyam/agni/datasheet/gen/go/agni/v1/derive"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
)

// A spec's document identity says which revision of which document its values were read from.
// SourceDoc.title is specified to hold it ("SNOS412Q - REVISED JANUARY 2023"), and nothing
// establishes it today (agni issue 290).
//
// This file does not guess it. Measured over real vendor documents, the printed identity came in
// several unrelated shapes, and the obvious "document number" detector accepted TPS22918 and
// TCAN1145, which are parts, and missed SLVSAG5, which is a document. So the run records the refusal
// as a gap. The gap holds the cover-page prose, the way an untyped pin holds its description.
// Answering the gap is a curation act. See
// docsite/content/architecture/datasheet-layer.md#how-a-partspec-is-derived-from-a-document.

// identityEvidenceBlocks is how many leading page-one text blocks the gap includes. That is enough
// for a title block (document number, revision line, date) and short enough to read at a glance.
const identityEvidenceBlocks = 6

// gapUnidentifiedDocument records that a run could not state which revision it derived from, with
// the document's own opening prose as the evidence to decide from.
//
// It is unconditional because nothing establishes an identity yet, so every run carries this gap.
func gapUnidentifiedDocument(d *docpb.Document, manifest *derivepb.RunManifest) {
	manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
		Kind: "unidentified-document",
		Detail: fmt.Sprintf("no document number or revision recorded, so a citation cannot say which "+
			"revision it cites; the doc-IR titles itself %q, which is the part rather than the document. "+
			"Opening prose: %s", d.GetTitle(), identityEvidence(d)),
	})
}

// identityEvidence renders page one's opening text blocks. A printed identity appears there if it
// appears anywhere. Some real documents have a company-transition notice as their cover. The
// evidence then shows that notice, and the notice explains why nothing was found.
func identityEvidence(d *docpb.Document) string {
	var parts []string
	for _, pg := range d.GetPages() {
		if pg.GetNumber() != 1 {
			continue
		}
		for _, tb := range pg.GetTextBlocks() {
			t := strings.TrimSpace(tb.GetText())
			if t == "" {
				continue
			}
			parts = append(parts, t)
			if len(parts) == identityEvidenceBlocks {
				break
			}
		}
		break
	}
	if len(parts) == 0 {
		return "(none: the document has no page-one text)"
	}
	return strings.Join(parts, " | ")
}

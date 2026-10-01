package derive

import (
	"fmt"
	"strings"

	derivepb "github.com/panyam/agni/datasheet/gen/go/agni/v1/derive"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
)

// Document identity: which revision of which document a spec's values were read from.
// SourceDoc.title is specified to carry it ("SNOS412Q - REVISED JANUARY 2023"), and nothing
// establishes it today (agni issue 290).
//
// This file does not guess it. Measured over real vendor documents, the printed identity came in
// several unrelated shapes, and the obvious "document number" detector accepted TPS22918 and
// TCAN1145, which are parts, and missed SLVSAG5, which is a document. So the refusal is recorded as
// a gap carrying the cover-page prose, the way an untyped pin carries its description, and answering
// it is a curation act. See
// docsite/content/architecture/datasheet-layer.md#how-a-partspec-is-derived-from-a-document.

// identityEvidenceBlocks is how many leading page-one text blocks ride along with the gap. Enough
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

// identityEvidence renders page one's opening text blocks, where a printed identity lives when it is
// anywhere. A document whose cover is a company-transition notice (a real shape) shows that notice,
// which explains why nothing was found.
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

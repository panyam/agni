package derive

import (
	"fmt"
	"strings"

	derivepb "github.com/panyam/agni/gen/go/agni/v1/derive"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
)

// Document identity: which revision of which document a spec's values were read from.
//
// SourceDoc.title is specified to carry it ("SNOS412Q - REVISED JANUARY 2023"), because it is the
// citation an engineer opens to check a value. Nothing establishes it today. derive used to copy the
// doc-IR's own title into it, which is a PART number, so every citation named the part and no
// citation named a revision -- identical before and after a reissue (agni issue 290).
//
// This file does not guess it either. A measurement over real vendor documents found the printed
// identity in several unrelated shapes, and found that the obvious detector for a "document number"
// matches part numbers just as happily: it accepted TPS22918 and TCAN1145, which are parts, and
// missed SLVSAG5, which is a document. A rule built on that would keep the bug and add machinery,
// which is the failure the pin-typing path already argued against: a guess that looks like a fact is
// the one thing nobody re-checks.
//
// So the refusal is recorded with its evidence, and answering it is a curation act. The cover-page
// prose travels with the gap so a reader decides from what the document says rather than reopening
// it, exactly as an untyped pin carries its description.

// identityEvidenceBlocks is how many leading page-one text blocks ride along with the gap. Enough
// for a title block (document number, revision line, date) and short enough to read at a glance.
const identityEvidenceBlocks = 6

// gapUnidentifiedDocument records that a run could not state which revision it derived from, with
// the document's own opening prose as the evidence to decide from.
//
// It is unconditional today because nothing establishes an identity yet. When a narrow rule for the
// standard printed shape lands, this becomes its else-branch, and the gap count becomes the honest
// measure of how much curation is left -- which is a number worth having, since the one available
// beforehand came from a detector that could not tell a part number from a document number.
func gapUnidentifiedDocument(d *docpb.Document, manifest *derivepb.RunManifest) {
	manifest.Gaps = append(manifest.Gaps, &derivepb.Gap{
		Kind: "unidentified-document",
		Detail: fmt.Sprintf("no document number or revision recorded, so a citation cannot say which "+
			"revision it cites; the doc-IR titles itself %q, which is the part rather than the document. "+
			"Opening prose: %s", d.GetTitle(), identityEvidence(d)),
	})
}

// identityEvidence renders the document's opening text blocks, where a printed identity lives when
// it is anywhere. Page one specifically: a title block is a cover-page thing, and a document whose
// cover is a company-transition notice (a real shape) will show exactly that, which is itself the
// answer to why nothing was found.
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

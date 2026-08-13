package param

import (
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Verification state: whether a person has stood behind a value, and whether that still means
// anything given the document may have moved on.
//
// The distinction this exists to make is that "nobody checked" and "someone checked a revision we no
// longer have" are different answers with different next steps, and a boolean collapses them. So
// does the older convention of reading confidence == 1.0 as verified: a float cannot say WHICH
// revision was checked, so it cannot notice when that revision is superseded.

// VerificationState is what is known about a value's human confirmation.
type VerificationState string

const (
	// Unverified: no one has confirmed this value. The ordinary state of anything an extractor
	// produced, and honest.
	Unverified VerificationState = "unverified"
	// Verified: a person confirmed it against the revision in hand.
	Verified VerificationState = "verified"
	// Stale: a person confirmed it against a DIFFERENT revision of the document. Not wrong, and not
	// trustworthy either: the vendor may have changed the very table it was read from. It needs
	// re-confirming, which is a much smaller job than finding it again.
	Stale VerificationState = "stale"
	// Unknown: a verification exists but no current revision was supplied, so drift cannot be ruled
	// out. Deliberately NOT folded into Verified: a caller that cannot check must not be told the
	// answer is fine, which is the same discipline the outcome vocabulary applies to a check that
	// could not run.
	Unknown VerificationState = "unknown"
)

// VerificationOf reports what is known about a parameter's confirmation, given the content hash of
// the document revision currently in hand.
//
// Pass the empty string when no document is available and the answer becomes Unknown rather than
// Verified. That is the point: staleness is derived from evidence, never remembered, so a caller
// without the evidence is told so instead of being reassured.
func VerificationOf(p *parampb.Parameter, currentDocHash string) VerificationState {
	v := p.GetVerification()
	if v.GetBy() == "" || v.GetDocContentHash() == "" {
		return Unverified
	}
	if currentDocHash == "" {
		return Unknown
	}
	if v.GetDocContentHash() != currentDocHash {
		return Stale
	}
	return Verified
}

// VerificationOfIn is VerificationOf with the current revision resolved from the spec the parameter
// belongs to, by following its provenance doc_ref to that SourceDoc's content_hash.
//
// This is the form nearly every caller wants, and it exists because the alternative is worse than
// verbose. A caller that has to source the hash itself has to know where a corpus keeps documents,
// which puts filesystem knowledge on the check path and gives every call site its own chance to pass
// the wrong document's hash. Here the join is the one the data already states: a value cites a doc_ref,
// and that SourceDoc says which revision the corpus holds.
//
// An unresolvable doc_ref, or a SourceDoc with no recorded hash, yields Unknown for a verified value
// rather than Verified. A missing comparison input is a missing answer, never a passing one.
func VerificationOfIn(spec *parampb.PartSpec, p *parampb.Parameter) VerificationState {
	return VerificationOf(p, DocContentHash(spec, p.GetProv().GetDocRef()))
}

// SourceDocOf resolves a doc_ref to the document it names within a spec; nil when the id names none.
func SourceDocOf(spec *parampb.PartSpec, docRef string) *parampb.SourceDoc {
	for _, d := range spec.GetDocs() {
		if d.GetId() == docRef {
			return d
		}
	}
	return nil
}

// DocContentHash resolves a doc_ref to the content hash of the revision the spec describes; "" when
// the id names no doc or the corpus recorded none, which every caller must read as unknown.
func DocContentHash(spec *parampb.PartSpec, docRef string) string {
	return SourceDocOf(spec, docRef).GetContentHash()
}

// MarkVerified records that a person checked a value against a specific revision of a document.
//
// It takes the DOCUMENT rather than a bare hash so the invalidation key and the human-readable
// revision snapshot are read from one place and cannot disagree. Passing them separately would let a
// caller pin the hash of one revision beside the printed name of another, and the resulting record
// would be wrong in the one way nothing downstream could detect: it would go stale correctly and
// then name the wrong revision to the person asked to re-confirm it.
//
// It also raises provenance confidence to 1.0, which is not decoration: the pre-existing convention
// across this layer is that only a human verification earns 1.0, and consumers already read it that
// way. Keeping both in step means a reader of the old signal is not misled while the new one becomes
// available, which is what makes this additive rather than a migration. Note that nothing lowers it
// again, which is why anything deciding whether to TRUST a value reads VerificationOfIn and not the
// float (see DECISIONS.md, "A stale verification is untrustworthy data").
//
// Verifying against no document, or against one whose revision the corpus never recorded, is
// refused. The record would assert someone checked something without saying what, and a verification
// nobody can invalidate is the failure this whole type exists to prevent.
func MarkVerified(p *parampb.Parameter, by string, doc *parampb.SourceDoc, at, note string) bool {
	if p == nil || by == "" || doc.GetContentHash() == "" {
		return false
	}
	p.Verification = &parampb.Verification{
		By:             by,
		DocContentHash: doc.GetContentHash(),
		DocRevision:    doc.GetTitle(),
		At:             at,
		Note:           note,
	}
	if p.Prov == nil {
		p.Prov = &parampb.ParamProvenance{}
	}
	p.Prov.Confidence = 1.0
	return true
}

// MarkVerifiedIn is MarkVerified with the document resolved from the spec, by following the
// parameter's own provenance doc_ref. This is the form a portal wants: a person confirms a value
// against the document it cites, and neither the hash nor the revision is the caller's to supply.
//
// Refused when the doc_ref names no document in the spec, since a value citing a document the corpus
// does not have is not something anyone can have checked.
func MarkVerifiedIn(spec *parampb.PartSpec, p *parampb.Parameter, by, at, note string) bool {
	return MarkVerified(p, by, SourceDocOf(spec, p.GetProv().GetDocRef()), at, note)
}

// StaleVerifications lists the parameters of a spec whose verification was performed against a
// different revision than the one the spec now describes, so a portal can offer "re-confirm these"
// after a document updates rather than waiting for someone to notice.
//
// Each parameter is compared against the document IT cites, not against one hash for the whole spec.
// That distinction is not hypothetical: a spec routinely carries a datasheet and an app note, and
// judging every parameter against a single revision reports every value citing the OTHER document as
// stale the moment either one moves.
//
// A spec with nothing verified returns nothing, which is the ordinary state of a freshly seeded
// corpus and not a problem to report.
func StaleVerifications(spec *parampb.PartSpec) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.GetParameters() {
		if VerificationOfIn(spec, p) == Stale {
			out = append(out, p)
		}
	}
	return out
}

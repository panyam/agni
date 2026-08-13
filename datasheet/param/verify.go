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

// MarkVerified records that a person checked a value against a specific revision.
//
// It also raises provenance confidence to 1.0, which is not decoration: the pre-existing convention
// across this layer is that only a human verification earns 1.0, and consumers already read it that
// way. Keeping both in step means a reader of the old signal is not misled while the new one becomes
// available, which is what makes this additive rather than a migration.
//
// Verifying against no document is refused. The record would assert someone checked something
// without saying what, and a verification nobody can invalidate is the failure this whole type
// exists to prevent.
func MarkVerified(p *parampb.Parameter, by, docContentHash, at, note string) bool {
	if p == nil || by == "" || docContentHash == "" {
		return false
	}
	p.Verification = &parampb.Verification{
		By: by, DocContentHash: docContentHash, At: at, Note: note,
	}
	if p.Prov == nil {
		p.Prov = &parampb.ParamProvenance{}
	}
	p.Prov.Confidence = 1.0
	return true
}

// StaleVerifications lists the parameters of a spec whose verification was performed against a
// different revision than the one in hand, so a portal can offer "re-confirm these" after a document
// updates rather than waiting for someone to notice.
//
// A spec with nothing verified returns nothing, which is the ordinary state of a freshly seeded
// corpus and not a problem to report.
func StaleVerifications(spec *parampb.PartSpec, currentDocHash string) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.GetParameters() {
		if VerificationOf(p, currentDocHash) == Stale {
			out = append(out, p)
		}
	}
	return out
}

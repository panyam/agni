package param

import (
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func verifiedParam(hash string) *parampb.Parameter {
	p := &parampb.Parameter{Symbol: "VCC", Prov: &parampb.ParamProvenance{DocRef: "d", Confidence: 0.9}}
	MarkVerified(p, "sri", hash, "2026-08-13", "")
	return p
}

func TestVerificationStates(t *testing.T) {
	cases := []struct {
		name    string
		p       *parampb.Parameter
		current string
		want    VerificationState
	}{
		{"nobody checked", &parampb.Parameter{Symbol: "VCC"}, "sha256:a", Unverified},
		{"checked against the revision in hand", verifiedParam("sha256:a"), "sha256:a", Verified},
		{"checked against a superseded revision", verifiedParam("sha256:a"), "sha256:b", Stale},
		{"no revision to compare against", verifiedParam("sha256:a"), "", Unknown},
	}
	for _, tc := range cases {
		if got := VerificationOf(tc.p, tc.current); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A caller that cannot check must not be told the answer is fine. Folding Unknown into Verified is
// exactly how a stale fact keeps its badge.
func TestUnknownIsNotVerified(t *testing.T) {
	if VerificationOf(verifiedParam("sha256:a"), "") == Verified {
		t.Error("with no revision in hand, drift cannot be ruled out and must not be reported as verified")
	}
}

// The old signal and the new one stay in step, so a consumer reading confidence is not misled while
// the explicit state becomes available.
func TestMarkVerifiedKeepsTheOlderSignalInStep(t *testing.T) {
	p := verifiedParam("sha256:a")
	if p.Prov.GetConfidence() != 1.0 {
		t.Errorf("confidence = %v, want 1.0: only a human verification earns it", p.Prov.GetConfidence())
	}
	if p.GetVerification().GetBy() != "sri" || p.GetVerification().GetDocContentHash() != "sha256:a" {
		t.Errorf("verification record incomplete: %+v", p.GetVerification())
	}
}

// A verification nobody can invalidate is the failure this type exists to prevent.
func TestMarkVerifiedRefusesAnUnanchoredClaim(t *testing.T) {
	p := &parampb.Parameter{Symbol: "VCC"}
	if MarkVerified(p, "sri", "", "2026-08-13", "") {
		t.Error("verifying against no document must be refused")
	}
	if MarkVerified(p, "", "sha256:a", "2026-08-13", "") {
		t.Error("an anonymous verification cannot be questioned and must be refused")
	}
	if p.GetVerification() != nil {
		t.Error("a refused verification must leave no record")
	}
}

// specWithDoc builds a one-document spec whose SourceDoc records the revision the corpus holds.
func specWithDoc(docHash string, params ...*parampb.Parameter) *parampb.PartSpec {
	return &parampb.PartSpec{
		Mpn:        "ACME-1",
		Docs:       []*parampb.SourceDoc{{Id: "d", Title: "ACME-1 datasheet", ContentHash: docHash}},
		Parameters: params,
	}
}

// After a revision lands, a portal can offer "re-confirm these" rather than waiting for someone to
// notice the document moved. The revision is the one the SPEC records, so nobody has to supply it.
func TestStaleVerificationsAfterARevision(t *testing.T) {
	same := specWithDoc("sha256:relK",
		verifiedParam("sha256:relK"),
		verifiedParam("sha256:relK"),
		&parampb.Parameter{Symbol: "IOUT"}, // never verified: not stale, just unverified
	)
	if got := StaleVerifications(same); len(got) != 0 {
		t.Errorf("same revision, nothing stale: %d", len(got))
	}

	// The vendor ships rev L: the corpus re-seeds and the SourceDoc hash moves, which is the whole
	// invalidation mechanism. Nothing about the verifications themselves changed.
	moved := specWithDoc("sha256:relL",
		verifiedParam("sha256:relK"),
		verifiedParam("sha256:relK"),
		&parampb.Parameter{Symbol: "IOUT"},
	)
	got := StaleVerifications(moved)
	if len(got) != 2 {
		t.Fatalf("a revision must invalidate both verified values, got %d", len(got))
	}
	for _, p := range got {
		if VerificationOfIn(moved, p) != Stale {
			t.Error("returned a parameter that is not stale")
		}
	}
}

// A spec carrying a datasheet AND an app note must judge each value against the document IT cites.
// Comparing every parameter to one hash reports everything citing the other document as stale, which
// is a false alarm on a shape that is ordinary rather than exotic.
func TestStalenessIsPerDocumentNotPerSpec(t *testing.T) {
	ds := &parampb.Parameter{Symbol: "VCC", Prov: &parampb.ParamProvenance{DocRef: "ds", Confidence: 0.9}}
	MarkVerified(ds, "sri", "sha256:relL", "2026-08-13", "")
	an := &parampb.Parameter{Symbol: "RPULLUP", Prov: &parampb.ParamProvenance{DocRef: "an", Confidence: 0.9}}
	MarkVerified(an, "sri", "sha256:note1", "2026-08-13", "")

	spec := &parampb.PartSpec{Mpn: "ACME-1", Parameters: []*parampb.Parameter{ds, an}, Docs: []*parampb.SourceDoc{
		{Id: "ds", Title: "ACME-1 datasheet", ContentHash: "sha256:relL"},
		{Id: "an", Title: "ACME-1 application note", ContentHash: "sha256:note1"},
	}}
	if got := StaleVerifications(spec); len(got) != 0 {
		t.Fatalf("both values match their own document's revision, nothing is stale: %d", len(got))
	}

	// Only the datasheet is re-seeded. The app-note value must be untouched.
	spec.Docs[0].ContentHash = "sha256:relM"
	got := StaleVerifications(spec)
	if len(got) != 1 || got[0].GetSymbol() != "VCC" {
		t.Fatalf("only the value citing the revised document is stale, got %+v", got)
	}
	if VerificationOfIn(spec, an) != Verified {
		t.Error("revising the datasheet must not invalidate a value read from the application note")
	}
}

// A value citing a document the spec does not list, or one whose revision the corpus never recorded,
// cannot be judged. That has to read as unknown: an unresolvable comparison is a missing answer.
func TestUnresolvableDocumentIsUnknownNotVerified(t *testing.T) {
	orphan := specWithDoc("sha256:a", verifiedParam("sha256:a"))
	orphan.Docs[0].Id = "somethingelse"
	if got := VerificationOfIn(orphan, orphan.Parameters[0]); got != Unknown {
		t.Errorf("doc_ref resolving to no document: got %q, want %q", got, Unknown)
	}

	unrecorded := specWithDoc("", verifiedParam("sha256:a"))
	if got := VerificationOfIn(unrecorded, unrecorded.Parameters[0]); got != Unknown {
		t.Errorf("SourceDoc with no recorded hash: got %q, want %q", got, Unknown)
	}
}

// Degrade-safety: a spec seeded before verification existed reads as unverified rather than breaking.
func TestSpecsWithoutVerificationAreUnverified(t *testing.T) {
	spec := specWithDoc("sha256:a", &parampb.Parameter{Symbol: "VCC"}, &parampb.Parameter{Symbol: "IOUT"})
	if got := StaleVerifications(spec); len(got) != 0 {
		t.Errorf("nothing was ever verified, so nothing can be stale: %d", len(got))
	}
	for _, p := range spec.Parameters {
		if VerificationOfIn(spec, p) != Unverified {
			t.Error("an unseeded verification must read as unverified")
		}
	}
}

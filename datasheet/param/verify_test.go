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

// After a revision lands, a portal can offer "re-confirm these" rather than waiting for someone to
// notice the document moved.
func TestStaleVerificationsAfterARevision(t *testing.T) {
	spec := &parampb.PartSpec{Mpn: "ACME-1", Parameters: []*parampb.Parameter{
		verifiedParam("sha256:relK"),
		verifiedParam("sha256:relK"),
		{Symbol: "IOUT"}, // never verified: not stale, just unverified
	}}
	if got := StaleVerifications(spec, "sha256:relK"); len(got) != 0 {
		t.Errorf("same revision, nothing stale: %d", len(got))
	}
	got := StaleVerifications(spec, "sha256:relL")
	if len(got) != 2 {
		t.Fatalf("a revision must invalidate both verified values, got %d", len(got))
	}
	for _, p := range got {
		if VerificationOf(p, "sha256:relL") != Stale {
			t.Error("returned a parameter that is not stale")
		}
	}
}

// Degrade-safety: a spec seeded before verification existed reads as unverified rather than breaking.
func TestSpecsWithoutVerificationAreUnverified(t *testing.T) {
	spec := &parampb.PartSpec{Parameters: []*parampb.Parameter{{Symbol: "VCC"}, {Symbol: "IOUT"}}}
	if got := StaleVerifications(spec, "sha256:a"); len(got) != 0 {
		t.Errorf("nothing was ever verified, so nothing can be stale: %d", len(got))
	}
	for _, p := range spec.Parameters {
		if VerificationOf(p, "sha256:a") != Unverified {
			t.Error("an unseeded verification must read as unverified")
		}
	}
}

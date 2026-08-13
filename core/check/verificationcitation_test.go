package check

import (
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

// A stale citation has to name BOTH revisions, because that is the difference between a report
// someone can act on and one they cannot. Doc resolves from the CURRENT SourceDoc, so after a re-seed
// it names the new revision and the checked one survives only in the verification's own snapshot.
func TestStaleCitationNamesBothRevisions(t *testing.T) {
	spec := &parampb.PartSpec{
		Mpn:  "ACME-33",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "SCES650K - REVISED JANUARY 2023", ContentHash: "sha256:relK"}},
		Parameters: []*parampb.Parameter{{
			Symbol: "VDD",
			Prov:   &parampb.ParamProvenance{DocRef: "ds", Page: 4, TableOrFigure: "Absolute Maximum Ratings", Method: "hand", Confidence: 0.9},
		}},
	}
	p := spec.Parameters[0]
	if !param.MarkVerifiedIn(spec, p, "sri", "2026-08-13", "") {
		t.Fatal("verifying against the spec's own document must succeed")
	}

	// Fresh: the citation reads verified and both sides agree.
	if c := DatasheetCitationOf(spec, p); c.Verification != string(param.Verified) {
		t.Fatalf("before any revision: verification = %q, want %q", c.Verification, param.Verified)
	}

	// The vendor ships rev L. A re-seed overwrites BOTH SourceDoc fields.
	spec.Docs[0].ContentHash = "sha256:relL"
	spec.Docs[0].Title = "SCES650L - REVISED MARCH 2026"

	c := DatasheetCitationOf(spec, p)
	if c.Verification != string(param.Stale) {
		t.Errorf("verification = %q, want %q", c.Verification, param.Stale)
	}
	if c.Doc != "SCES650L - REVISED MARCH 2026" {
		t.Errorf("Doc = %q, want the revision the corpus holds now", c.Doc)
	}
	if c.VerifiedRevision != "SCES650K - REVISED JANUARY 2023" {
		t.Errorf("VerifiedRevision = %q, want the revision that was actually checked", c.VerifiedRevision)
	}
}

// Nothing verified means nothing to snapshot. An empty string here is what keeps the field out of the
// JSON report and off the wire for the ordinary case, which is every value in a freshly seeded corpus.
func TestUnverifiedCitationCarriesNoRevisionSnapshot(t *testing.T) {
	spec := &parampb.PartSpec{
		Mpn:  "ACME-33",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "SCES650K", ContentHash: "sha256:relK"}},
		Parameters: []*parampb.Parameter{{
			Symbol: "VDD",
			Prov:   &parampb.ParamProvenance{DocRef: "ds", Page: 4, Method: "derive/v0", Confidence: 0.9},
		}},
	}
	c := DatasheetCitationOf(spec, spec.Parameters[0])
	if c.Verification != string(param.Unverified) {
		t.Errorf("verification = %q, want %q", c.Verification, param.Unverified)
	}
	if c.VerifiedRevision != "" {
		t.Errorf("VerifiedRevision = %q, want empty: nobody verified anything", c.VerifiedRevision)
	}
}

// A citation built from a bare provenance (a pin declaration, a relation bound) has no parameter and
// therefore no verification to report. It must not inherit one from elsewhere in the spec.
func TestProvOnlyCitationHasNoVerification(t *testing.T) {
	spec := &parampb.PartSpec{
		Mpn:  "ACME-33",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "SCES650K", ContentHash: "sha256:relK"}},
	}
	c := DatasheetCitationOfProv(spec, &parampb.ParamProvenance{DocRef: "ds", Page: 2, Method: "hand", Confidence: 1})
	if c.Verification != "" || c.VerifiedRevision != "" {
		t.Errorf("prov-only citation carried verification %q / revision %q, want both empty", c.Verification, c.VerifiedRevision)
	}
}

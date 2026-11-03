package derive

import (
	"testing"

	"github.com/panyam/agni/core/param"
)

// The seeder is the only producer of SourceDoc.content_hash. Downstream code reads it to tell a
// stale verification from a current one. Imagine a run leaves it empty. Its corpus can then never
// conclude staleness, only "unknown", and "unknown" demotes every verified value.
func TestRunRecordsTheRevisionItDerivedFrom(t *testing.T) {
	d := loadDocFixture(t, "bss138-raw-docir.textproto")
	recipes, patches := loadArtifacts(t)

	spec, _, err := Run(d, recipes, patches, Identity{MPN: "BSS138", Manufacturer: "onsemi", Locator: "onsemi/BSS138/BSS138.textproto"})
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.GetDocs()) != 1 {
		t.Fatalf("want one SourceDoc, got %d", len(spec.GetDocs()))
	}
	sd := spec.GetDocs()[0]
	if sd.GetContentHash() != d.GetContentHash() {
		t.Errorf("content_hash = %q, want the document's own %q", sd.GetContentHash(), d.GetContentHash())
	}
	// The hash used to be written into locator, which is a path field. A reader resolving the locator
	// would have got a hash, and a reader comparing revisions would have got nothing.
	if sd.GetLocator() != "onsemi/BSS138/BSS138.textproto" {
		t.Errorf("locator = %q, want the operator-supplied path", sd.GetLocator())
	}
}

// This tests the field's purpose end to end, through the real seeder. Verify a derived value
// against the revision it came from, re-derive from a revised document, and the verification must
// stop counting without anyone touching it.
func TestAReDeriveFromANewRevisionStalesAVerification(t *testing.T) {
	d := loadDocFixture(t, "bss138-raw-docir.textproto")
	recipes, patches := loadArtifacts(t)
	id := Identity{MPN: "BSS138", Manufacturer: "onsemi"}

	spec, _, err := Run(d, recipes, patches, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.GetParameters()) == 0 {
		t.Fatal("fixture derived no parameters, so this test proves nothing")
	}
	// derive leaves the document identity unstated (issue 290) because it cannot read the printed
	// revision yet. A part number in that field would be a citation that cannot name what it cites.
	// An author curates the field. The test curates it here, so the snapshot below has something to
	// record.
	spec.Docs[0].Title = "BSS138 datasheet - REVISED MAY 2019"

	p := spec.GetParameters()[0]
	if !param.MarkVerifiedIn(spec, p, "sri", "2026-08-13", "") {
		t.Fatal("a derived value must be verifiable against the document it was derived from")
	}
	if got := param.VerificationOfIn(spec, p); got != param.Verified {
		t.Fatalf("just verified: got %q, want %q", got, param.Verified)
	}

	// The vendor reissues. Same part, same recipes, different bytes.
	revised := loadDocFixture(t, "bss138-raw-docir.textproto")
	revised.ContentHash = "sha256:areissue"
	newSpec, _, err := Run(revised, recipes, patches, id)
	if err != nil {
		t.Fatal(err)
	}

	// The corpus re-seeds. The spec's documents move to the new revision. The verification lives on
	// the fact, and it is still pinned to the old one.
	spec.Docs = newSpec.GetDocs()
	if got := param.VerificationOfIn(spec, p); got != param.Stale {
		t.Errorf("after a re-derive from a new revision: got %q, want %q", got, param.Stale)
	}
	if got := p.GetVerification().GetDocRevision(); got != "BSS138 datasheet - REVISED MAY 2019" {
		t.Errorf("the revision that was checked must survive the re-seed, or the re-confirm task cannot name it; got %q", got)
	}
}

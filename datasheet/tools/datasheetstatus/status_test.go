package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/panyam/agni/datasheet/corpus"
)

func TestClassify(t *testing.T) {
	const (
		h1  = "sha256:aaa"
		h2  = "sha256:bbb"
		tc  = "docling/2.5.1"
		old = "docling/2.4.0"
	)
	cases := []struct {
		name                                     string
		docExists                                bool
		pdfHash, storedHash, producer, curToolch string
		want                                     PDFStatus
	}{
		{"no sibling", false, h1, "", "", tc, NotExtracted},
		{"hash matches, toolchain matches", true, h1, h1, tc, tc, Fresh},
		{"hash matches, toolchain unknown -> fresh", true, h1, h1, tc, "", Fresh},
		{"pdf bytes changed", true, h2, h1, tc, tc, StaleSource},
		{"toolchain drifted", true, h1, h1, old, tc, StaleToolchain},
		{"drifted but unknown current -> fresh", true, h1, h1, old, "", Fresh},
	}
	for _, c := range cases {
		if got := classify(c.docExists, c.pdfHash, c.storedHash, c.producer, c.curToolch); got != c.want {
			t.Errorf("%s: classify = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestNeedsExtraction(t *testing.T) {
	want := map[PDFStatus]bool{
		NotExtracted:   true,
		StaleSource:    true,
		StaleToolchain: false,
		Fresh:          false,
	}
	for s, w := range want {
		if s.needsExtraction() != w {
			t.Errorf("%q.needsExtraction() = %v, want %v", s, s.needsExtraction(), w)
		}
	}
}

// TestHashPDF pins the digest of a known input so the hash stays byte-for-byte compatible with
// tools/pdf2doc (sha256 over raw bytes, "sha256:" prefix). If this drifts, extracted files would
// spuriously read stale-source.
func TestHashPDF(t *testing.T) {
	const want = "sha256:febf683c0e4eb3ab8872459fd5e67aee3e35ae8a928f187a8506dcd48692f0c4"
	p := filepath.Join(t.TempDir(), "x.pdf")
	if err := os.WriteFile(p, []byte("datasheet-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := hashPDF(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("hashPDF = %q, want %q", got, want)
	}
}

// A part reports the drafts citing its PDFs, by the mount URI the workbench cites them with, and
// whether each draft's MPN is published; a draft citing another datasheet is not attached.
func TestAttachDraftsByCitedDocument(t *testing.T) {
	draft := func(mpn, doc string) string {
		return `{"mpn": "` + mpn + `", "spec": {"mpn": "` + mpn + `"}, "documentUris": ["` + doc + `"]}`
	}
	store := fstest.MapFS{
		corpus.DraftFile("LM1117-3.3"): {Data: []byte(draft("LM1117-3.3", "mount://ds/ti/LM1117/LM1117.pdf"))},
		corpus.DraftFile("LM1117-5.0"): {Data: []byte(draft("LM1117-5.0", "mount://ds/ti/LM1117/LM1117.pdf"))},
		corpus.DraftFile("BSS138"):     {Data: []byte(draft("BSS138", "mount://ds/onsemi/BSS138/BSS138.pdf"))},
		corpus.IndexFile:               {Data: []byte(`{"generation": 3, "entries": [{"mpn": "LM1117-3.3", "file": "LM1117-3.3.textproto", "hash": "sha256:x"}]}`)},
	}
	parts := []partInfo{{name: "ti/LM1117", pdfs: []pdfInfo{{name: "LM1117.pdf", uri: "mount://ds/ti/LM1117/LM1117.pdf"}}}}
	if err := attachDrafts(parts, store); err != nil {
		t.Fatal(err)
	}
	if got := draftColumn(parts[0].drafts); got != "LM1117-3.3 (published), LM1117-5.0 (draft)" {
		t.Errorf("draft column = %q", got)
	}
	if got := draftColumn(nil); got != "no-draft" {
		t.Errorf("no drafts = %q", got)
	}
}

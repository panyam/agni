package main

import (
	"os"
	"path/filepath"
	"testing"
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

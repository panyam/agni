package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/datasheet/dsservice"
	"github.com/panyam/agni/internal/mountset"
)

// stubProducer writes a minimal valid doc-IR to its -o path (invoked as `<script> <pdf> -o <out>`,
// so the output path is $3). It stands in for docling, which is CI-excluded.
const stubProducer = "#!/bin/sh\ncat > \"$3\" <<'DOC'\ncontent_hash: \"sha256:stub\"\nproducer: \"stub\"\npage_count: 1\npages { number: 1 width: 100 height: 100 }\nDOC\n"

func TestOsDocExtractor(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "stub.sh")
	if err := os.WriteFile(stub, []byte(stubProducer), 0o755); err != nil {
		t.Fatal(err)
	}

	// No configured command -> unavailable.
	if (&osDocExtractor{}).Available() {
		t.Error("empty cmd should be unavailable")
	}

	corpus := t.TempDir()
	writePDF(t, filepath.Join(dir, "LM1117.pdf"), "pdf-lm1117")
	ms := []mountset.Mount{{Name: "m", Root: dir}}
	e := &osDocExtractor{derived: derived{mounts: tableOf(ms), corpus: corpus}, cmd: []string{stub}}
	if !e.Available() {
		t.Fatal("a configured cmd should be available")
	}

	// A run writes the doc-IR into the store and returns the parsed + validated doc-IR.
	d, err := e.Extract(context.Background(), mustURI("m", "LM1117.pdf"))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if d.GetContentHash() != "sha256:stub" || d.GetPageCount() != 1 {
		t.Errorf("produced doc-IR: %+v", d)
	}
	if _, err := os.Stat(filepath.Join(corpus, "derived", sha256Hex("pdf-lm1117"), "doc.txtpb")); err != nil {
		t.Fatalf("doc-IR not written to the store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "LM1117.doc.textproto")); !os.IsNotExist(err) {
		t.Errorf("a doc-IR was written beside the PDF: %v", err)
	}

	// With no corpus there is nowhere to keep the doc-IR (agni issue 1734).
	bare := &osDocExtractor{derived: derived{mounts: tableOf(ms)}, cmd: []string{stub}}
	if _, err := bare.Extract(context.Background(), mustURI("m", "LM1117.pdf")); !errors.Is(err, dsservice.ErrNoCorpus) {
		t.Errorf("Extract with no corpus = %v, want ErrNoCorpus", err)
	}

	// A failing command is an error (not a silent success).
	bad := &osDocExtractor{derived: e.derived, cmd: []string{"false"}}
	if _, err := bad.Extract(context.Background(), mustURI("m", "LM1117.pdf")); err == nil {
		t.Error("a failing producer command should error")
	}
}

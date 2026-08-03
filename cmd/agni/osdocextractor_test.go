package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/internal/mounts"
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

	e := &osDocExtractor{mounts: []mounts.Mount{{Name: "m", Root: dir}}, cmd: []string{stub}}
	if !e.Available() {
		t.Fatal("a configured cmd should be available")
	}

	// A run writes the sibling and returns the parsed + validated doc-IR.
	d, err := e.Extract(context.Background(), "m", "LM1117.pdf")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if d.GetContentHash() != "sha256:stub" || d.GetPageCount() != 1 {
		t.Errorf("produced doc-IR: %+v", d)
	}
	if _, err := os.Stat(filepath.Join(dir, "LM1117.doc.textproto")); err != nil {
		t.Fatalf("sibling not written: %v", err)
	}

	// A failing command is an error (not a silent success).
	bad := &osDocExtractor{mounts: e.mounts, cmd: []string{"false"}}
	if _, err := bad.Extract(context.Background(), "m", "LM1117.pdf"); err == nil {
		t.Error("a failing producer command should error")
	}
}

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/internal/mounts"
)

func TestDocSibling(t *testing.T) {
	cases := map[string]string{
		"LM1117.pdf":    "LM1117.doc.textproto",
		"ti/LM1117.pdf": "ti/LM1117.doc.textproto",
		"a/b/Part.PDF":  "a/b/Part.doc.textproto", // extension case is preserved by TrimSuffix of Ext
	}
	for in, want := range cases {
		if got := docSibling(in); got != want {
			t.Errorf("docSibling(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOsDocLoaderDocument(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "d.doc.textproto"),
		[]byte(`content_hash: "sha256:abc" producer: "hand" page_count: 1`), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &osDocLoader{mounts: []mounts.Mount{{Name: "m", Root: dir}}}

	// A datasheet whose sibling doc-IR exists parses it.
	doc, err := l.Document(context.Background(), "m", "d.pdf")
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	if doc == nil || doc.GetContentHash() != "sha256:abc" {
		t.Fatalf("want parsed doc-IR, got %+v", doc)
	}

	// A datasheet with no sibling is (nil, nil): "not yet extracted", a normal state.
	missing, err := l.Document(context.Background(), "m", "other.pdf")
	if err != nil || missing != nil {
		t.Fatalf("absent sibling => (%v, %v), want (nil, nil)", missing, err)
	}

	// An unknown mount is a classified error, not a panic.
	if _, err := l.Document(context.Background(), "nope", "d.pdf"); err == nil {
		t.Fatal("unknown mount should error")
	}
}

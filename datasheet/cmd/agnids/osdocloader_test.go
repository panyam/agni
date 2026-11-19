package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/internal/mountset"
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
	l := &osDocLoader{derived: derived{mounts: tableOf([]mountset.Mount{{Name: "m", Root: dir}})}}

	// With no corpus, a doc-IR beside the PDF, from `make pdf2doc`, is still read.
	doc, err := l.Document(context.Background(), mustURI("m", "d.pdf"))
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	if doc == nil || doc.GetContentHash() != "sha256:abc" {
		t.Fatalf("want parsed doc-IR, got %+v", doc)
	}

	// A datasheet with no doc-IR returns (nil, nil). It means "not yet extracted", a normal state.
	missing, err := l.Document(context.Background(), mustURI("m", "other.pdf"))
	if err != nil || missing != nil {
		t.Fatalf("absent doc-IR => (%v, %v), want (nil, nil)", missing, err)
	}

	// An unknown mount is a classified error, not a panic.
	if _, err := l.Document(context.Background(), mustURI("nope", "d.pdf")); err == nil {
		t.Fatal("unknown mount should error")
	}
}

// The corpus store holds the doc-IR under the PDF's hash, and it wins over one beside the PDF. A PDF
// the store has no doc-IR for falls back to the old layout (agni issue 1734).
func TestOsDocLoaderReadsTheStoreFirst(t *testing.T) {
	dir, corpus := t.TempDir(), t.TempDir()
	writePDF(t, filepath.Join(dir, "d.pdf"), "pdf-d")
	writePDF(t, filepath.Join(dir, "e.pdf"), "pdf-e")
	for path, hash := range map[string]string{
		filepath.Join(dir, "d.doc.textproto"):                             "sha256:beside",
		filepath.Join(dir, "e.doc.textproto"):                             "sha256:beside-e",
		filepath.Join(corpus, "derived", sha256Hex("pdf-d"), "doc.txtpb"): "sha256:store",
	} {
		writePDF(t, path, `content_hash: "`+hash+`" producer: "hand" page_count: 1`)
	}
	l := &osDocLoader{derived: derived{mounts: tableOf([]mountset.Mount{{Name: "m", Root: dir}}), corpus: corpus}}
	for pdf, want := range map[string]string{"d.pdf": "sha256:store", "e.pdf": "sha256:beside-e"} {
		doc, err := l.Document(context.Background(), mustURI("m", pdf))
		if err != nil || doc.GetContentHash() != want {
			t.Errorf("Document(%s) = %v, %v, want %s", pdf, doc.GetContentHash(), err, want)
		}
	}
}

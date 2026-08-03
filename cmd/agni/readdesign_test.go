package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWarnEdsSibling: the guardrail warns only when reading a .eds whose sibling .edn exists (the
// silent-wrong-count footgun), and is silent on a lone .eds, on a .edn input, and case-insensitively.
func TestWarnEdsSibling(t *testing.T) {
	dir := t.TempDir()
	eds := filepath.Join(dir, "board.eds")
	if err := os.WriteFile(eds, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var b bytes.Buffer
	warnEdsSibling(eds, &b) // .eds, no sibling .edn -> silent
	if b.Len() != 0 {
		t.Errorf("no sibling: want no warning, got %q", b.String())
	}

	if err := os.WriteFile(filepath.Join(dir, "board.edn"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	warnEdsSibling(eds, &b) // .eds WITH sibling .edn -> warn
	if !strings.Contains(b.String(), "board.edn") || !strings.Contains(b.String(), "authoritative") {
		t.Errorf("with sibling: want a warning naming board.edn, got %q", b.String())
	}

	b.Reset()
	warnEdsSibling(filepath.Join(dir, "board.edn"), &b) // reading the .edn itself -> silent
	if b.Len() != 0 {
		t.Errorf(".edn input: want no warning, got %q", b.String())
	}

	// Case-insensitive: .EDS with a .EDN sibling still warns.
	up := t.TempDir()
	os.WriteFile(filepath.Join(up, "B.EDS"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(up, "B.EDN"), []byte("x"), 0o644)
	b.Reset()
	warnEdsSibling(filepath.Join(up, "B.EDS"), &b)
	if b.Len() == 0 {
		t.Errorf(".EDS with .EDN sibling: want a warning, got none")
	}
}

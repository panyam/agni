package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// migrate-drafts moves each per-datasheet .partspec.json into the store as the draft for the MPN it
// names, citing its datasheet. An empty seed is left behind, a draft with content but no MPN is
// reported, a dry run writes nothing, and a second run never overwrites what the first moved.
func TestMigrateDraftsKeysByMPN(t *testing.T) {
	ds := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(ds, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ti/LM1117.partspec.json", `{"mpn": "LM1117", "pins": [{"id": "vin", "name": "VIN"}]}`)
	write("ti/browsed.partspec.json", `{"docs": [{"id": "d", "title": "browsed"}]}`)
	write("ti/unnamed.partspec.json", `{"pins": [{"id": "gnd", "name": "GND"}]}`)
	corpusDir := t.TempDir()
	args := []string{"migrate-drafts", "--mount", "ds=" + ds, "--corpus", corpusDir}

	out, err := runAgnids(t, append(args, "--dry-run")...)
	if err != nil || !strings.Contains(out, "would move 1 draft(s); 1 empty seed(s) left behind; 1 skipped") {
		t.Fatalf("dry run: %q, %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(corpusDir, "drafts")); !os.IsNotExist(err) {
		t.Fatal("a dry run wrote drafts")
	}

	out, err = runAgnids(t, args...)
	if err != nil || !strings.Contains(out, "moved 1 draft(s)") || !strings.Contains(out, "unnamed.partspec.json: it has content but names no MPN") {
		t.Fatalf("migrate: %q, %v", out, err)
	}
	d, found, err := newOSDraftStore(corpusDir).Get(context.Background(), "LM1117")
	if err != nil || !found {
		t.Fatalf("no LM1117 draft after migrating: %v", err)
	}
	if len(d.GetDocumentUris()) != 1 || d.GetDocumentUris()[0] != "mount://ds/ti/LM1117.pdf" || len(d.GetSpec().GetPins()) != 1 {
		t.Errorf("migrated draft = %v", d)
	}

	out, err = runAgnids(t, args...)
	if err != nil || !strings.Contains(out, "the store already has a draft for LM1117") {
		t.Errorf("second run: %q, %v", out, err)
	}
}

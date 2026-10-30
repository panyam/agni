package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/readers/formats"
)

const backannotateDir = "testdata/github-issues/backannotate-locate"

// A schematic companion the descriptor declares and that fails to parse was silent everywhere but
// render, so a check or query read as though every part were drawn on no sheet (agni issue 1230). The
// read says so once per file, and the intact fixture is the control.
func TestLocalLoaderWarnsAnUnreadableCompanion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"design.yaml", "demo_netlist.edn", "demo_schematic.eds"} {
		b, err := os.ReadFile(filepath.Join(backannotateDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "demo_schematic.eds" {
			b = bytes.TrimRight(b, ")\n") // close nothing, as the fixture's first commit did
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		dir  string
		warn bool
	}{{dir, true}, {backannotateDir, false}} {
		freshWorkspace(t)
		var notes bytes.Buffer
		l := &localLoader{loader: &formats.Loader{}, notes: &notes}
		u := uriOf(t, filepath.Join(c.dir, "demo_netlist.edn"))
		for range 2 {
			_, err := l.Geometry(context.Background(), u, "faithful", false)
			if (err != nil) != c.warn {
				t.Fatalf("%s: geometry error %v", c.dir, err)
			}
		}
		got := strings.Count(notes.String(), "warning: demo_schematic.eds, the schematic design.yaml declares, could not be read")
		if c.warn && got != 1 {
			t.Errorf("unreadable companion warned %d times, want once:\n%s", got, notes.String())
		}
		if !c.warn && strings.Contains(notes.String(), "warning:") {
			t.Errorf("an intact companion warned:\n%s", notes.String())
		}
	}
}

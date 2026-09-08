package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestReadDesignUnknownExtension(t *testing.T) {
	p := filepath.Join(t.TempDir(), "design.txt")
	if err := os.WriteFile(p, []byte("whatever"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadDesign(p)
	if err == nil || !strings.Contains(err.Error(), "no reader") {
		t.Errorf("ReadDesign(.txt) error = %v, want a \"no reader\" error", err)
	}
}

func TestReadDesignIPCSniff(t *testing.T) {
	// A .xml that is not IPC-2581 must be rejected by the root sniff, not handed to the reader.
	p := filepath.Join(t.TempDir(), "board.xml")
	if err := os.WriteFile(p, []byte(`<?xml version="1.0"?><notipc/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadDesign(p)
	if err == nil || !strings.Contains(err.Error(), "not an IPC-2581") {
		t.Errorf("ReadDesign(non-IPC .xml) error = %v, want a \"not an IPC-2581\" error", err)
	}
}

// TestReadCarriesTheIngestionPasses is the guard this package did not have, and the reason it needed
// one is that its absence is silent.
//
// Both readers here used to dispatch straight to edif.Read and friends, skipping formats.Loader,
// which is where the format-neutral passes run. classify.StampMPN is one of them, so every example
// read a design whose components carried NO part number: nothing errored, the counts were right, and
// every datasheet-tier question answered "none" rather than failing. A query for uncovered parts
// grouped by MPN reported that the board was clean.
//
// Asserted on BOTH paths, because they are separate calls and a fix to one would look complete.
func TestReadCarriesTheIngestionPasses(t *testing.T) {
	const fixture = "probe-coverage.edn"
	for _, tc := range []struct {
		name string
		read func() (*ir.Design, error)
	}{
		{"embedded", func() (*ir.Design, error) { return ReadFixture(fixture) }},
		{"on disk", func() (*ir.Design, error) { return ReadDesign(filepath.Join("designs", fixture)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := tc.read()
			if err != nil {
				t.Fatal(err)
			}
			var withMPN int
			for _, c := range d.GetComponents() {
				if c.GetMpn() != "" {
					withMPN++
				}
			}
			if withMPN == 0 {
				t.Errorf("no component carries an MPN, so the read skipped classify.StampMPN; "+
					"%d components were read", len(d.GetComponents()))
			}
		})
	}
}

func TestReadDesignFromDisk(t *testing.T) {
	// ReadDesign exercises the os.Open edge; the bundled fixtures sit under designs/ relative
	// to this package during tests.
	d, err := ReadDesign("designs/two-resistors.edn")
	if err != nil {
		t.Fatalf("ReadDesign: %v", err)
	}
	if d.Name != "DEMO" {
		t.Errorf("Name = %q, want DEMO", d.Name)
	}
}

func TestReadDesignMissing(t *testing.T) {
	if _, err := ReadDesign("designs/nope.edn"); err == nil {
		t.Error("ReadDesign of a missing file should error")
	}
}

func TestLoadDiskPath(t *testing.T) {
	// A path on disk (relative to the working directory) is read directly.
	d, err := Load("designs/two-resistors.edn")
	if err != nil {
		t.Fatalf("Load(path): %v", err)
	}
	if d.Name != "DEMO" {
		t.Errorf("Name = %q, want DEMO", d.Name)
	}
}

func TestLoadFixtureFallback(t *testing.T) {
	// A bare name that is not a file at the cwd falls back to the embedded fixture.
	d, err := Load("two-resistors.edn")
	if err != nil {
		t.Fatalf("Load(fixture name): %v", err)
	}
	if d.Name != "DEMO" {
		t.Errorf("Name = %q, want DEMO", d.Name)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load("no-such-design.edn"); err == nil {
		t.Error("Load of a missing path and unknown fixture should error")
	}
}

func TestLoadParseErrorNotMasked(t *testing.T) {
	// A file that exists but fails to parse is reported, not masked by the fixture fallback.
	p := filepath.Join(t.TempDir(), "board.xml")
	if err := os.WriteFile(p, []byte("<not-ipc/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "not an IPC-2581") {
		t.Errorf("Load(bad .xml) = %v, want the IPC-2581 parse error surfaced", err)
	}
}

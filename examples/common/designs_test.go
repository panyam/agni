package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadByExtUnknown(t *testing.T) {
	_, err := readByExt(strings.NewReader("whatever"), "design.txt")
	if err == nil || !strings.Contains(err.Error(), "no reader") {
		t.Errorf("readByExt(.txt) error = %v, want a \"no reader\" error", err)
	}
}

func TestReadByExtIPCSniff(t *testing.T) {
	// A .xml that is not IPC-2581 must be rejected by the root sniff, not handed to the reader.
	_, err := readByExt(strings.NewReader(`<?xml version="1.0"?><notipc/>`), "board.xml")
	if err == nil || !strings.Contains(err.Error(), "not an IPC-2581") {
		t.Errorf("readByExt(non-IPC .xml) error = %v, want a \"not an IPC-2581\" error", err)
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

package expect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoad checks the sidecar parses both blocks, defaults fires to non-nil, and errors on a
// missing file (the caller decides which designs must have a sidecar).
func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.expect.yaml")
	if err := os.WriteFile(path, []byte(`fires:
  duplicate-ref-des: [U1]
  single-pin-net: [STUB, N$2]
pending:
  net-naming-convention: [BAD]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.Fires["single-pin-net"].Subjects, []string{"STUB", "N$2"}) {
		t.Errorf("fires single-pin-net = %v", e.Fires["single-pin-net"])
	}
	if !reflect.DeepEqual(e.Pending["net-naming-convention"].Subjects, []string{"BAD"}) {
		t.Errorf("pending = %v", e.Pending)
	}
}

// TestLoadEmpty: a sidecar with an empty fires map is a valid "no findings expected" (fires is
// non-nil so a caller can range it safely).
func TestLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.expect.yaml")
	if err := os.WriteFile(path, []byte("fires: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if e.Fires == nil || len(e.Fires) != 0 {
		t.Errorf("empty fires = %v, want non-nil empty", e.Fires)
	}
}

// TestLoadWhyLongForm: an entry may be a mapping ({subjects, why}) instead of a bare subject
// list, so a fixture can narrate its intent (WS6-008); short and long forms mix freely.
func TestLoadWhyLongForm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.expect.yaml")
	if err := os.WriteFile(path, []byte(`fires:
  decoupling-present:
    subjects: [VCC1]
    why: "VCC1 has no cap; VCC2 is the control with C1"
  single-pin-net: [STUB]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	dp := e.Fires["decoupling-present"]
	if !reflect.DeepEqual(dp.Subjects, []string{"VCC1"}) {
		t.Errorf("long-form subjects = %v", dp.Subjects)
	}
	if dp.Why != "VCC1 has no cap; VCC2 is the control with C1" {
		t.Errorf("why = %q", dp.Why)
	}
	if got := e.Fires["single-pin-net"]; !reflect.DeepEqual(got.Subjects, []string{"STUB"}) || got.Why != "" {
		t.Errorf("short form = %+v", got)
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.expect.yaml")); err == nil {
		t.Error("missing file should error")
	}
}

package edif

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestPortInstanceMapping is the WS1-025 acceptance: connection pin identity resolves
// through the instance's portInstance table to PHYSICAL pin designators. J1's logical
// GND port fans out to pins 5 and 6 (one portRef, two connections); its SIG port maps to
// pin 2; the "&1"-style port keeps its stripped name (which equals its designator — the
// key-stability path for the common case); J2's GND maps independently to its own pin 6
// without colliding with J1's.
func TestPortInstanceMapping(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "dup_ports.edn"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Read(bytes.NewReader(raw), "dup_ports.edn")
	if err != nil {
		t.Fatal(err)
	}
	pins := map[string][]string{} // net -> "ref.pin"
	for _, n := range d.Nets {
		for _, c := range n.Connections {
			pins[n.Name] = append(pins[n.Name], c.ComponentRef+"."+c.PinRef)
		}
	}
	want := map[string][]string{
		"GND_A": {"J1.5", "J1.6", "J1.2"}, // GND fans out to 5+6, SIG maps to 2
		"GND_B": {"J2.6", "J1.1"},         // J2's own mapping; &1 falls through as "1"
	}
	for net, exp := range want {
		got := pins[net]
		if len(got) != len(exp) {
			t.Fatalf("%s connections = %v, want %v", net, got, exp)
		}
		for i := range exp {
			if got[i] != exp[i] {
				t.Errorf("%s[%d] = %s, want %s", net, i, got[i], exp[i])
			}
		}
	}
}

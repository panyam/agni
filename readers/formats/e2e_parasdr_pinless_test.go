package formats

import (
	"slices"
	"testing"

	"github.com/panyam/agni/core/check"
)

// TestParasdrsMechanicalPartsHaveNoPins reads parasdr's board and both of its exports, and asks
// which parts have no pins. The labels ET1 and ET2 have no pin in any of the three files. The
// fiducials E97 to E99 have one mechanical pad each. The board stores that pad with no number, and
// IPC-2581 types it MECHANICAL. ODX types every pin U, so it cannot tell the pad from an unconnected
// pin, and the fiducials keep a pin there. Before agni issue 1437, neither export built a part type,
// so no part read as pinless and unconnected-component failed all five.
func TestParasdrsMechanicalPartsHaveNoPins(t *testing.T) {
	l := &Loader{}
	for file, want := range map[string][]string{
		"parasdr_layout.brd":  {"A1", "E97", "E98", "E99", "ET1", "ET2"},
		"parasdr_ipc2581.xml": {"E97", "E98", "E99", "ET1", "ET2"},
		"parasdr_odb++.xml":   {"ET1", "ET2"},
	} {
		d, err := l.ReadDesign(parasdr + file)
		if err != nil {
			t.Fatalf("%s: %v (run `make samples-oracle`)", file, err)
		}
		m := check.NewModel(d)
		var got []string
		for _, c := range d.GetComponents() {
			if m.IsPinlessPart(c.GetRefDes()) {
				got = append(got, c.GetRefDes())
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s: pinless parts %v, want %v", file, got, want)
		}
		// X3 has two mechanical pins beside its electrical ones, so it still has pins.
		if m.IsPinlessPart("X3") || !m.IsConnected("X3") {
			t.Errorf("%s: X3 reads as pinless or unconnected", file)
		}
	}
}

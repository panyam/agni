package symread

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// TestResolvePinsReportsUnresolved (WS1-013): ResolvePins counts placements whose symbol
// failed to resolve — the signal the readers gate dangling emission on. A resolved symbol
// contributes its pins; an unresolved one contributes none and increments the count.
func TestResolvePinsReportsUnresolved(t *testing.T) {
	place := func(px, py float64) (float64, float64) { return px, py }
	pls := []Placement{
		{Symref: "good.sym", Ref: "R1", Part: &ir.PartType{}, Place: place},
		{Symref: "missing.sym", Ref: "R2", Part: &ir.PartType{}, Place: place},
		{Symref: "good.sym", Ref: "R3", Part: &ir.PartType{}, Place: place}, // cached hit, still resolved
	}
	load := func(symref string) ([]Pin, bool) {
		if symref == "good.sym" {
			return []Pin{{X: 0, Y: 0, Number: "1"}, {X: 0, Y: 10, Number: "2"}}, true
		}
		return nil, false
	}
	quant := func(x, y float64) netgraph.Point { return netgraph.Point{X: int64(x), Y: int64(y)} }

	pins, unresolved := ResolvePins(pls, load, quant)
	if unresolved != 1 {
		t.Errorf("unresolved = %d, want 1 (only missing.sym)", unresolved)
	}
	if len(pins) != 4 { // R1 and R3 contribute 2 each; R2 none
		t.Errorf("pins = %d, want 4", len(pins))
	}
}

// TestResolvePinsSlotRemap (WS1-032): a Placement with SlotPins maps each drawn pin's number to
// its slot's physical package pin, indexed by the pin's Seq. Two slots of one symbol share the
// drawn geometry (numbers 1,2) but resolve to distinct package pins; a swapped slot row
// (slotdef=K:9,8) proves the remap follows Seq order, not draw order. A pin whose Seq falls
// outside the table keeps its drawn number.
func TestResolvePinsSlotRemap(t *testing.T) {
	place := func(px, py float64) (float64, float64) { return px, py }
	load := func(string) ([]Pin, bool) {
		return []Pin{{Number: "1", Seq: 1}, {Number: "2", Seq: 2}}, true
	}
	quant := func(x, y float64) netgraph.Point { return netgraph.Point{X: int64(x), Y: int64(y)} }
	pls := []Placement{
		{Symref: "g.sym", Ref: "U1", Part: &ir.PartType{}, Place: place, SlotPins: []string{"1", "2"}},
		{Symref: "g.sym", Ref: "U1", Part: &ir.PartType{}, Place: place, SlotPins: []string{"3", "4"}},
		{Symref: "g.sym", Ref: "U1", Part: &ir.PartType{}, Place: place, SlotPins: []string{"9", "8"}}, // swapped order
		{Symref: "g.sym", Ref: "U2", Part: &ir.PartType{}, Place: place},                              // no slotting -> drawn numbers
	}
	pins, _ := ResolvePins(pls, load, quant)
	var got []string
	for _, p := range pins {
		got = append(got, p.Pin)
	}
	want := []string{"1", "2", "3", "4", "9", "8", "1", "2"}
	if len(got) != len(want) {
		t.Fatalf("pin numbers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pin[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

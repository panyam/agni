package diff

import (
	"maps"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// place is a board placement whose pads are "number=net" pairs, "" for an unconnected pad.
func place(ref string, pads ...[2]string) *geom.ComponentPlacement {
	pl := &geom.ComponentPlacement{RefDes: ref}
	for _, p := range pads {
		pl.Pads = append(pl.Pads, &geom.Pad{Number: p[0], Net: p[1]})
	}
	return pl
}

func comps(refs ...string) []*ir.Component {
	var out []*ir.Component
	for _, r := range refs {
		out = append(out, &ir.Component{RefDes: r})
	}
	return out
}

// viewPair is a schematic and its board that disagree in each way Views reports.
func viewPair() (*ir.Design, *geom.BoardGeometry) {
	sch := &ir.Design{
		Components: comps("R1", "R2", "C1", "U1"),
		Nets: []*ir.Net{
			net("GND", "sch", "R1.2", "C1.2", "#PWR01.1"), // the power pin is no part: equal
			net("VIN", "sch", "R1.1", "R2.1"),             // the board names it Net-(R1-Pad1): renamed
			net("OUT", "sch", "R2.2", "C1.1"),             // the board leaves C1.1 off: hard
			net("EN", "sch", "U1.1"),                      // U1 is not on the board: deleted
			net("NC1", "sch", "C1.3"),                     // C1's pad 3 joins no net on the board: deleted
			net("FLAG", "sch", "#FLG01.1"),                // joins no real pin: left out
		},
	}
	board := &geom.BoardGeometry{Placements: []*geom.ComponentPlacement{
		place("R1", [2]string{"1", "Net-(R1-Pad1)"}, [2]string{"2", "GND"}),
		place("R2", [2]string{"1", "Net-(R1-Pad1)"}, [2]string{"2", "OUT"}),
		place("C1", [2]string{"1", "Net-(C1-Pad1)"}, [2]string{"2", "GND"}, [2]string{"3", ""}),
		place("H1", [2]string{"1", ""}), // a mounting hole only the board places
	}}
	return sch, board
}

func TestViewsCountsEachDisagreement(t *testing.T) {
	v := Views(viewPair())
	if want := (Tally{Matched: 3, Schematic: 4, Board: 4}); v.Parts != want {
		t.Errorf("parts = %+v, want %+v", v.Parts, want)
	}
	if !eq(v.Report.ComponentsRemoved, []string{"U1"}) || !eq(v.Report.ComponentsAdded, []string{"H1"}) {
		t.Errorf("schematic-only parts %v, board-only %v; want [U1] and [H1]", v.Report.ComponentsRemoved, v.Report.ComponentsAdded)
	}
	// Five schematic nets join a real pin. GND and VIN match, OUT's pins differ, and EN and NC1 are
	// on the schematic alone.
	if want := (Tally{Matched: 2, Schematic: 5, Board: 4}); v.Nets != want {
		t.Errorf("nets = %+v, want %+v", v.Nets, want)
	}
	kinds := map[string]NetChange{}
	for _, nc := range v.Report.Nets {
		kinds[nc.Name] = nc
	}
	if len(kinds) != 5 {
		t.Fatalf("net changes = %+v, want 5", v.Report.Nets)
	}
	for name, kind := range map[string]NetChangeKind{
		"Net-(R1-Pad1)": NetRenamed, "OUT": NetHard, "EN": NetDeleted, "NC1": NetDeleted, "Net-(C1-Pad1)": NetNew,
	} {
		if kinds[name].Kind != kind {
			t.Errorf("%s = %q, want %q", name, kinds[name].Kind, kind)
		}
	}
	if nc := kinds["OUT"]; !eq(nc.Removed, []string{"C1.1"}) || len(nc.Added) != 0 {
		t.Errorf("OUT on the board gained %v and lost %v, want it to lose C1.1 alone", nc.Added, nc.Removed)
	}
	// The pins on R1, R2 and C1. C1.3 is on a net only in the schematic.
	if want := (Tally{Matched: 6, Schematic: 7, Board: 6}); v.Pins != want {
		t.Errorf("pins = %+v, want %+v", v.Pins, want)
	}
	if !eq(v.SchematicOnlyPins, []string{"C1.3"}) || len(v.BoardOnlyPins) != 0 {
		t.Errorf("schematic-only pins %v, board-only %v; want [C1.3] and none", v.SchematicOnlyPins, v.BoardOnlyPins)
	}
	if len(v.Report.ComponentsChanged) != 0 {
		t.Errorf("component field changes = %+v, want none, since a board holds no part fields", v.Report.ComponentsChanged)
	}
}

func TestBoardRenamesMapsTheBoardNameToTheSchematics(t *testing.T) {
	got := Views(viewPair()).BoardRenames()
	if want := map[string]string{"Net-(R1-Pad1)": "VIN"}; !maps.Equal(got, want) {
		t.Errorf("BoardRenames = %v, want %v", got, want)
	}
}

func TestViewsOfAgreeingViewsMatchEverything(t *testing.T) {
	sch := &ir.Design{Components: comps("R1", "R2"), Nets: []*ir.Net{net("A", "s", "R1.1", "R2.1"), net("B", "s", "R1.2", "R2.2")}}
	board := &geom.BoardGeometry{Placements: []*geom.ComponentPlacement{
		place("R1", [2]string{"1", "A"}, [2]string{"2", "B"}),
		place("R2", [2]string{"1", "A"}, [2]string{"2", "B"}),
	}}
	v := Views(sch, board)
	if v.Parts != (Tally{2, 2, 2}) || v.Nets != (Tally{2, 2, 2}) || v.Pins != (Tally{4, 4, 4}) || len(v.Report.Nets) != 0 {
		t.Errorf("parts %+v, nets %+v, pins %+v, changes %+v; want everything matched", v.Parts, v.Nets, v.Pins, v.Report.Nets)
	}
}

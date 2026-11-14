package service

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/edif"
	"github.com/panyam/agni/readers/odb"
)

// locateFixture is a two-unit U1 drawn across two sheets, unit A (pins 1-3) on S1 beside R1 and
// unit B (pins 4-6) on S2, plus a board carrying both footprints, N1's copper and a GND pour.
func locateFixture() (*ir.Design, *geom.SchematicGeometry, *geom.BoardGeometry) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1"}, {RefDes: "R1"}},
		Nets: []*ir.Net{
			{Name: "N1", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "R1", PinRef: "1"}}},
			{Name: "N5", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "5"}, {ComponentRef: "R1", PinRef: "2"}}},
			{Name: "GND", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "6"}}},
		},
	}
	pins := func(ns ...string) []*geom.PinPoint {
		var out []*geom.PinPoint
		for _, n := range ns {
			out = append(out, &geom.PinPoint{PortRef: n})
		}
		return out
	}
	place := func(ref, cell, view string) *geom.SymbolPlacement {
		return &geom.SymbolPlacement{RefDes: ref, CellRef: cell, ViewRef: view, Transform: &geom.Transform{Origin: &geom.Point{}}}
	}
	wire := func(net string) *geom.WireGeometry {
		return &geom.WireGeometry{Net: net, Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}}}}
	}
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{
			{CellRef: "U", ViewRef: "A", Pins: pins("1", "2", "3")},
			{CellRef: "U", ViewRef: "B", Pins: pins("4", "5", "6")},
			{CellRef: "R", Pins: pins("1", "2")},
		},
		Sheets: []*geom.SheetGeometry{
			{Id: "S1", Placements: []*geom.SymbolPlacement{place("U1", "U", "A"), place("R1", "R", "")}, Wires: []*geom.WireGeometry{wire("N1")}},
			{Id: "S2", Placements: []*geom.SymbolPlacement{place("U1", "U", "B")}, Wires: []*geom.WireGeometry{wire("N5")}},
		},
	}
	pad := func(n, net string) *geom.Pad {
		return &geom.Pad{Number: n, Net: net, At: &geom.Point{}, Layers: []string{"F.Cu"}}
	}
	b := &geom.BoardGeometry{
		Placements: []*geom.ComponentPlacement{
			{RefDes: "U1", At: &geom.Point{}, Pads: []*geom.Pad{pad("1", "N1"), pad("5", "N5"), pad("6", "GND")}},
			{RefDes: "R1", At: &geom.Point{}, Pads: []*geom.Pad{pad("1", "N1"), pad("2", "N5")}},
		},
		Nets:  []*geom.NetCopper{{Net: "N1", Segments: []*geom.TrackSegment{{A: &geom.Point{}, B: &geom.Point{X: 1}, Layer: "F.Cu"}}}},
		Zones: []*geom.Zone{{Net: "GND", Layer: "B.Cu", Outline: &geom.Polyline{}}},
	}
	return d, g, b
}

func locate(t *testing.T, board bool, subjects ...*checkspb.Subject) []*webapi.SubjectSheets {
	t.Helper()
	d, g, b := locateFixture()
	svc := NewDesignService(fakeLoader{design: d, geom: g, board: b}, noNative{}, render.Style{}, nil)
	resp, err := svc.LocateSubjects(context.Background(), &webapi.LocateSubjectsRequest{
		Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch"), Subjects: subjects, Board: board,
	})
	if err != nil {
		t.Fatalf("LocateSubjects: %v", err)
	}
	if len(resp.GetSubjects()) != len(subjects) {
		t.Fatalf("answered %d subjects for %d asked", len(resp.GetSubjects()), len(subjects))
	}
	return resp.GetSubjects()
}

func part(ref string) *checkspb.Subject {
	return &checkspb.Subject{Kind: check.KindComponent, Ref: ref}
}
func pinOf(ref, pin string) *checkspb.Subject {
	return &checkspb.Subject{Kind: check.KindPin, Ref: ref, Pin: pin}
}
func netNamed(name string) *checkspb.Subject {
	return &checkspb.Subject{Kind: check.KindNet, Ref: name}
}

// A multi-unit part draws each unit on its own sheet, so a pin answers its unit's sheet and the part
// answers both.
func TestLocateSubjectsAnswersEachUnitsSheet(t *testing.T) {
	got := locate(t, false, part("U1"), pinOf("U1", "5"), pinOf("U1", "1"), pinOf("U1", "9"), netNamed("N5"))
	want := [][]string{{"S1", "S2"}, {"S2"}, {"S1"}, {"S1", "S2"}, {"S2"}}
	for i, w := range want {
		if !slicesEqual(got[i].GetSheetIds(), w) {
			t.Errorf("%v: sheets = %v, want %v", got[i].GetSubject(), got[i].GetSheetIds(), w)
		}
		if got[i].GetSubject().GetRef() == "" {
			t.Errorf("answer %d does not echo its subject", i)
		}
	}
}

// The board sheet joins only when asked, after the schematic's, by ref-des and net NAME. GND has a
// pour and no wire, so the board is its only sheet.
func TestLocateSubjectsAddsTheBoardWhenAsked(t *testing.T) {
	subjects := []*checkspb.Subject{part("U1"), pinOf("R1", "2"), netNamed("N1"), netNamed("GND"),
		{Kind: check.KindNet, Ref: "N1", NetId: "an-id-the-board-never-saw"}}
	want := [][]string{{"S1", "S2", "board"}, {"S1", "board"}, {"S1", "board"}, {"board"}, {"S1", "board"}}
	got := locate(t, true, subjects...)
	for i, w := range want {
		if !slicesEqual(got[i].GetSheetIds(), w) {
			t.Errorf("%v: sheets = %v, want %v", got[i].GetSubject(), got[i].GetSheetIds(), w)
		}
	}
	for _, s := range locate(t, false, subjects...) {
		for _, id := range s.GetSheetIds() {
			if id == boardSheetID {
				t.Errorf("%v answered the board without being asked", s.GetSubject())
			}
		}
	}
}

// A subject drawn nowhere says why, as a finding does.
func TestLocateSubjectsSaysWhyASubjectIsUndrawn(t *testing.T) {
	got := locate(t, true, part("U99"), netNamed("NOPE"))
	for _, s := range got {
		if len(s.GetSheetIds()) != 0 || s.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_NOT_IN_DESIGN {
			t.Errorf("%v: sheets %v reason %v, want none and NOT_IN_DESIGN", s.GetSubject(), s.GetSheetIds(), s.GetLocateReason())
		}
	}
	if r := locate(t, false, part("U1"))[0].GetLocateReason(); r != checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED {
		t.Errorf("a drawn part carries reason %v", r)
	}
}

func TestLocateSubjectsRefusesAnEmptyRequest(t *testing.T) {
	d, g, b := locateFixture()
	svc := NewDesignService(fakeLoader{design: d, geom: g, board: b}, noNative{}, render.Style{}, nil)
	_, err := svc.LocateSubjects(context.Background(), &webapi.LocateSubjectsRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch")})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("an empty request answered err %v, want invalid argument", err)
	}
}

// A design with no board answers the schematic alone when the board is asked for.
func TestLocateSubjectsWithoutABoard(t *testing.T) {
	d, g, _ := locateFixture()
	svc := NewDesignService(fakeLoader{design: d, geom: g}, noNative{}, render.Style{}, nil)
	resp, err := svc.LocateSubjects(context.Background(), &webapi.LocateSubjectsRequest{
		Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch"), Subjects: []*checkspb.Subject{part("R1")}, Board: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.GetSubjects()[0].GetSheetIds(); !slicesEqual(got, []string{"S1"}) {
		t.Errorf("sheets = %v, want [S1]", got)
	}
}

// A check run that read the board badges its findings and verdicts with the board sheet (#142's
// WS7-034), so a reader opens the copper a finding is about.
func TestCheckDesignBadgesTheBoard(t *testing.T) {
	d, g, b := locateFixture()
	svc := NewCheckService(fakeLoader{design: d, geom: g, board: b}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch")})
	if err != nil {
		t.Fatal(err)
	}
	var n1 int
	for _, v := range resp.GetVerdicts() {
		subs := v.GetSubjects()
		if len(subs) != 1 || subs[0].GetKind() != check.KindNet || subs[0].GetRef() != "N1" {
			continue
		}
		n1++
		if !slicesEqual(v.GetSheets(), []string{"S1", "board"}) {
			t.Errorf("verdict %s sheets = %v, want [S1 board]", v.GetId(), v.GetSheets())
		}
	}
	if n1 == 0 {
		t.Fatal("the run answered no verdict about N1, so the test checks nothing")
	}
	// The same run without a board badges no board.
	svc = NewCheckService(fakeLoader{design: d, geom: g}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err = svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch")})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range resp.GetVerdicts() {
		for _, id := range v.GetSheets() {
			if id == boardSheetID {
				t.Fatalf("a run with no board badged verdict %s with the board", v.GetId())
			}
		}
	}
}

// An EDIF netlist and an ODB++ job of the same board share no net id, so the board answers by
// ref-des and net name. Both mixer files are exports of one KiCad project.
func TestLocateSubjectsJoinsAnEDIFNetlistToAnODBJob(t *testing.T) {
	f, err := os.Open("../examples/common/designs/mixer.edn")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := edif.Read(f, "mixer.edn")
	if err != nil {
		t.Fatal(err)
	}
	b, err := odb.ReadBoardGeometry(os.DirFS("../readers/odb/testdata/mixer"), "matrix/matrix")
	if err != nil {
		t.Fatal(err)
	}
	var subjects []*checkspb.Subject
	for _, n := range d.GetNets() {
		subjects = append(subjects, netNamed(n.GetName()))
	}
	for _, c := range d.GetComponents() {
		subjects = append(subjects, part(c.GetRefDes()))
	}
	if len(subjects) < 4 {
		t.Fatalf("mixer.edn reads %d nets and parts, so the test checks almost nothing", len(subjects))
	}
	svc := NewDesignService(fakeLoader{design: d, board: b}, noNative{}, render.Style{}, nil)
	resp, err := svc.LocateSubjects(context.Background(), &webapi.LocateSubjectsRequest{
		Snapshot: snapOf(t, svc.Snapshots(), "mount://m/mixer.edn"), Subjects: subjects, Board: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range resp.GetSubjects() {
		if !slicesEqual(s.GetSheetIds(), []string{boardSheetID}) {
			t.Errorf("%v: sheets = %v, want [board]", s.GetSubject(), s.GetSheetIds())
		}
	}
}

// sideFixture is locateFixture's design on a board that places U1 on the back and R1 on the front,
// routes N1 on F.Cu and In2.Cu, and lists each pad's mask layer, as KiCad does.
func sideFixture() (*ir.Design, *geom.SchematicGeometry, *geom.BoardGeometry) {
	d, g, _ := locateFixture()
	pad := func(n, net string, layers ...string) *geom.Pad {
		return &geom.Pad{Number: n, Net: net, At: &geom.Point{}, Layers: layers}
	}
	b := &geom.BoardGeometry{
		Layers: []*geom.BoardLayer{{Name: "F.Cu", Kind: "signal"}, {Name: "In1.Cu", Kind: "signal"}, {Name: "In2.Cu", Kind: "signal"},
			{Name: "B.Cu", Kind: "signal"}, {Name: "F.Mask", Kind: "user"}, {Name: "B.Mask", Kind: "user"}},
		Placements: []*geom.ComponentPlacement{
			{RefDes: "U1", Layer: "B.Cu", Mirror: true, At: &geom.Point{},
				Pads: []*geom.Pad{pad("1", "N1", "B.Cu", "B.Mask"), pad("5", "N5", "*.Cu", "*.Mask"), pad("6", "GND", "B.Cu", "B.Mask")}},
			{RefDes: "R1", Layer: "F.Cu", At: &geom.Point{}, Pads: []*geom.Pad{pad("1", "N1", "F.Cu", "F.Mask"), pad("2", "N5", "*.Cu")}},
		},
		Nets: []*geom.NetCopper{
			{Net: "N1", Segments: []*geom.TrackSegment{
				{A: &geom.Point{}, B: &geom.Point{X: 1}, Layer: "In2.Cu"},
				{A: &geom.Point{}, B: &geom.Point{X: 1}, Layer: "F.Cu"}}},
			{Net: "N5", Vias: []*geom.Via{{At: &geom.Point{}, LayerFrom: "F.Cu", LayerTo: "B.Cu"}}},
		},
		Zones: []*geom.Zone{{Net: "GND", Layer: "In1.Cu", Outline: &geom.Polyline{}}},
	}
	return d, g, b
}

// A board view turns to a part's side and shows the copper a selection uses (agni issue 1587). A
// pin answers its part's side. A net answers every copper layer its tracks, zones and single-layer
// pads use, in the layer table's order, and leaves out the mask layers its pads also list. N5 runs
// through a via and two through-hole pads alone, so it names no layer.
func TestLocateSubjectsNamesTheBoardSideAndLayers(t *testing.T) {
	d, g, b := sideFixture()
	svc := NewDesignService(fakeLoader{design: d, geom: g, board: b}, noNative{}, render.Style{}, nil)
	subjects := []*checkspb.Subject{part("U1"), part("R1"), pinOf("U1", "6"), netNamed("N1"), netNamed("N5"), netNamed("GND")}
	type want struct {
		side   string
		layers []string
	}
	wants := []want{{"back", []string{"B.Cu"}}, {"front", []string{"F.Cu"}}, {"back", []string{"B.Cu"}},
		{"", []string{"F.Cu", "In2.Cu", "B.Cu"}}, {"", nil}, {"", []string{"In1.Cu", "B.Cu"}}}
	for _, board := range []bool{true, false} {
		resp, err := svc.LocateSubjects(context.Background(), &webapi.LocateSubjectsRequest{
			Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch"), Subjects: subjects, Board: board,
		})
		if err != nil {
			t.Fatalf("LocateSubjects: %v", err)
		}
		if len(resp.GetSubjects()) != len(wants) {
			t.Fatalf("answered %d subjects for %d asked", len(resp.GetSubjects()), len(wants))
		}
		for i, s := range resp.GetSubjects() {
			w := wants[i]
			if !board {
				w = want{}
			}
			if s.GetBoardSide() != w.side || !slicesEqual(s.GetBoardLayers(), w.layers) {
				t.Errorf("board=%v %v: side %q layers %v, want %q %v", board, s.GetSubject(), s.GetBoardSide(), s.GetBoardLayers(), w.side, w.layers)
			}
		}
	}
}

// A part whose board names neither outer layer and does not mirror it has no side, so a board view
// keeps the side it shows.
func TestBoardIndexLeavesAnUnstatedSideEmpty(t *testing.T) {
	ix := newBoardIndex(&geom.BoardGeometry{Placements: []*geom.ComponentPlacement{{RefDes: "J1", Layer: "TOP_SIDE", At: &geom.Point{}}}})
	if got := ix.sideOf(part("J1")); got != "" {
		t.Errorf("J1's side = %q, want none", got)
	}
	if got := ix.sideOf(part("J9")); got != "" {
		t.Errorf("an unplaced part's side = %q, want none", got)
	}
}

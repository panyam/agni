package formats

import (
	"os"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pinGroups is every connected pin as `REF.PIN~FIRST`, where FIRST is the first pin of its net in
// sort order. Two reads put a pin on the same net exactly when its row is in both, whatever each
// names the net, so a pin moved between nets shows as one row missing and one extra.
func pinGroups(nets map[string][]string) []string {
	var out []string
	for _, pins := range nets {
		sorted := append([]string(nil), pins...)
		sort.Strings(sorted)
		for _, p := range sorted {
			out = append(out, p+"~"+sorted[0])
		}
	}
	sort.Strings(out)
	return out
}

func designNets(d *ir.Design) map[string][]string {
	out := map[string][]string{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			out[n.GetName()] = append(out[n.GetName()], c.GetComponentRef()+"."+c.GetPinRef())
		}
	}
	return out
}

func pstxnetNets(rows []string) map[string][]string {
	out := map[string][]string{}
	for _, r := range rows {
		pin, net, _ := strings.Cut(r, "=")
		out[net] = append(out[net], pin)
	}
	return out
}

// TestOrCADReadsAsClaysNetlist reads Clay R6's Capture design, the top CLAYR6.DSN and the ten
// blocks it places, and holds it to pstxnet.dat, the netlist Capture handed Allegro. Every part is
// named by its occurrence designator, as the netlist names it, and every pin sits on the same net
// as the netlist puts it on, the net's name aside. The known differences are listed, and any other
// fails.
func TestOrCADReadsAsClaysNetlist(t *testing.T) {
	top := clayR6 + "CLAYR6.DSN"
	if _, err := os.Stat(top); err != nil {
		t.Fatalf("Clay R6's design is missing; run `make samples-oracle`: %v", err)
	}
	d, err := (&Loader{}).ReadDesign(top)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.GetAttributes()["orcad_format"]; got != "3.2" {
		t.Errorf("orcad_format %q, want 3.2", got)
	}
	if u := d.GetInputDiagnostics().GetUnexpandedHierarchy(); len(u) != 0 {
		t.Errorf("%d blocks were left out: %v", len(u), u)
	}

	want := readPstxnet(t, clayR6+"pstxnet.dat")
	wantRefs := map[string]bool{}
	for _, r := range want {
		wantRefs[r[:strings.IndexByte(r, '.')]] = true
	}
	gotRefs := map[string]bool{}
	for _, c := range d.GetComponents() {
		gotRefs[c.GetRefDes()] = true
	}
	if len(gotRefs) != 149 || len(wantRefs) != 149 {
		t.Fatalf("parts: read %d, netlist %d, want 149 each", len(gotRefs), len(wantRefs))
	}
	for r := range wantRefs {
		if !gotRefs[r] {
			t.Errorf("netlist part %s is not in the read", r)
		}
	}

	// Capture gathers every pin flagged not-connected onto one net named NC. The read leaves each
	// unconnected instead, since joining them would make a net the drawing never drew.
	notConnected := map[string]bool{
		"J7.7": true, "U2.7": true, "U2.19": true, "U2.21": true, "U7.6": true, "U7.9": true, "U7.10": true,
	}
	nets := pstxnetNets(want)
	nc := nets["NC"]
	sort.Strings(nc)
	if len(nc) != len(notConnected) {
		t.Errorf("the netlist's NC net holds %v, want the %d not-connected pins", nc, len(notConnected))
	}
	for _, p := range nc {
		if !notConnected[p] {
			t.Errorf("the netlist puts %s on NC, which the test does not expect", p)
		}
	}
	delete(nets, "NC")
	got := designNets(d)
	for _, pins := range got {
		for _, p := range pins {
			if notConnected[p] {
				t.Errorf("not-connected pin %s reads on a net", p)
			}
		}
	}

	wantRows, gotRows := pinGroups(nets), pinGroups(got)
	if len(wantRows) != 564 {
		t.Fatalf("the netlist puts %d pins on nets other than NC, want 564", len(wantRows))
	}
	if diffs := rowDiff(wantRows, gotRows); len(diffs) > 0 {
		t.Errorf("%d pins sit on other nets than the netlist puts them on:\n  %s", len(diffs), capLines(diffs))
	}
}

// rowDiff is the rows only one side has, the netlist's first.
func rowDiff(netlist, read []string) []string {
	in := func(rows []string) map[string]bool {
		m := map[string]bool{}
		for _, r := range rows {
			m[r] = true
		}
		return m
	}
	n, r := in(netlist), in(read)
	var out []string
	for _, row := range netlist {
		if !r[row] {
			out = append(out, "netlist only: "+row)
		}
	}
	for _, row := range read {
		if !n[row] {
			out = append(out, "read only: "+row)
		}
	}
	return out
}

// TestOrCADJoinsClaysBoard holds Clay R6's Capture design to CLAYR6.brd, the board Allegro saved
// from it, by designator, the join a cross-probe makes. Every part the board places is a part of the
// schematic, and every pin both carry sits on the same net in each, save the ones Capture's netlist
// moved after the board was saved.
func TestOrCADJoinsClaysBoard(t *testing.T) {
	l := &Loader{}
	sch, err := l.ReadDesign(clayR6 + "CLAYR6.DSN")
	if err != nil {
		t.Fatal(err)
	}
	brd, err := l.ReadDesign(clayR6 + "CLAYR6.brd")
	if err != nil {
		t.Fatal(err)
	}
	schRefs := map[string]bool{}
	for _, c := range sch.GetComponents() {
		schRefs[c.GetRefDes()] = true
	}
	if len(brd.GetComponents()) != 146 {
		t.Fatalf("the board places %d parts, want 146", len(brd.GetComponents()))
	}
	for _, c := range brd.GetComponents() {
		if !schRefs[c.GetRefDes()] {
			t.Errorf("board part %s is not in the schematic", c.GetRefDes())
		}
	}

	schNets, brdNets := designNets(sch), designNets(brd)
	onBoth := map[string]bool{}
	for _, pins := range brdNets {
		for _, p := range pins {
			onBoth[p] = true
		}
	}
	keep := func(nets map[string][]string, in map[string]bool) map[string][]string {
		out := map[string][]string{}
		for n, pins := range nets {
			for _, p := range pins {
				if in[p] {
					out[n] = append(out[n], p)
				}
			}
		}
		return out
	}
	inSch := map[string]bool{}
	for _, pins := range schNets {
		for _, p := range pins {
			inSch[p] = true
		}
	}
	both := map[string]bool{}
	for p := range onBoth {
		if inSch[p] {
			both[p] = true
		}
	}
	if len(both) != 558 {
		t.Errorf("%d pins are on a net in both reads, want 558", len(both))
	}
	// Capture's netlist moved four of U5's pins onto the buzzer after Allegro saved the board, as
	// TestAllegro166ReadsAsClaysNetlist lists, so the schematic and the board disagree on those and
	// on nothing else.
	moved := map[string]bool{"U5.71": true, "U5.72": true, "U5.94": true, "U5.97": true}
	var diffs []string
	seen := map[string]bool{}
	for _, d := range rowDiff(pinGroups(keep(schNets, both)), pinGroups(keep(brdNets, both))) {
		pin := d[strings.Index(d, ": ")+2 : strings.IndexByte(d, '~')]
		if moved[pin] {
			seen[pin] = true
		} else {
			diffs = append(diffs, d)
		}
	}
	if len(seen) != len(moved) {
		t.Errorf("%d of the %d moved pins differ, so the list above is stale", len(seen), len(moved))
	}
	if len(diffs) > 0 {
		t.Errorf("%d pins sit on other nets on the board than in the schematic:\n  %s", len(diffs), capLines(diffs))
	}
}

// TestABlockResolvesWhateverCaseItsPathIsSpelledIn holds the block opener to Capture's paths: a
// Windows separator, a leading ".", and a file spelled in another case than the design records it.
// Clay R6 names `.\SOFT_PWR_SW\SOFT_PWR_SW.dsn` for a file spelled .DSN.
func TestABlockResolvesWhateverCaseItsPathIsSpelledIn(t *testing.T) {
	l := &Loader{FS: fstest.MapFS{
		"design/TOP.DSN":                     {Data: []byte("top")},
		"design/SOFT_PWR_SW/SOFT_PWR_SW.DSN": {Data: []byte("block")},
	}}
	name, b, err := l.orcadOpener()("design/TOP.DSN", `.\SOFT_PWR_SW\SOFT_PWR_SW.dsn`)
	if err != nil || string(b) != "block" || name != "design/SOFT_PWR_SW/SOFT_PWR_SW.DSN" {
		t.Errorf("opened %q (%q, %v), want the block's file as the directory spells it", name, b, err)
	}
	if _, _, err := l.orcadOpener()("design/TOP.DSN", `E:\git\clay\Circuit\R6\SOFT_PWR_SW\SOFT_PWR_SW.DSN`); err == nil {
		t.Error("an absolute path from the machine that saved the design was followed")
	}
	if _, _, err := l.orcadOpener()("design/TOP.DSN", `.\MISSING\MISSING.DSN`); err == nil {
		t.Error("a block file that is not there opened")
	}
}

// TestASPECCTRADesignIsRefusedAsOne shares .dsn with Capture and is plain text.
func TestASPECCTRADesignIsRefusedAsOne(t *testing.T) {
	l := &Loader{FS: fstest.MapFS{"board.dsn": {Data: []byte("(pcb board.dsn\n  (parser (host_cad KiCad))\n)\n")}}}
	if _, err := l.ReadDesign("board.dsn"); err == nil || !strings.Contains(err.Error(), "SPECCTRA") {
		t.Errorf("err %v, want a SPECCTRA design refused by name", err)
	}
}

// TestOrCADDrawsEveryClayPageAndJoinsItsNetlist reads Clay R6's pages and holds them to its
// netlist by name, the joins a viewer makes when it locates a part or a net. Each of the eleven
// pages is a sheet, each block's under the top page. Every part draws once under the designator the
// netlist gives it, no placement is left undrawn, and every wire that reaches a pin, power symbol
// or port names a net the netlist has.
func TestOrCADDrawsEveryClayPageAndJoinsItsNetlist(t *testing.T) {
	l := &Loader{}
	top := clayR6 + "CLAYR6.DSN"
	d, err := l.ReadDesign(top)
	if err != nil {
		t.Fatal(err)
	}
	g, err := l.FaithfulGeometry(top)
	if err != nil {
		t.Fatal(err)
	}
	if g.GetDesignRef() != d.GetName() || g.GetUnitNm() != 254000 {
		t.Errorf("geometry for %q in %d nm units, want %q in Capture's 254000", g.GetDesignRef(), g.GetUnitNm(), d.GetName())
	}
	sheets := g.GetSheets()
	if len(sheets) != 11 {
		t.Fatalf("%d sheets, want Clay's top page and its ten blocks", len(sheets))
	}
	// A sheet's id is its block path, as KiCad's are, and its name the path's last part.
	root := sheets[0].GetId()
	if root != "/PAGE1" || sheets[0].GetName() != "PAGE1" {
		t.Errorf("the top page is %q named %q, want /PAGE1 named PAGE1", root, sheets[0].GetName())
	}
	ids := map[string]string{}
	for _, s := range sheets[1:] {
		ids[s.GetId()] = s.GetName()
		if s.GetParentId() != root {
			t.Errorf("sheet %q has parent %q, want the top page %q", s.GetName(), s.GetParentId(), root)
		}
	}
	if ids["/IMU"] != "IMU" || ids["/uC"] != "uC" {
		t.Errorf("block sheets %v, want /IMU named IMU and /uC named uC among them", ids)
	}
	if u := g.GetUndrawn(); len(u) != 0 {
		t.Errorf("%d placements did not draw: %v", len(u), u)
	}

	parts := map[string]bool{}
	for _, c := range d.GetComponents() {
		parts[c.GetRefDes()] = true
	}
	nets := map[string]bool{}
	for _, n := range d.GetNets() {
		nets[n.GetName()] = true
	}
	drawn := map[string]int{}
	named, anchored := 0, 0
	for _, s := range sheets {
		for _, p := range s.GetPlacements() {
			if p.GetRefDes() == "" {
				// A power symbol or port names the net it touches, and that net must be the netlist's.
				if a := p.GetNetAnchor(); a != "" {
					anchored++
					if !nets[a] {
						t.Errorf("sheet %q anchors a symbol to %q, which the netlist lacks", s.GetName(), a)
					}
				}
				continue
			}
			drawn[p.GetRefDes()]++
			if !parts[p.GetRefDes()] {
				t.Errorf("sheet %q draws %s, which the netlist lacks", s.GetName(), p.GetRefDes())
			}
		}
		for _, w := range s.GetWires() {
			if w.GetNet() == "" {
				continue
			}
			named++
			if !nets[w.GetNet()] {
				t.Errorf("sheet %q draws a wire on %q, which the netlist lacks", s.GetName(), w.GetNet())
			}
		}
	}
	if len(drawn) != len(parts) || len(drawn) != 149 {
		t.Errorf("%d parts drawn of the netlist's %d, want all 149", len(drawn), len(parts))
	}
	for r, n := range drawn {
		if n != 1 {
			t.Errorf("%s draws %d times, want once", r, n)
		}
	}
	// A resistor shows neither its pins' names nor their numbers, and the microcontroller shows both,
	// as each part's cached flags say.
	shows := map[string][2]bool{}
	for _, sd := range g.GetSymbols() {
		if len(sd.GetPins()) > 0 {
			p := sd.GetPins()[0]
			shows[strings.SplitN(sd.GetCellRef(), "|", 2)[0]] = [2]bool{p.GetLabelOrigin() != nil, p.GetNumberOrigin() != nil}
		}
	}
	if got := shows["RESISTOR.Normal"]; got != [2]bool{false, false} {
		t.Errorf("a resistor's pin shows name and number %v, want neither", got)
	}
	if got := shows["MK64FN1M0VLL12.Normal"]; got != [2]bool{true, true} {
		t.Errorf("the microcontroller's pin shows name and number %v, want both", got)
	}
	if named < 150 || anchored < 200 {
		t.Errorf("%d wires and %d symbols name a net, want most of Clay's", named, anchored)
	}
}

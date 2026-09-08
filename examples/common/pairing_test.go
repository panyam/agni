package common

import (
	"sort"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// The i2c-sensor fixture is one design in two files, and they are hand-authored separately. The
// netlist is what every analysis reads and the schematic is what a finding gets drawn on, and they
// are joined BY NAME: a highlight for net SDA lands on the wire the .eds calls SDA and nowhere else.
// Nothing in the engine holds the two together, so a ref-des renamed in one and not the other
// produces a drawing that is silently pointing at the wrong part, which is exactly the failure a
// reader trusts the picture not to have.

func TestTheDrawnSheetCoversEveryPartInTheNetlist(t *testing.T) {
	d, err := ReadFixture("i2c-sensor.edn")
	if err != nil {
		t.Fatalf("netlist: %v", err)
	}
	g, err := ReadSchematicFixture("i2c-sensor.eds")
	if err != nil {
		t.Fatalf("schematic: %v", err)
	}
	drawn := map[string]bool{}
	for _, s := range g.GetSheets() {
		for _, p := range s.GetPlacements() {
			drawn[p.GetRefDes()] = true
		}
	}
	var missing []string
	for _, c := range d.GetComponents() {
		if !drawn[c.GetRefDes()] {
			missing = append(missing, c.GetRefDes())
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("in the netlist and not on the sheet: %v", missing)
	}
	if len(drawn) != len(d.GetComponents()) {
		t.Errorf("sheet draws %d parts, netlist has %d; a part drawn that is not in the netlist "+
			"points somewhere the analysis never looked", len(drawn), len(d.GetComponents()))
	}
}

// A wire's name is the join key a finding's highlight resolves through, so a net the netlist has and
// the sheet spells differently is a highlight that silently lands nowhere.
func TestEveryNetlistNetIsDrawnUnderItsOwnName(t *testing.T) {
	d, err := ReadFixture("i2c-sensor.edn")
	if err != nil {
		t.Fatalf("netlist: %v", err)
	}
	g, err := ReadSchematicFixture("i2c-sensor.eds")
	if err != nil {
		t.Fatalf("schematic: %v", err)
	}
	drawn := map[string]bool{}
	for _, s := range g.GetSheets() {
		for _, w := range s.GetWires() {
			drawn[w.GetNet()] = true
		}
	}
	var missing []string
	for _, n := range d.GetNets() {
		if !drawn[n.GetName()] {
			missing = append(missing, n.GetName())
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("nets with no wire of that name on the sheet: %v", missing)
	}
}

// A wire may not run over a pin that belongs to a different net.
//
// This is the failure the other two guards cannot see and the one a reader is least able to defend
// against, because the drawing looks deliberate. The first version of this schematic ran the VCC rail
// along y=1000, which is exactly where R1 sits, so the rail passed through the resistor's body and
// touched its far pin. That pin is on SDA, so the picture showed VCC shorted to SDA while the netlist
// underneath said nothing of the kind, and every highlight drawn on it inherited the lie.
//
// Names agreeing is not enough for a drawing to be true. Where the wires RUN has to agree too.
func TestNoWireRunsOverAPinOfAnotherNet(t *testing.T) {
	d, err := ReadFixture("i2c-sensor.edn")
	if err != nil {
		t.Fatalf("netlist: %v", err)
	}
	g, err := ReadSchematicFixture("i2c-sensor.eds")
	if err != nil {
		t.Fatalf("schematic: %v", err)
	}
	// netOf maps "REF.PIN" to the net the NETLIST puts it on, which is the truth the drawing owes.
	netOf := map[string]string{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			netOf[c.GetComponentRef()+"."+c.GetPinRef()] = n.GetName()
		}
	}
	byCell := map[string]*geom.SymbolDef{}
	for _, s := range g.GetSymbols() {
		byCell[s.GetCellRef()] = s
	}
	for _, sh := range g.GetSheets() {
		type pinAt struct{ key string }
		pins := map[[2]int64]pinAt{}
		for _, p := range sh.GetPlacements() {
			tf := p.GetTransform()
			// Rotation would make "origin plus local" wrong, and this fixture uses none. Fail rather
			// than compute a position that is quietly off.
			if tf.GetRotationDeg() != 0 || tf.GetMirrorX() || tf.GetMirrorY() {
				t.Fatalf("%s is rotated or mirrored; this check assumes neither", p.GetRefDes())
			}
			sym := byCell[p.GetCellRef()]
			if sym == nil {
				t.Fatalf("%s references symbol %q, which the library does not hold", p.GetRefDes(), p.GetCellRef())
			}
			for _, pin := range sym.GetPins() {
				at := [2]int64{tf.GetOrigin().GetX() + pin.GetLoc().GetX(), tf.GetOrigin().GetY() + pin.GetLoc().GetY()}
				pins[at] = pinAt{key: p.GetRefDes() + "." + pin.GetPortRef()}
			}
		}
		for _, w := range sh.GetWires() {
			for _, pl := range w.GetPolylines() {
				pts := pl.GetPoints()
				for i := 0; i+1 < len(pts); i++ {
					a, b := pts[i], pts[i+1]
					for at, pin := range pins {
						if !onSegment(at, a.GetX(), a.GetY(), b.GetX(), b.GetY()) {
							continue
						}
						if want := netOf[pin.key]; want != w.GetNet() {
							t.Errorf("wire %s runs over pin %s at (%d,%d), which the netlist puts on %s",
								w.GetNet(), pin.key, at[0], at[1], want)
						}
					}
				}
			}
		}
	}
}

// onSegment reports whether p lies on the segment a->b, endpoints included. Integer arithmetic on a
// 10nm grid, so exact rather than tolerant: a wire drawn to meet a pin meets it exactly, and a near
// miss is a drawing bug worth seeing rather than rounding away.
func onSegment(p [2]int64, ax, ay, bx, by int64) bool {
	cross := (p[0]-ax)*(by-ay) - (p[1]-ay)*(bx-ax)
	if cross != 0 {
		return false
	}
	return p[0] >= min64(ax, bx) && p[0] <= max64(ax, bx) &&
		p[1] >= min64(ay, by) && p[1] <= max64(ay, by)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

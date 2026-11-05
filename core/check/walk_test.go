package check

import (
	"errors"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// walkFixture is an I2C bus with a pull-up, a sensor and a series test path:
//
//	R1:  SDA -> +3V3 (the pull-up, onto a rail)
//	U1:  the MCU, on SDA, SCL, +3V3 and GND
//	U2:  the sensor, on SDA, SCL, +3V3, GND and INT
//	R2:  SDA -> SDA_TP, where TP1 sits and C2 runs to GND
//	C1:  +3V3 -> GND (decoupling, beyond a rail, so never reached)
//	J1:  a connector on SDA and GND
//	R9:  SDA -> SPARE, not fitted
func walkFixture() *ir.Design {
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	r9 := comp("R9")
	r9.Attributes = map[string]string{"dnp": "yes"}
	return &ir.Design{
		Components: []*ir.Component{comp("R1"), comp("U1"), comp("U2"), comp("R2"), comp("TP1"), comp("C1"), comp("C2"), comp("J1"), r9},
		Nets: []*ir.Net{
			tnet("SDA", "R1.1", "U1.3", "U2.4", "R2.1", "J1.2", "R9.1"),
			tnet("SCL", "U1.4", "U2.5"),
			tnet("+3V3", "R1.2", "U1.1", "U2.1", "C1.1"),
			tnet("GND", "U1.2", "U2.2", "C1.2", "C2.2", "J1.1"),
			tnet("INT", "U2.3"),
			tnet("SDA_TP", "R2.2", "TP1.1", "C2.1"),
			tnet("SPARE", "R9.2"),
		},
	}
}

func walkOf(t *testing.T, from Endpoint, kind string, hops int, through ...string) Walk {
	t.Helper()
	w, err := WalkFrom(NewModel(walkFixture()), from, kind, hops, through)
	if err != nil {
		t.Fatal(err)
	}
	if w.Reason != "" {
		t.Fatalf("the walk did not start: %s", w.Reason)
	}
	return w
}

// hopNets renders a walk's nets as "hop:net" in answer order, with ended ones marked.
func hopNets(w Walk) string {
	var out []string
	for _, n := range w.Nets {
		s := string(rune('0'+n.Hop)) + ":" + n.Net.Name
		if n.Ended != "" {
			s += "(" + n.Ended + ")"
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

func boundaryOf(w Walk) map[string]string {
	out := map[string]string{}
	for _, b := range w.Boundary {
		out[b.RefDes] = b.Reason
	}
	return out
}

func TestWalkFromAPartStartsOnItsOwnNetsAndEndsAtTheRail(t *testing.T) {
	w := walkOf(t, Endpoint{RefDes: "R1"}, "small-parts", 1)
	if got, want := hopNets(w), "1:SDA 1:+3V3(rail)"; got != want {
		t.Errorf("hop 1 = %s, want %s", got, want)
	}
	for _, n := range w.Nets {
		if n.Entered.RefDes != "R1" || n.Entered.FromNet != "" || n.Entered.ExitPin == "" {
			t.Errorf("%s was entered by %+v, want R1's own pin", n.Net.Name, n.Entered)
		}
	}
}

func TestSmallPartsWalkCrossesPassivesAndStopsAtICsAndConnectors(t *testing.T) {
	w := walkOf(t, Endpoint{RefDes: "R1"}, "small-parts", 3)
	// U1 has exactly four connected pins, so the rule crosses it into SCL and GND. R2 leads to SDA_TP.
	if got, want := hopNets(w), "1:SDA 1:+3V3(rail) 2:SCL 2:GND(ground) 2:SDA_TP"; got != want {
		t.Errorf("nets = %s, want %s", got, want)
	}
	b := boundaryOf(w)
	if b["U2"] != "5 connected pins" || b["J1"] != "a connector" || b["R9"] != "not fitted" {
		t.Errorf("boundary = %v, want U2 for its pins, J1 as a connector and R9 as not fitted", b)
	}
	if _, ok := b["U1"]; ok {
		t.Errorf("U1 has four pins and sits on the boundary: %v", b)
	}
	// C2 crosses from SDA_TP into GND, which U1 reached first. It leaves GND's list of other parts
	// all the same, and C1, which nothing crossed, stays on it.
	for _, n := range w.Nets {
		if n.Net.Name != "GND" {
			continue
		}
		var on []string
		for _, st := range n.Net.Stubs {
			on = append(on, st.RefDes)
		}
		if got := strings.Join(on, " "); strings.Contains(got, "C2") || !strings.Contains(got, "C1") {
			t.Errorf("GND lists %q, want C1 and not C2", got)
		}
		if len(n.AlsoEntered) != 1 || n.AlsoEntered[0].RefDes != "C2" || n.AlsoEntered[0].FromNet != "SDA_TP" {
			t.Errorf("GND's further ways in = %+v, want C2 from SDA_TP", n.AlsoEntered)
		}
	}
}

func TestSeriesWalkCrossesWhatTraceCrosses(t *testing.T) {
	m := NewModel(walkFixture())
	w, err := WalkFrom(m, Endpoint{RefDes: "U2", Pin: "4"}, "series", 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hopNets(w), "1:SDA 2:+3V3 2:SDA_TP"; got != want {
		t.Errorf("nets = %s, want %s", got, want)
	}
	// Every net the series walk reaches, trace routes to from the same pin.
	for _, n := range w.Nets {
		var to Endpoint
		for _, c := range m.Nets() {
			if c.Name == n.Net.Name {
				to = Endpoint{RefDes: c.Connections[0].ComponentRef, Pin: c.Connections[0].PinRef}
			}
		}
		if tr := TracePins(m, Endpoint{RefDes: "U2", Pin: "4"}, to, 3); tr.Outcome != TraceRouted {
			t.Errorf("the series walk reached %s, and trace answers %s: %s", n.Net.Name, tr.Outcome, tr.Reason)
		}
	}
	b := boundaryOf(w)
	if b["U1"] != "an ic is not a series part" || b["C2"] != "a capacitor is not a series part" || b["R9"] != "not fitted" {
		t.Errorf("boundary = %v, want U1 and C2 refused as not series parts and R9 as not fitted", b)
	}
}

func TestWalkThroughCrossesANamedPart(t *testing.T) {
	w := walkOf(t, Endpoint{RefDes: "R1"}, "small-parts", 2, "U2")
	if !strings.Contains(hopNets(w), "2:INT") {
		t.Errorf("naming U2 in through did not cross it into INT: %s", hopNets(w))
	}
	if _, refused := boundaryOf(w)["U2"]; refused {
		t.Errorf("U2 is still on the boundary after naming it: %v", w.Boundary)
	}
}

func TestEveryPartWalkCountsWhatItsCapLeftOut(t *testing.T) {
	d := walkFixture()
	big := tnet("SDA", "R1.1")
	d.Components = append(d.Components, &ir.Component{RefDes: "U9", Prov: &ir.Provenance{SourceFile: "t"}})
	for i := 0; i < walkHopNetCap+5; i++ {
		name := "N" + string(rune('A'+i/26)) + string(rune('A'+i%26))
		d.Nets = append(d.Nets, tnet(name, "U9."+name))
	}
	big.Connections = append(big.Connections, &ir.Connection{ComponentRef: "U9", PinRef: "S"})
	d.Nets[0] = big
	w, err := WalkFrom(NewModel(d), Endpoint{RefDes: "R1", Pin: "1"}, "all", 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Elided) != 1 || w.Elided[0].Hop != 2 || w.Elided[0].Nets != 5 {
		t.Errorf("elided = %+v, want hop 2 leaving out 5 nets", w.Elided)
	}
	two := 0
	for _, n := range w.Nets {
		if n.Hop == 2 {
			two++
		}
	}
	if two != walkHopNetCap {
		t.Errorf("hop 2 lists %d nets, want the cap of %d", two, walkHopNetCap)
	}
}

func TestWalkStartThatNamesNothingSaysSo(t *testing.T) {
	m := NewModel(walkFixture())
	for _, from := range []Endpoint{{RefDes: "R77"}, {RefDes: "R1", Pin: "9"}} {
		w, err := WalkFrom(m, from, "series", 2, nil)
		if err != nil {
			t.Fatal(err)
		}
		if w.Reason == "" || len(w.Nets) != 0 {
			t.Errorf("%v: reason %q with %d nets, want a reason and nothing walked", from, w.Reason, len(w.Nets))
		}
	}
	if _, err := WalkFrom(m, Endpoint{RefDes: "R1"}, "sideways", 2, nil); !errors.Is(err, ErrUnknownWalkKind) {
		t.Errorf("err = %v, want ErrUnknownWalkKind", err)
	}
}

func TestWalkKindsAreListedWithASummary(t *testing.T) {
	ks := WalkKinds()
	if len(ks) != 3 {
		t.Fatalf("kinds = %+v, want three", ks)
	}
	for _, k := range ks {
		if k.Name == "" || k.Title == "" || k.Summary == "" {
			t.Errorf("kind %+v lacks a name, title or summary", k)
		}
	}
}

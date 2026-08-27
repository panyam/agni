package intake

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// fixtureDesign: a hand-authored design that classifies by ref-des prefix (no reader needed). It carries
// a deliberately DISTINCTIVE rail net name so the sanitization test can prove it never reaches the output.
func fixtureDesign() *ir.Design {
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	conn := func(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }
	return &ir.Design{
		Components: []*ir.Component{
			comp("R1"), comp("C1"), comp("D1"), comp("TP1"), comp("TP2"), comp("U1"),
		},
		Nets: []*ir.Net{
			{Name: "3V3_ZZSECRETRAIL", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{conn("R1", "1"), conn("U1", "1")}},
			{Name: "GND", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{conn("R1", "2"), conn("C1", "2")}},
		},
	}
}

// TestBuildClassCensus pins the deterministic counts: each component classifies by its ref-des prefix,
// and the class census is exact (a diode family tag counts the diode too).
func TestBuildClassCensus(t *testing.T) {
	s := Build(check.NewModel(fixtureDesign()))
	if s.Components != 6 {
		t.Fatalf("components = %d, want 6", s.Components)
	}
	want := map[string]int{"resistor": 1, "capacitor": 1, "diode": 1, "test_point": 2, "ic": 1}
	for cls, n := range want {
		if s.ClassCount[cls] != n {
			t.Errorf("class %q = %d, want %d (census: %+v)", cls, s.ClassCount[cls], n, s.ClassCount)
		}
	}
	if s.Nets != 2 {
		t.Errorf("nets = %d, want 2", s.Nets)
	}
}

// TestBuildRailNominals: the 3V3-named rail resolves to a 3.3 nominal (GND is not a rail nominal).
func TestBuildRailNominals(t *testing.T) {
	s := Build(check.NewModel(fixtureDesign()))
	if len(s.RailNominals) != 1 || s.RailNominals[0] != 3.3 {
		t.Fatalf("rail nominals = %v, want [3.3]", s.RailNominals)
	}
}

// A SIGNAL net whose name encodes a level must not be summarized as a rail voltage (agni issue
// 194). Skeleton.RailNominals reads net.nominal_voltage, which used to project any net whose name
// token-scanned to a voltage, so a house convention naming signals `<from>_<to>_<level>` inflated
// the rail set of the sanitized summary. This is the intake-side consequence of that overload, and
// it is the one that reached a published artifact rather than only a query.
func TestSignalLevelIsNotSummarizedAsARail(t *testing.T) {
	d := fixtureDesign()
	d.Nets = append(d.Nets, &ir.Net{
		Name: "U3_12_U7_4_1V8", Prov: &ir.Provenance{SourceFile: "t"},
		Connections: []*ir.Connection{
			{ComponentRef: "U1", PinRef: "12"}, {ComponentRef: "D1", PinRef: "4"}},
	})

	s := Build(check.NewModel(d))
	for _, v := range s.RailNominals {
		if v == 1.8 {
			t.Fatalf("a signal net's level leaked into the rail summary: %v", s.RailNominals)
		}
	}
	if len(s.RailNominals) != 1 || s.RailNominals[0] != 3.3 {
		t.Fatalf("the real rail must survive unchanged, got %v", s.RailNominals)
	}
}

// TestSanitizationNoNetNames is the load-bearing guarantee (WS3-091): a distinctive rail NET NAME in the
// design must never appear in EITHER rendered form, while its nominal (3.3) must — proving the Skeleton
// carries the voltage, not the name. Guards against a future field that would leak topology.
func TestSanitizationNoNetNames(t *testing.T) {
	s := Build(check.NewModel(fixtureDesign()))
	js, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	// Both the type-BOM (default) and the full per-component view, plus json, must be net-name-free.
	for _, out := range []string{Markdown(s, false), Markdown(s, true), string(js)} {
		if strings.Contains(out, "ZZSECRETRAIL") {
			t.Errorf("net name leaked into intake output: %s", out)
		}
		if !strings.Contains(out, "3.3") {
			t.Errorf("rail nominal 3.3 missing from output: %s", out)
		}
		if strings.Contains(out, "GND") {
			t.Errorf("ground net name leaked into output: %s", out)
		}
	}
}

// TestMarkdownAnomalies: an anomaly renders as kind + count + ref-des; the type has no net-name field,
// so a pin-net conflict can only name the component, never the conflicting nets.
func TestMarkdownAnomalies(t *testing.T) {
	md := Markdown(&Skeleton{
		ClassCount: map[string]int{},
		Anomalies:  []Anomaly{{Kind: "pin_net_conflict", Count: 2, RefDes: []string{"U7"}}},
	}, false)
	if !strings.Contains(md, "pin_net_conflict: 2 (U7)") {
		t.Errorf("anomaly line missing/wrong:\n%s", md)
	}
}

// TestBuildPartTypes: the BOM collapses per-component rows by (mpn, mfr, value, class), and a
// manufacturer spelling variant ("Murata" vs "MURATA") stays a SEPARATE row — the AVL-hygiene signal.
func TestBuildPartTypes(t *testing.T) {
	cap := func(ref, mfr string) *ir.Component {
		return &ir.Component{RefDes: ref, Attributes: map[string]string{"Manufacturer": mfr}, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	d := &ir.Design{Components: []*ir.Component{
		cap("C1", "Murata"), cap("C2", "Murata"), cap("C3", "Murata"), cap("C4", "MURATA"),
	}}
	s := Build(check.NewModel(d))
	if len(s.PartTypes) != 2 {
		t.Fatalf("part types = %d, want 2 (Murata x3 + MURATA x1): %+v", len(s.PartTypes), s.PartTypes)
	}
	if s.PartTypes[0].Count != 3 || s.PartTypes[0].Manufacturer != "Murata" || s.PartTypes[0].Class != "capacitor" {
		t.Errorf("top type = %+v, want {count:3 Murata capacitor}", s.PartTypes[0])
	}
	if s.PartTypes[1].Count != 1 || s.PartTypes[1].Manufacturer != "MURATA" {
		t.Errorf("drift type = %+v, want {count:1 MURATA}", s.PartTypes[1])
	}
}

// mpnDesign carries the same MPN on several components, which is the shape the gap queue got wrong:
// seeding is per part number, so two placements of one jellybean are one piece of work.
func mpnDesign() *ir.Design {
	comp := func(ref, mpn string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Mpn: mpn}
	}
	return &ir.Design{Components: []*ir.Component{
		comp("C1", "ACME-CAP-100N"), comp("C2", "ACME-CAP-100N"), comp("C3", "ACME-CAP-100N"),
		comp("R1", "ACME-RES-10K"), comp("R2", "ACME-RES-10K"),
		comp("U1", "ACME-MCU-G1"),
	}}
}

// emptyCorpus is a provider that seeds nothing, so every MPN on the board is a gap.
var emptyCorpus = param.ProviderFunc(func(string) *parampb.PartSpec { return nil })

func TestDatasheetGapsAreDistinctMPNs(t *testing.T) {
	s := Build(check.NewModelWithParams(mpnDesign(), nil, emptyCorpus))
	want := []string{"ACME-CAP-100N", "ACME-MCU-G1", "ACME-RES-10K"}
	if len(s.DatasheetGaps) != len(want) {
		t.Fatalf("gaps = %v (%d), want %d distinct MPNs; six components carry three part numbers and seeding is per part number", s.DatasheetGaps, len(s.DatasheetGaps), len(want))
	}
	for i, w := range want {
		if s.DatasheetGaps[i] != w {
			t.Errorf("gaps[%d] = %q, want %q (sorted, so the queue is diffable across runs)", i, s.DatasheetGaps[i], w)
		}
	}
}

// TestDatasheetGapsAreStablyOrdered guards the set the dedupe introduced: Go randomizes map iteration,
// so an unsorted queue would differ between runs and no two reports could be compared.
func TestDatasheetGapsAreStablyOrdered(t *testing.T) {
	first := Build(check.NewModelWithParams(mpnDesign(), nil, emptyCorpus)).DatasheetGaps
	for i := 0; i < 20; i++ {
		got := Build(check.NewModelWithParams(mpnDesign(), nil, emptyCorpus)).DatasheetGaps
		if strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("run %d gave %v, first run gave %v", i, got, first)
		}
	}
}

// TestGapSectionDistinguishesEmptyFromAbsent: a corpus that seeds everything and no corpus at all both
// used to render as no section, so a run that forgot the corpus looked like a fully seeded board.
func TestGapSectionDistinguishesEmptyFromAbsent(t *testing.T) {
	seeded := param.ProviderFunc(func(mpn string) *parampb.PartSpec { return &parampb.PartSpec{Mpn: mpn} })
	withCorpus := Markdown(Build(check.NewModelWithParams(mpnDesign(), nil, seeded)), false)
	if !strings.Contains(withCorpus, "Datasheet gaps") {
		t.Error("a board with every part seeded printed no gap section, so it reads as a run with no corpus")
	}
	if !strings.Contains(withCorpus, "none") {
		t.Errorf("the empty queue does not say so:\n%s", withCorpus)
	}
	noCorpus := Markdown(Build(check.NewModel(mpnDesign())), false)
	if strings.Contains(noCorpus, "Datasheet gaps") {
		t.Error("a run with no corpus printed a gap section, which would claim a queue it never built")
	}
}

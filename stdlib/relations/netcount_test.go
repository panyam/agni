package relations

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// netCountDesign carries one of each case component.net_count has to tell apart: a two-terminal
// part across two nets, a part with both pins on ONE net, a placed part wired to nothing, and a
// connection naming a ref the design has no component record for.
func netCountDesign() *ir.Design {
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	return &ir.Design{
		Components: []*ir.Component{comp("R1"), comp("J1"), comp("U9")},
		Nets: []*ir.Net{
			tnet("A", "R1.1", "J1.1", "J1.2", "X1.1"),
			tnet("B", "R1.2"),
		},
	}
}

func netCounts(t *testing.T, m check.Model) map[string]float64 {
	t.Helper()
	got := map[string]float64{}
	for _, f := range factsByRelation(Facts(m))[RelComponentNetCount] {
		if f.Num == nil {
			t.Fatalf("component.net_count(%s) carries no number: %+v", f.Subject, f)
		}
		if _, dup := got[f.Subject]; dup {
			t.Errorf("component.net_count(%s) projected twice", f.Subject)
		}
		got[f.Subject] = *f.Num
	}
	return got
}

// TestComponentNetCountCountsDistinctNets checks that the count is of NETS, so J1's two pins on A
// count once, and every component gets a row, so U9 answers 0 rather than being absent.
func TestComponentNetCountCountsDistinctNets(t *testing.T) {
	got := netCounts(t, check.NewModel(netCountDesign()))
	want := map[string]float64{"R1": 2, "J1": 1, "U9": 0, "X1": 1}
	for ref, n := range want {
		c, ok := got[ref]
		if !ok {
			t.Errorf("component.net_count(%s) missing, want %v", ref, n)
			continue
		}
		if c != n {
			t.Errorf("component.net_count(%s) = %v, want %v", ref, c, n)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rows = %v, want exactly %v", got, want)
	}
}

// TestComponentNetCountAgreesWithComponentOnNet checks that for every ref the count equals the
// number of distinct nets component.net places it on. The relation is a derived count of that
// one, and a second opinion about connectivity is how two relations start to disagree.
func TestComponentNetCountAgreesWithComponentOnNet(t *testing.T) {
	m := check.NewModel(netCountDesign())
	nets := map[string]map[string]bool{}
	for _, f := range factsByRelation(Facts(m))[RelComponentOnNet] {
		if nets[f.Subject] == nil {
			nets[f.Subject] = map[string]bool{}
		}
		nets[f.Subject][f.Object] = true
	}
	counts := netCounts(t, m)
	if len(counts) != len(nets)+1 { // +1 for U9, which component.net never mentions
		t.Fatalf("net_count has %d rows, want one per ref component.net names plus U9: %v", len(counts), counts)
	}
	for ref, c := range counts {
		if int(c) != len(nets[ref]) {
			t.Errorf("%s: net_count %v, component.net places it on %d nets", ref, c, len(nets[ref]))
		}
	}
}

// TestComponentNetCountCitesTheComponent checks that a row cites the placement it rests on. A ref with
// no component record has no placement to cite and cites nothing rather than inventing one.
func TestComponentNetCountCitesTheComponent(t *testing.T) {
	rows := factsByRelation(Facts(check.NewModel(netCountDesign())))[RelComponentNetCount]
	if len(rows) == 0 {
		t.Fatal("no component.net_count rows, so there is nothing whose citations to check")
	}
	for _, f := range rows {
		switch f.Subject {
		case "X1":
			if len(f.Cites) != 0 {
				t.Errorf("X1 cites %v, want none", f.Cites)
			}
		default:
			if len(f.Cites) == 0 {
				t.Errorf("%s cites nothing, want its component's provenance", f.Subject)
			}
		}
	}
}

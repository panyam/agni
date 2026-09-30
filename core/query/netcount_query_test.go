package query

import (
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestNetCountReplacesTheSelfJoin is agni issue 727's acceptance. "Capacitors on exactly two nets"
// used to need three copies of component-on-net and a negation; component.net_count says it in one
// clause, and the two must name the same parts. C1 is two-terminal, C2 has both pins on one net, C3
// spans three.
func TestNetCountReplacesTheSelfJoin(t *testing.T) {
	prov := &ir.Provenance{SourceFile: "d"}
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, DeviceClasses: classify.Tags("capacitor"), Prov: prov}
	}
	net := func(name string, conns ...[2]string) *ir.Net {
		n := &ir.Net{Name: name, Prov: prov}
		for _, c := range conns {
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: c[0], PinRef: c[1]})
		}
		return n
	}
	d := &ir.Design{
		Components: []*ir.Component{comp("C1"), comp("C2"), comp("C3")},
		Nets: []*ir.Net{
			net("A", [2]string{"C1", "1"}, [2]string{"C2", "1"}, [2]string{"C2", "2"}, [2]string{"C3", "1"}),
			net("B", [2]string{"C1", "2"}, [2]string{"C3", "2"}),
			net("C", [2]string{"C3", "3"}),
		},
	}
	m := check.NewModel(d)
	refs := func(q string) string {
		var out []string
		for _, r := range runQuery(t, m, q) {
			out = append(out, r.Bind["r"].S)
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	selfJoin := refs(`three(?r) :- component-on-net(?r,?a), component-on-net(?r,?b), component-on-net(?r,?c), ?a < ?b, ?b < ?c;
		two(?r) :- component.class(?r,"capacitor"), component-on-net(?r,?a), component-on-net(?r,?b), ?a < ?b, not three(?r);
		two(?r) => ?r`)
	oneClause := refs(`component.class(?r,"capacitor"), component.net_count(?r, 2) => ?r`)
	if selfJoin != "C1" {
		t.Fatalf("self-join = %q, want C1 (the control the new clause is compared against)", selfJoin)
	}
	if oneClause != selfJoin {
		t.Errorf("net_count clause = %q, self-join = %q, want the same parts", oneClause, selfJoin)
	}
}

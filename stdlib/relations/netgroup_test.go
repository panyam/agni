package relations

import (
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestNetGroupProjectsEveryMember projects one declared pair and one match group (agni issue 1360).
// Each member net is a row, the kind is spelled for a query, and a group of an unknown kind is left
// out rather than published with an empty kind.
func TestNetGroupProjectsEveryMember(t *testing.T) {
	prov := &ir.Provenance{SourceFile: "board.brd"}
	d := &ir.Design{
		Nets: []*ir.Net{tnet("CLK", "U1.1"), tnet("CLKN", "U1.2"), tnet("D0", "U1.3")},
		NetGroups: []*ir.NetGroup{
			{Name: "DIFFPAIR2", Kind: ir.NetGroupKind_NET_GROUP_KIND_DIFF_PAIR, Nets: []string{"CLK", "CLKN"}, Prov: prov},
			{Name: "LANE0", Kind: ir.NetGroupKind_NET_GROUP_KIND_MATCH_GROUP, Nets: []string{"CLK", "CLKN", "D0"}, Prov: prov},
			{Name: "ODD", Nets: []string{"D0"}, Prov: prov},
		},
	}
	rows := factsByRelation(Facts(check.NewModel(d)))[RelNetGroup]
	var got []string
	for _, r := range rows {
		got = append(got, r.Subject+" "+r.Object+" "+r.Value)
		if len(r.Cites) == 0 {
			t.Errorf("row %s %s carries no citation", r.Subject, r.Value)
		}
	}
	sort.Strings(got)
	want := "DIFFPAIR2 diff_pair CLK|DIFFPAIR2 diff_pair CLKN|LANE0 match_group CLK|LANE0 match_group CLKN|LANE0 match_group D0"
	if strings.Join(got, "|") != want {
		t.Errorf("net.group rows = %v, want %s", got, want)
	}
}

package formats

import (
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestAllegroBoardsDeclareTheirPairs reads the differential pairs and match groups two Allegro
// boards declare (agni issue 1360). No export lists them, so the name vocabulary is the positive
// control. Four declared pairs are pairs by name too, so the read and the vocabulary agree there. The
// other 27 are the ones the declaration is for.
func TestAllegroBoardsDeclareTheirPairs(t *testing.T) {
	l := &Loader{}
	bbai, err := l.ReadDesign("../../tools/samples/allegro/beaglebone-ai/BeagleBone-AI.brd")
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]*ir.NetGroup{}
	pairs, lanes := 0, 0
	for _, g := range bbai.GetNetGroups() {
		groups[g.GetName()] = g
		switch g.GetKind() {
		case ir.NetGroupKind_NET_GROUP_KIND_DIFF_PAIR:
			pairs++
		case ir.NetGroupKind_NET_GROUP_KIND_MATCH_GROUP:
			lanes++
		}
	}
	if pairs != 31 || lanes != 7 {
		t.Fatalf("the BeagleBone AI declares %d pairs and %d match groups, want 31 and 7", pairs, lanes)
	}
	for name, want := range map[string][]string{
		"DIFFPAIR2": {"DDR1_CLK0", "DDR1_CLK0N"},
		"SSTX1":     {"SSTX1+", "SSTX1-"},
		"USB1_D":    {"USB1_DM", "USB1_DP"},
		"DDR1_DQS1": {"DDR1_DQS1", "DDR1_DQSN1"},
	} {
		if g := groups[name]; g == nil || !slices.Equal(g.GetNets(), want) {
			t.Errorf("pair %s = %v, want %v", name, g.GetNets(), want)
		}
	}
	// A strobe pair sits in its byte lane as well as in its own pair.
	if lane := groups["DDR1_DQ1"]; lane == nil || len(lane.GetNets()) != 11 || !slices.Contains(lane.GetNets(), "DDR1_DQSN1") {
		t.Errorf("byte lane DDR1_DQ1 = %v, want its 11 nets with the strobe pair", lane.GetNets())
	}

	// The vocabulary pairs _DP with _DN, so it expects USB1_DN where the board pairs USB1_DP with
	// USB1_DM. Those two pairs are the ones it names wrongly.
	m := check.NewModel(bbai)
	agree, missed := 0, 0
	var wrong []string
	for _, g := range bbai.GetNetGroups() {
		for _, n := range g.GetNets() {
			if !m.HasNetName(n) {
				t.Errorf("group %s names %s, which the design lacks", g.GetName(), n)
			}
		}
		if g.GetKind() != ir.NetGroupKind_NET_GROUP_KIND_DIFF_PAIR {
			continue
		}
		a, b := g.GetNets()[0], g.GetNets()[1]
		negA, posA := m.DiffNegative(a)
		negB, posB := m.DiffNegative(b)
		switch {
		case posA && strings.EqualFold(negA, b), posB && strings.EqualFold(negB, a):
			agree++
		case posA || posB:
			wrong = append(wrong, g.GetName())
		default:
			missed++
		}
	}
	t.Logf("of 31 declared pairs, the name vocabulary finds %d, names %v wrongly, and misses %d", agree, wrong, missed)
	if agree != 4 || missed != 25 || !slices.Equal(wrong, []string{"USB1_D", "USB2_D"}) {
		t.Errorf("the vocabulary finds %d declared pairs, names %v wrongly and misses %d, want 4, [USB1_D USB2_D] and 25", agree, wrong, missed)
	}

	parasdr, err := l.ReadDesign(parasdr + "parasdr_layout.brd")
	if err != nil {
		t.Fatal(err)
	}
	if gs := parasdr.GetNetGroups(); len(gs) != 1 || gs[0].GetName() != "RX_MG" || gs[0].GetKind() != ir.NetGroupKind_NET_GROUP_KIND_MATCH_GROUP || len(gs[0].GetNets()) != 14 {
		t.Errorf("parasdr's groups = %v, want its one 16.6 match group RX_MG of 14 nets", gs)
	}
}

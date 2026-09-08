package kicad

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The oracle cross-check: read a schematic OUR way, read the same design's answer from the tool that
// produced it, and require the two to agree on which pins share a net.
//
// It compares the PARTITION and never the names or the counts, for two reasons that both cost time
// to learn. Names cannot match: an unnamed net is auto-named by each tool in its own vocabulary
// (`N$37` against `Net-(C104-Pad1)`), and KiCad writes a root-sheet label as `/AN0` where we write
// `AN0`. Counts hide compensating errors: on StickHub we read 47 nets against KiCad's 47 while
// disagreeing about 19 of them, because a swapped pin pair moves one connection out of a net and
// another in (see build/evidence.md on why a matching total is not agreement).

// pinNets maps "REF.PIN" -> net name over a design's nets.
func pinNets(d *ir.Design) map[string]string {
	out := map[string]string{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			out[c.GetComponentRef()+"."+c.GetPinRef()] = n.GetName()
		}
	}
	return out
}

// readOracle parses a committed reference netlist: "<net> <ref>.<pin> ..." per line, '#' comments.
func readOracle(t *testing.T, name string) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("oracle %s: %v", name, err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		for _, pin := range fields[1:] {
			out[pin] = fields[0]
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("oracle %s: %v", name, err)
	}
	if len(out) == 0 {
		t.Fatalf("oracle %s parsed to nothing", name)
	}
	return out
}

// disagreements compares two pin->net maps over the pins BOTH describe and returns one line per
// disagreement: a reference net whose pins we spread across several nets (a split), and one of ours
// that spans several of theirs (a merge). Empty means the two partitions are identical.
func disagreements(ours, ref map[string]string) []string {
	shared := map[string]bool{}
	for p := range ours {
		if _, ok := ref[p]; ok {
			shared[p] = true
		}
	}
	group := func(m map[string]string) map[string][]string {
		g := map[string][]string{}
		for p := range shared {
			g[m[p]] = append(g[m[p]], p)
		}
		for _, pins := range g {
			sort.Strings(pins)
		}
		return g
	}
	var out []string
	for _, side := range []struct {
		label      string
		from, into map[string]string
	}{
		{"we SPLIT reference net", ref, ours},
		{"we MERGE into one net", ours, ref},
	} {
		var names []string
		for n := range group(side.from) {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			pins := group(side.from)[n]
			seen := map[string]bool{}
			for _, p := range pins {
				seen[side.into[p]] = true
			}
			if len(seen) <= 1 {
				continue
			}
			var got []string
			for s := range seen {
				got = append(got, s)
			}
			sort.Strings(got)
			out = append(out, fmt.Sprintf("%s %q (pins %v) -> %v", side.label, n, pins, got))
		}
	}
	return out
}

// TestBusVectorCrossesSheetBoundary is the regression guard for agni issue 561: a bus VECTOR entering
// a sub-sheet through a bus sheet pin carries its members with it, so the root's DATA0 tap and the
// sub-sheet's DATA0 tap are one net. Before the fix each half kept its own sheet's scope, `AN0` and
// `/inout_user/AN0` never met, and every such signal reported as two single-pin stubs.
//
// It red-checks: revert busPinPromotions and both members split.
func TestBusVectorCrossesSheetBoundary(t *testing.T) {
	d, complete, err := ReadSchematicHierarchyNets("hier_busvec_root.kicad_sch",
		readFixture(t, "hier_busvec_root.kicad_sch"), hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Fatal("hierarchy walk did not complete; the sub-sheet did not open")
	}
	if bad := disagreements(pinNets(d), readOracle(t, "hier_busvec.oracle")); len(bad) > 0 {
		t.Errorf("net partition disagrees with kicad-cli:\n  %s", strings.Join(bad, "\n  "))
	}
}

// TestBusVectorOffsetRangeMapsByPosition is the other half of the same fix, and the half that says
// why promotion is a MAP rather than a rename. The parent's bus is `PP[2..3]` and the sheet pin is
// `PP[0..1]`, so the child's `PP0` is the parent's `PP2`. Joining members by name would leave all
// four taps separate; promoting by name would merge the two buses into one.
//
// This is what `vme-wren` does at scale: it cuts `PP_OUT[0..31]` into four eight-wide slices and
// hands each to its own instance of the same driver sheet, whose pin is always spelled
// `PP_OUT[0..7]`. An earlier revision refused to promote there at all, which was safe and left the
// nets split.
func TestBusVectorOffsetRangeMapsByPosition(t *testing.T) {
	d, complete, err := ReadSchematicHierarchyNets("hier_busoffset_root.kicad_sch",
		readFixture(t, "hier_busoffset_root.kicad_sch"), hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Fatal("hierarchy walk did not complete; the sub-sheet did not open")
	}
	if bad := disagreements(pinNets(d), readOracle(t, "hier_busoffset.oracle")); len(bad) > 0 {
		t.Errorf("net partition disagrees with kicad-cli:\n  %s", strings.Join(bad, "\n  "))
	}
}

// TestGroupBusCrossesSheetBoundary is the guard for agni issue 597, the other third of issue 561.
// A GROUP bus takes its members from a `bus_alias` rather than an index range, and it crosses a sheet
// boundary the same way a vector does: the root's `I2C0.SDA` tap and the sub-sheet's `I2C7.SDA` tap
// are one net, named by the parent's prefix.
//
// The fixture carries its own control. `I2C8{I2C}` expands the SAME two member names off the SAME
// alias and crosses nothing, so R3 and R4 must stay off I2C0's nets. That direction is the dangerous
// one: eight buses share the `I2C` alias on the jetson baseboard, so a promotion that dropped the
// prefix would short all eight while moving the net count toward KiCad's answer.
//
// It red-checks both ways: drop the group branch of busPinPromotions and the crossing halves split;
// promote without the prefix and the control merges.
func TestGroupBusCrossesSheetBoundary(t *testing.T) {
	d, complete, err := ReadSchematicHierarchyNets("hier_busgroup_root.kicad_sch",
		readFixture(t, "hier_busgroup_root.kicad_sch"), hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Fatal("hierarchy walk did not complete; the sub-sheet did not open")
	}
	if bad := disagreements(pinNets(d), readOracle(t, "hier_busgroup.oracle")); len(bad) > 0 {
		t.Errorf("net partition disagrees with kicad-cli:\n  %s", strings.Join(bad, "\n  "))
	}
}

// TestGroupBusNeedsADeclaredAlias is the false-positive control, and the reason recognition is a
// table lookup rather than a tighter pattern.
//
// KiCad renders `_{...}` as a subscript, so `A_{1}` and `3V3_{OUT}` are ordinary scalar labels with
// the exact shape of a group bus. No pattern separates them from `I2C0{I2C}`; what separates them is
// that no `bus_alias` declares `1` or `OUT`. The jetson baseboard carries over a hundred distinct
// subscript groups and not one names an alias, so the lookup is exact where a pattern could only
// guess.
func TestGroupBusNeedsADeclaredAlias(t *testing.T) {
	aliases := map[string][]string{"I2C": {"SDA", "SCL"}}
	for _, tc := range []struct {
		name    string
		prefix  string
		isGroup bool
	}{
		{name: "I2C0{I2C}", prefix: "I2C0", isGroup: true},
		// The alias is the LAST brace group, and a prefix may carry one of its own. This spelling is
		// the most common group bus on the jetson baseboard.
		{name: "I2C_{SYS}{I2C}", prefix: "I2C_{SYS}", isGroup: true},
		{name: "A_{1}"},
		{name: "3V3_{OUT}"},
		{name: "I2C0{SPI}"},
		{name: "I2C0"},
		{name: "DATA[0..1]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix, _, ok := groupBus(tc.name, aliases)
			if ok != tc.isGroup {
				t.Fatalf("groupBus(%q) recognized = %v, want %v", tc.name, ok, tc.isGroup)
			}
			if ok && prefix != tc.prefix {
				t.Errorf("groupBus(%q) prefix = %q, want %q", tc.name, prefix, tc.prefix)
			}
		})
	}
}

// TestBusMembersAscending pins the ordering the positional map is built on. kicad-cli joins the
// child's bit 0 of `B[0..1]` to `A0` of a parent bus spelled `A[3..0]`, not to `A3`, so the members
// pair off by ascending index whatever direction the range is written in. netgraph.ExpandBusName
// keeps the WRITTEN direction on purpose, for diagrams, which is why this does not reuse it.
func TestBusMembersAscending(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"A[0..1]", []string{"A0", "A1"}},
		{"A[3..0]", []string{"A0", "A1", "A2", "A3"}},
		{"PP_OUT[8..10]", []string{"PP_OUT8", "PP_OUT9", "PP_OUT10"}},
		{"DATA[1:0]", nil}, // xschem's spelling, not a KiCad bus
		{"PLAIN", nil},
	}
	for _, c := range cases {
		got := busMembersAscending(c.in)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("busMembersAscending(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

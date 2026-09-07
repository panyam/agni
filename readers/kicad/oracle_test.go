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

// TestBusVectorSlicedPrefixNotPromoted guards the other side of the same fix. A parent that cuts one
// prefix into several ranges (PP_OUT[0..31] alongside PP_OUT[0..7], PP_OUT[8..15], ...) hands each
// slice to its own instance of the same child sheet, and every instance's pin is spelled PP_OUT[0..7].
// Member names no longer identify a signal there — KiCad maps the two buses by bit position, so the
// child's PP_OUT1 is the parent's PP_OUT1 under one instance and PP_OUT9 under the next. Promoting by
// name merged nets the design keeps apart, so a sliced prefix is skipped.
func TestBusVectorSlicedPrefixNotPromoted(t *testing.T) {
	root, err := parse(strings.NewReader(`(kicad_sch
		(label "PP_OUT[0..31]" (at 10 10 0))
		(label "PP_OUT[0..7]" (at 10 20 0))
		(label "OTHER[0..7]" (at 10 30 0)))`))
	if err != nil {
		t.Fatal(err)
	}
	sliced := slicedBusPrefixes(root)
	if !sliced["PP_OUT"] {
		t.Error("PP_OUT is drawn as several ranges and must count as sliced")
	}
	if sliced["OTHER"] {
		t.Error("OTHER has one range and must not count as sliced")
	}
}

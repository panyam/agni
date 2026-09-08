package formats

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/edif"
)

// emitCases is one design per reader that produces connectivity, which is the axis agni issue 563
// went unnoticed along: the EDIF writer was only ever exercised on EDIF input, so a fallback that
// dropped every anchor was invisible for every other format. Adding a row is how a new reader
// inherits the guarantee.
//
// The counts are the SOURCE design's, so a row states what the fixture holds as well as what
// survives, and a reader change that moves either shows up here rather than silently rebasing.
var emitCases = []struct {
	name, path           string
	wantComps, wantConns int
	// unanchored is the number of connections naming a component the design does not carry: a KiCad
	// power symbol or PWR_FLAG, which the reader records on "#PWR01" while keeping the component list
	// physical. EDIF has no instance for one, so those alone come back as no-ref connections.
	unanchored int
	// unresolvedRefs is the number of emitted portRefs naming nothing on the instance's cell, which
	// is zero for every format that delivers part types and every reference for the two that do not.
	// Read by TestEmitEDIFResolvesEveryPortRef.
	unresolvedRefs int
}{
	{name: "kicad-sch", path: "../../examples/tutorial-project/designs/gateway/gateway.kicad_sch", wantComps: 19, wantConns: 56},
	{name: "kicad-pcb", path: "../../examples/tutorial-project/designs/gateway/gateway.kicad_pcb", wantComps: 19, wantConns: 56, unresolvedRefs: 56},
	{name: "kicad-hier", path: "../kicad/testdata/hier_root.kicad_sch", wantComps: 6, wantConns: 13, unanchored: 3},
	{name: "kicad-multisection", path: "../kicad/testdata/dup_refdes.kicad_sch", wantComps: 2, wantConns: 2},
	{name: "edif", path: "../../examples/tutorial-project/designs/gateway/gateway.edn", wantComps: 19, wantConns: 56},
	{name: "ipc2581", path: "../ipc2581/testdata/board.xml", wantComps: 3, wantConns: 8, unresolvedRefs: 8},
	{name: "telesis", path: "../telesis/testdata/basic.tel", wantComps: 13, wantConns: 19},
	{name: "geda", path: "../geda/testdata/dup_refdes.sch", wantComps: 7, wantConns: 18},
	{name: "geda-slotted", path: "../geda/testdata/slotted.sch", wantComps: 1, wantConns: 4},
	{name: "xschem", path: "../xschem/testdata/divider.sch", wantComps: 2, wantConns: 4},
}

// TestEmitEDIFKeepsConnectivity is the guard agni issue 563 needed and did not have. Read a design in
// each format, emit an EDIF netlist, read that back, and require every pin-to-part link to survive.
//
// It asserts the PAIRS rather than a count, because a count is the thing the bug already satisfied:
// the broken writer emitted one (portRef ...) per connection and the re-read produced one connection
// per portRef, so connection totals matched exactly while every one of them had lost its component.
// The failing report on a real board was 1123 components and 1387 nets, all correct, and 1123
// unconnected components.
func TestEmitEDIFKeepsConnectivity(t *testing.T) {
	for _, tc := range emitCases {
		t.Run(tc.name, func(t *testing.T) {
			src := readForEmit(t, tc.path)
			if got := len(src.GetComponents()); got != tc.wantComps {
				t.Errorf("source components = %d, want %d", got, tc.wantComps)
			}
			var conns int
			for _, n := range src.GetNets() {
				conns += len(n.GetConnections())
			}
			if conns != tc.wantConns {
				t.Errorf("source connections = %d, want %d", conns, tc.wantConns)
			}

			out := roundTripEDIF(t, src, tc.path)
			want, unanchored := anchoredPairs(src)
			if unanchored != tc.unanchored {
				t.Errorf("connections naming no component = %d, want %d", unanchored, tc.unanchored)
			}
			if len(want) == 0 {
				t.Fatal("no anchored links in the source; the assertion below would pass vacuously")
			}
			got, _ := anchoredPairs(out)
			gotSet := map[string]int{}
			for _, p := range got {
				gotSet[p]++
			}
			var missing []string
			for _, p := range want {
				if gotSet[p] == 0 {
					missing = append(missing, p)
					continue
				}
				gotSet[p]--
			}
			if len(missing) > 0 {
				sort.Strings(missing)
				t.Errorf("%d of %d pin-to-part link(s) lost through the EDIF emit, first few: %v",
					len(missing), len(want), missing[:min(8, len(missing))])
			}
		})
	}
}

// TestEmitEDIFNamesEveryInstanceOnce is the property the anchors rest on. An (instanceRef ...) is a
// lookup by name, so two instances written under one name make one of them unreachable and silently
// move its pins onto the other.
//
// Nothing in the IR supplies a unique name on its own. A section's native id is absent for two of
// these formats and repeats in a third (one KiCad symbol placed on two sheets of a hierarchy carries
// the same id under two ref-des), and a ref-des repeats for every multi-gate part and every genuine
// duplicate. Before the table that breaks these collisions, a .tel design emitted thirteen instances
// all called I0.
func TestEmitEDIFNamesEveryInstanceOnce(t *testing.T) {
	for _, tc := range emitCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := edif.WriteNetlist(&buf, readForEmit(t, tc.path)); err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for line := range bytes.SplitSeq(buf.Bytes(), []byte("\n")) {
				fields := bytes.Fields(line)
				if len(fields) < 2 || string(fields[0]) != "(instance" {
					continue
				}
				name := string(fields[1])
				if seen[name] {
					t.Errorf("instance name %q written twice; an instanceRef to it reaches only the first", name)
				}
				seen[name] = true
			}
			if len(seen) == 0 {
				t.Fatal("no instances emitted; the assertion would pass vacuously")
			}
		})
	}
}

func readForEmit(t *testing.T, path string) *ir.Design {
	t.Helper()
	l := &Loader{SymbolPaths: []string{filepath.Dir(path), filepath.Join(filepath.Dir(path), "symbols")}}
	d, err := l.ReadDesign(path)
	if err != nil {
		t.Fatalf("%s: read: %v", path, err)
	}
	return d
}

func roundTripEDIF(t *testing.T, d *ir.Design, path string) *ir.Design {
	t.Helper()
	var buf bytes.Buffer
	if err := edif.WriteNetlist(&buf, d); err != nil {
		t.Fatalf("%s: write: %v", path, err)
	}
	out, err := edif.Read(bytes.NewReader(buf.Bytes()), path)
	if err != nil {
		t.Fatalf("%s: re-read of emitted netlist: %v\n%s", path, err, buf.String())
	}
	return out
}

// anchoredPairs lists every "net/refdes.pin" a design states, and separately counts the connections
// that name no component of it. The pair is what a consumer of a netlist reads; the net name is part
// of the key because a link surviving onto the WRONG net is not a link that survived.
func anchoredPairs(d *ir.Design) ([]string, int) {
	have := map[string]bool{}
	for _, c := range d.GetComponents() {
		have[c.GetRefDes()] = true
	}
	var pairs []string
	var unanchored int
	for _, n := range d.GetNets() {
		for _, cn := range n.GetConnections() {
			if cn.GetComponentRef() == "" || !have[cn.GetComponentRef()] {
				unanchored++
				continue
			}
			pairs = append(pairs, fmt.Sprintf("%s/%s.%s", n.GetName(), cn.GetComponentRef(), cn.GetPinRef()))
		}
	}
	return pairs, unanchored
}

package formats

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/panyam/agni/core/classify"
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
}{
	{name: "kicad-sch", path: "../../examples/tutorial-project/designs/gateway/gateway.kicad_sch", wantComps: 19, wantConns: 56},
	{name: "kicad-pcb", path: "../../examples/tutorial-project/designs/gateway/gateway.kicad_pcb", wantComps: 19, wantConns: 56},
	{name: "kicad-hier", path: "../kicad/testdata/hier_root.kicad_sch", wantComps: 6, wantConns: 13, unanchored: 3},
	{name: "kicad-multisection", path: "../kicad/testdata/dup_refdes.kicad_sch", wantComps: 2, wantConns: 2},
	{name: "edif", path: "../../examples/tutorial-project/designs/gateway/gateway.edn", wantComps: 19, wantConns: 56},
	{name: "ipc2581", path: "../ipc2581/testdata/board.xml", wantComps: 3, wantConns: 8},
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
			for _, m := range emittedInstanceIDs.FindAllStringSubmatch(buf.String(), -1) {
				// The identifier is the bare atom, or the ID inside a (rename ID "display") for a
				// source name that had to be minted into a legal one.
				name := m[1]
				if name == "" {
					name = m[2]
				}
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

// emittedInstanceIDs pulls the identifier an (instance ...) is written under, in either name form.
// The identifier is what an (instanceRef ...) looks up, so it is the string that has to be unique.
var emittedInstanceIDs = regexp.MustCompile(`\(instance (?:\(rename ([^\s()]+) "[^"]*"\)|([^\s()]+)) `)

// TestEmitEDIFKeepsPartIdentity requires a part number stated by the source to survive an export.
//
// The datasheet tier joins on component.mpn, so an export that drops it hands the recipient a netlist
// that looks complete and silently fails a BOM join. The writer carries a SECTION's attributes into
// the instance's properties, which is why a reader that records the part number only on the component
// exports nothing: the value is in the IR and not in the half the writer reads (agni issue 584).
//
// Rows with no MPN in the source are skipped rather than asserted at zero, because most fixtures here
// state none and a design that carries no part number legitimately exports none.
func TestEmitEDIFKeepsPartIdentity(t *testing.T) {
	for _, tc := range emitCases {
		t.Run(tc.name, func(t *testing.T) {
			src := readForEmit(t, tc.path)
			want := map[string]string{}
			for _, c := range src.GetComponents() {
				if m := c.GetMpn(); m != "" {
					want[c.GetRefDes()] = m
				}
			}
			if len(want) == 0 {
				t.Skip("source states no part number")
			}
			// The re-read goes through edif.Read rather than the Loader, so the format-neutral
			// ingestion passes have not run on it. Stamping here is what makes the two sides
			// comparable; without it the assertion fails for every format on an empty right-hand side.
			out := roundTripEDIF(t, src, tc.path)
			classify.StampMPN(out)
			got := map[string]string{}
			for _, c := range out.GetComponents() {
				if m := c.GetMpn(); m != "" {
					got[c.GetRefDes()] = m
				}
			}
			var lost []string
			for ref, m := range want {
				if got[ref] != m {
					lost = append(lost, fmt.Sprintf("%s=%s", ref, m))
				}
			}
			if len(lost) > 0 {
				sort.Strings(lost)
				t.Errorf("%d of %d part number(s) lost through the emit, first few: %v",
					len(lost), len(want), lost[:min(5, len(lost))])
			}
		})
	}
}

package edif

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"google.golang.org/protobuf/proto"
)

func readFlat(t *testing.T, name string) *ir.Design {
	t.Helper()
	d, err := ReadWith(bytes.NewReader(readFixture(t, name)), name, Options{FlattenHierarchy: true})
	if err != nil {
		t.Fatalf("ReadWith %s: %v", name, err)
	}
	return d
}

// partition maps every "REF.PIN" to the net it lands on. It is what a hierarchy walk has to get
// right, because a pin joined to the wrong side of a block's port moves between nets while every
// count stays the same.
func partition(d *ir.Design) map[string]string {
	out := map[string]string{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			out[c.GetComponentRef()+"."+c.GetPinRef()] = n.GetName()
		}
	}
	return out
}

func refs(d *ir.Design) []string {
	var out []string
	for _, c := range d.GetComponents() {
		out = append(out, c.GetRefDes())
	}
	return out
}

// TestHierarchicalEDIFFlattens reads hier_blocks.edn, whose top cell places a connector, one gate of
// U5, the block LED_DRV once (A1) and the block RC twice (B1, B2). LED_DRV and RC each hold a local
// net named MID, so all three are qualified, and LED_DRV's SENSE is unique and keeps its name.
func TestHierarchicalEDIFFlattens(t *testing.T) {
	d := readFlat(t, "hier_blocks.edn")

	wantRefs := []string{"U5", "J1", "R1", "D1", "TP2", "B1/R1", "B1/C1", "B1/TP1", "B2/R1", "B2/C1", "B2/TP1"}
	if got := refs(d); strings.Join(got, " ") != strings.Join(wantRefs, " ") {
		t.Errorf("components = %v, want %v", got, wantRefs)
	}
	want := map[string]string{
		"J1.1": "VCC", "R1.A": "VCC", "B1/R1.A": "VCC", "B2/R1.A": "VCC",
		"U5.1": "NET_LED_CATHODE", "D1.K": "NET_LED_CATHODE", "U5.4": "NET_LED_CATHODE",
		"J1.2": "GND", "B1/C1.2": "GND", "B2/C1.2": "GND",
		"U5.2": "OUT1", "B1/R1.B": "OUT1", "B1/C1.1": "OUT1",
		"B2/R1.B": "OUT2", "B2/C1.1": "OUT2",
		"R1.B": "/A1/MID", "D1.A": "/A1/MID", "U5.3": "/A1/MID",
		"TP2.1": "SENSE", "B1/TP1.1": "/B1/MID", "B2/TP1.1": "/B2/MID",
	}
	got := partition(d)
	if len(got) != len(want) {
		t.Errorf("%d pins connected, want %d: %v", len(got), len(want), got)
	}
	for pin, net := range want {
		if got[pin] != net {
			t.Errorf("%s on %q, want %q", pin, got[pin], net)
		}
	}
	var nets []string
	for _, n := range d.GetNets() {
		nets = append(nets, n.GetName())
	}
	if w := "VCC NET_LED_CATHODE GND OUT1 OUT2 /A1/MID SENSE /B1/MID /B2/MID"; strings.Join(nets, " ") != w {
		t.Errorf("nets = %v, want %s", nets, w)
	}
	if u := d.GetInputDiagnostics().GetUnexpandedHierarchy(); len(u) != 0 {
		t.Errorf("unexpanded_hierarchy = %v, want none once every block is walked", u)
	}
	for _, lib := range d.GetLibraries() {
		for _, p := range lib.GetParts() {
			if p.GetName() == "LED_DRV" || p.GetName() == "RC" {
				t.Errorf("block %s read as a part type", p.GetName())
			}
		}
	}
}

// TestUnflattenedReadIsTheTopCell is the control: the same file read without the option is its top
// cell and lists both blocks.
func TestUnflattenedReadIsTheTopCell(t *testing.T) {
	d := readEDN(t, "hier_blocks.edn")
	if got := refs(d); len(got) != 5 || compByRef(d, "R1") != nil {
		t.Errorf("components = %q, want the top cell's five instances alone", got)
	}
	var names []string
	for _, u := range d.GetInputDiagnostics().GetUnexpandedHierarchy() {
		names = append(names, u.GetName())
	}
	if strings.Join(names, " ") != "LED_DRV RC" {
		t.Errorf("unexpanded_hierarchy = %v, want LED_DRV RC", names)
	}
	for _, u := range d.GetInputDiagnostics().GetUnexpandedHierarchy() {
		if u.GetReason() != ir.UnexpandedReason_UNEXPANDED_REASON_NOT_WALKED {
			t.Errorf("%s reason = %v, want NOT_WALKED on a top-level read", u.GetName(), u.GetReason())
		}
	}
}

// TestCellPlacedTwiceYieldsTwoSets guards the failure #718 names, two placements read as one merged
// set, which gives a design that looks smaller and cleaner than it is.
func TestCellPlacedTwiceYieldsTwoSets(t *testing.T) {
	d := readFlat(t, "hier_blocks.edn")
	p := partition(d)
	if p["B1/R1.B"] == p["B2/R1.B"] {
		t.Errorf("both placements of RC put R1.B on %q, want two nets", p["B1/R1.B"])
	}
	n := 0
	for _, c := range d.GetComponents() {
		if strings.HasSuffix(c.GetRefDes(), "/R1") {
			n++
			if len(c.GetSections()) != 1 {
				t.Errorf("%s has %d sections, want 1", c.GetRefDes(), len(c.GetSections()))
			}
		}
	}
	if n != 2 {
		t.Errorf("%d R1s inside RC, want 2", n)
	}
}

// TestMultiGateAcrossCellsIsOneComponent holds that a designator shared across sheets is one part.
func TestMultiGateAcrossCellsIsOneComponent(t *testing.T) {
	d := readFlat(t, "hier_blocks.edn")
	u5 := compByRef(d, "U5")
	if u5 == nil || len(u5.GetSections()) != 2 {
		t.Fatalf("U5 = %v, want one component with a section in TOP and one in LED_DRV", u5)
	}
}

// TestCyclicCellIsListedNotWalked holds that a block placing itself ends the walk and is reported,
// rather than recursing until the stack runs out.
func TestCyclicCellIsListedNotWalked(t *testing.T) {
	d := readFlat(t, "hier_cycle.edn")
	if got := refs(d); strings.Join(got, " ") != "R1" {
		t.Errorf("components = %v, want R1 from the one placement walked", got)
	}
	u := d.GetInputDiagnostics().GetUnexpandedHierarchy()
	if len(u) != 1 || u[0].GetName() != "LOOP" || u[0].GetReason() != ir.UnexpandedReason_UNEXPANDED_REASON_CYCLE {
		t.Errorf("unexpanded_hierarchy = %v, want LOOP for a cycle", u)
	}
}

// TestFlatEDIFReadsIdenticallyFlattened holds every flat netlist fixture to the same IR with the
// option on and off, so turning it on moves nothing for a design with no blocks.
func TestFlatEDIFReadsIdenticallyFlattened(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "*.edn"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	compared := 0
	for _, path := range names {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		off, err := Read(bytes.NewReader(b), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(off.GetInputDiagnostics().GetUnexpandedHierarchy()) > 0 {
			continue
		}
		on, err := ReadWith(bytes.NewReader(b), name, Options{FlattenHierarchy: true})
		if err != nil {
			t.Fatalf("%s flattened: %v", name, err)
		}
		if !proto.Equal(off, on) {
			t.Errorf("%s reads differently flattened", name)
		}
		compared++
	}
	if compared < 10 {
		t.Errorf("compared %d flat fixtures, want at least 10", compared)
	}
}

// TestFlattenedDesignWritesAndReadsBack holds the writer to a flattened read, whose ref-des and
// block-local net names carry "/", so the emitted netlist re-reads to the same partition.
func TestFlattenedDesignWritesAndReadsBack(t *testing.T) {
	d := readFlat(t, "hier_blocks.edn")
	var b bytes.Buffer
	if err := WriteNetlist(&b, d); err != nil {
		t.Fatal(err)
	}
	back, err := Read(&b, "written.edn")
	if err != nil {
		t.Fatal(err)
	}
	want, got := partition(d), partition(back)
	if len(got) != len(want) {
		t.Errorf("re-read connects %d pins, want %d", len(got), len(want))
	}
	for pin, net := range want {
		if got[pin] != net {
			t.Errorf("re-read puts %s on %q, want %q", pin, got[pin], net)
		}
	}
}

package ipc2581

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// summarize renders the modeled IR (netlist + physical tier) as a canonical, order-independent
// string, so two designs that are semantically equal produce identical summaries regardless of
// ordering. It is the oracle's comparison key.
func summarize(d *ir.Design) string {
	var lines []string
	for _, c := range d.Components {
		lines = append(lines, fmt.Sprintf("component %s fp=%s", c.RefDes, c.FootprintRef))
	}
	for _, fp := range d.Footprints {
		lines = append(lines, "footprint "+fp.Name)
	}
	for _, n := range d.Nets {
		var conns []string
		for _, cn := range n.Connections {
			conns = append(conns, cn.ComponentRef+"."+cn.PinRef)
		}
		sort.Strings(conns)
		lines = append(lines, fmt.Sprintf("net %s = %s", n.Name, strings.Join(conns, ",")))
	}
	for _, l := range d.Layers {
		lines = append(lines, fmt.Sprintf("layer %s fn=%v", l.Name, l.Function))
	}
	if d.Stackup != nil {
		for _, sl := range d.Stackup.Layers {
			lines = append(lines, fmt.Sprintf("stackup %s %dnm %s", sl.LayerRef, sl.ThicknessNm, sl.Material))
		}
	}
	for _, b := range d.Bom {
		rd := append([]string(nil), b.RefDes...)
		sort.Strings(rd)
		lines = append(lines, fmt.Sprintf("bom %s %d [%s]", b.Mpn, b.Quantity, strings.Join(rd, ",")))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestRoundTrip is the semantic round-trip oracle: read -> IR -> Write -> read -> IR', and the
// two IRs must be equal on every modeled field. Confirms the reader and emitter agree.
func TestRoundTrip(t *testing.T) {
	d1, err := Read(bytes.NewReader(readFixture(t, "board.xml")), "board.xml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	var buf bytes.Buffer
	if err := Write(&buf, d1); err != nil {
		t.Fatalf("Write: %v", err)
	}

	d2, err := Read(bytes.NewReader(buf.Bytes()), "roundtrip.xml")
	if err != nil {
		t.Fatalf("re-Read emitted XML: %v\n---emitted---\n%s", err, buf.String())
	}

	if s1, s2 := summarize(d1), summarize(d2); s1 != s2 {
		t.Errorf("round-trip changed the IR:\n--- before ---\n%s\n--- after ---\n%s", s1, s2)
	}

	// Sanity: the emitted document is a real IPC-2581 the reader accepts, with content.
	if len(d2.Components) == 0 || len(d2.Nets) == 0 || len(d2.Layers) == 0 {
		t.Errorf("round-tripped design is missing modeled content: %d comps, %d nets, %d layers",
			len(d2.Components), len(d2.Nets), len(d2.Layers))
	}
}

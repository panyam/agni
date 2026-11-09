package kicad

import (
	"bytes"
	"slices"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// busNames reads bus-names.kicad_sch and returns each drawn bus piece's name by its uuid, in the
// order drawn, and each entry's name by its uuid.
func busNames(t *testing.T) (trunks map[string][]string, entries map[string]string) {
	t.Helper()
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "bus-names.kicad_sch")), "bus-names.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	trunks, entries = map[string][]string{}, map[string]string{}
	for _, w := range g.GetSheets()[0].GetWires() {
		id := w.GetProv().GetSourceId()
		switch w.GetKind() {
		case geom.WireGeometry_KIND_BUS:
			trunks[id] = append(trunks[id], w.GetNet())
		case geom.WireGeometry_KIND_BUS_ENTRY:
			entries[id] = w.GetNet()
		}
	}
	if len(trunks) != 6 || len(entries) != 3 {
		t.Fatalf("read %d trunks and %d entries, want 6 and 3", len(trunks), len(entries))
	}
	return trunks, entries
}

// A drawn bus is named for the bus it carries, so a click selects it and a bus finding lights it
// (agni issue 1435). A label on one segment names every segment connected to it, and an entry takes
// the name of the bus it touches.
func TestABusLabelNamesEveryConnectedSegment(t *testing.T) {
	trunks, entries := busNames(t)
	for _, id := range []string{"vec-a", "vec-b", "vec-c"} {
		if !slices.Equal(trunks[id], []string{"DATA[0..3]"}) {
			t.Errorf("%s is named %q, want DATA[0..3]", id, trunks[id])
		}
	}
	if entries["vec-entry"] != "DATA[0..3]" {
		t.Errorf("the entry on the vector bus is named %q, want DATA[0..3]", entries["vec-entry"])
	}
}

// A group bus is named by its label, here a hierarchical one, as Jetson names every bus it draws.
func TestAGroupBusTakesItsHierarchicalLabel(t *testing.T) {
	trunks, entries := busNames(t)
	if !slices.Equal(trunks["grp"], []string{"I2C0{I2C}"}) || entries["grp-entry"] != "I2C0{I2C}" {
		t.Errorf("the group bus is named %q and its entry %q, want I2C0{I2C} for both", trunks["grp"], entries["grp-entry"])
	}
}

// A trunk carrying two labelled branches names each for its own label, as the net solver names the
// branch under a sheet pin. The stretch both labels reach at once names neither, and a bus no label
// reaches stays unnamed, as KiCad leaves it.
func TestEachBranchTakesItsNearestLabel(t *testing.T) {
	trunks, entries := busNames(t)
	if !slices.Equal(trunks["two"], []string{"A[0..1]", "", "B[0..1]"}) {
		t.Errorf("the two-label trunk is named %q, want A[0..1], none, B[0..1]", trunks["two"])
	}
	if !slices.Equal(trunks["bare"], []string{""}) || entries["bare-entry"] != "" {
		t.Errorf("the unlabelled bus is named %q and its entry %q, want none", trunks["bare"], entries["bare-entry"])
	}
}

// A group bus records the nets it carries, PREFIX.MEMBER, so Properties lists them for a clicked bus.
func TestAGroupBusRecordsItsMembers(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "bus-names.kicad_sch")), "bus-names.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	groups := d.GetInputDiagnostics().GetGroupBuses()
	if len(groups) != 1 || groups[0].GetLabel() != "I2C0{I2C}" || !slices.Equal(groups[0].GetMembers(), []string{"I2C0.SDA", "I2C0.SCL"}) {
		t.Errorf("group buses = %v, want I2C0{I2C} carrying I2C0.SDA and I2C0.SCL", groups)
	}
	for _, b := range d.GetInputDiagnostics().GetUnmodeledBuses() {
		if b.GetLabel() == "I2C0{I2C}" {
			t.Error("the group bus is listed for bus-not-modeled, which must not judge one (agni issue 939)")
		}
	}
}

// A member broken out on one sheet is often carried by a net another sheet names. It takes that net's
// name, and a member no net carries keeps its own.
func TestAMemberTakesTheNameOfTheNetCarryingIt(t *testing.T) {
	groups := []*ir.GroupBus{{Label: "U{UART}", Members: []string{"/a/U.TX", "/a/U.RX", "/a/U.CTS"}}}
	carriedBy(groups, []netgraph.Net{
		{Name: "/b/U.TX", Aliases: []netgraph.Alias{{Name: "/a/U.TX"}, {Name: "/b/U.TX"}}},
		{Name: "/b/U.RX", Aliases: []netgraph.Alias{{Name: "/a/U.RX"}}},
		{Name: "/c/U.RX", Aliases: []netgraph.Alias{{Name: "/a/U.RX"}}},
	})
	if want := []string{"/b/U.TX", "/a/U.RX", "/a/U.CTS"}; !slices.Equal(groups[0].Members, want) {
		t.Errorf("members = %v, want %v (a name two nets share stays as it was)", groups[0].Members, want)
	}
}

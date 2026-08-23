package kicad

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestReadSchematic(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "sch.kicad_sch")), "test.kicad_sch")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	if d.SourceFormat != "kicad-sch" || d.IrVersion != "0" {
		t.Errorf("source_format=%q ir_version=%q, want kicad-sch / 0", d.SourceFormat, d.IrVersion)
	}

	// Part-type library: DUAL, R, GND (definitions are kept even for the virtual symbol).
	if len(d.Libraries) != 1 {
		t.Fatalf("libraries = %d, want 1", len(d.Libraries))
	}
	dual := findPartType(d, "test:DUAL")
	if dual == nil {
		t.Fatal("part type test:DUAL not found")
	}
	if dual.DesignatorPrefix != "U" {
		t.Errorf("DUAL designator_prefix = %q, want U", dual.DesignatorPrefix)
	}
	if len(dual.Pins) != 2 {
		t.Fatalf("DUAL pins = %d, want 2", len(dual.Pins))
	}
	if dir := pinDir(dual, "1"); dir != ir.PinDirection_PIN_DIRECTION_INPUT {
		t.Errorf("DUAL pin 1 direction = %v, want INPUT", dir)
	}
	if dir := pinDir(dual, "2"); dir != ir.PinDirection_PIN_DIRECTION_OUTPUT {
		t.Errorf("DUAL pin 2 direction = %v, want OUTPUT", dir)
	}

	// Components: U1 (2 sections from units 1+2) and R1 (1 section). #PWR01 skipped.
	if len(d.Components) != 2 {
		t.Fatalf("components = %d, want 2 (#PWR skipped)", len(d.Components))
	}
	if findComponent(d, "#PWR01") != nil {
		t.Error("#PWR01 should be skipped (virtual symbol)")
	}
	u1 := findComponent(d, "U1")
	if u1 == nil {
		t.Fatal("component U1 not found")
	}
	if len(u1.Sections) != 2 {
		t.Fatalf("U1 sections = %d, want 2", len(u1.Sections))
	}
	for _, s := range u1.Sections {
		if s.PartRef != "test:DUAL" {
			t.Errorf("U1 section part_ref = %q, want test:DUAL", s.PartRef)
		}
	}
	if r1 := findComponent(d, "R1"); r1 == nil || len(r1.Sections) != 1 {
		t.Errorf("R1 = %v, want 1 section", r1)
	}

	// Hierarchy: the sub-sheet reference is recorded.
	if len(d.Sheets) != 1 {
		t.Errorf("sheets = %d, want 1", len(d.Sheets))
	}
}

func findPartType(d *ir.Design, name string) *ir.PartType {
	for _, lib := range d.Libraries {
		for _, p := range lib.Parts {
			if p.Name == name {
				return p
			}
		}
	}
	return nil
}

func pinDir(pt *ir.PartType, designator string) ir.PinDirection {
	for _, p := range pt.Pins {
		if p.Designator == designator {
			return p.Direction
		}
	}
	return ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
}

// TestReadSchematicNets checks net extraction from schematic geometry (WS1-010). Before this the
// reader produced zero nets, so every connectivity rule saw all components unconnected. Both
// fixtures' connectivity was cross-checked against `kicad-cli sch export netlist`.
// MPN and Manufacturer symbol properties land in attributes (the WS10-003 join key
// fallback when no BomLine exists); other user properties are deliberately not swept.
func TestReadSchematicPartIdentityProperties(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "sch.kicad_sch")), "sch.kicad_sch")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	var r1 *ir.Component
	for _, c := range d.Components {
		if c.RefDes == "R1" {
			r1 = c
		}
	}
	if r1 == nil {
		t.Fatal("R1 not found")
	}
	if got := r1.Attributes["MPN"]; got != "RC0603FR-0710KL" {
		t.Errorf(`R1 MPN attribute = %q, want RC0603FR-0710KL`, got)
	}
	if got := r1.Attributes["Manufacturer"]; got != "Yageo" {
		t.Errorf(`R1 Manufacturer attribute = %q, want Yageo`, got)
	}
}

// TestReadSchematicFabricationAttributes covers WS1-037: the dnp/in_bom/on_board fabrication
// tokens and the Footprint/Datasheet properties reach the component attributes so a check, a
// diff, or the WS10 datasheet join can see them. R1 is placed do-not-populate and out of both
// BOM and assembly scope.
func TestReadSchematicFabricationAttributes(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "sch.kicad_sch")), "sch.kicad_sch")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	var r1 *ir.Component
	for _, c := range d.Components {
		if c.RefDes == "R1" {
			r1 = c
		}
	}
	if r1 == nil {
		t.Fatal("R1 not found")
	}
	want := map[string]string{
		"dnp":       "yes",
		"in_bom":    "no",
		"on_board":  "no",
		"Footprint": "Resistor_SMD:R_0603_1608Metric",
		"Datasheet": "https://example.com/r.pdf",
	}
	for k, v := range want {
		if got := r1.Attributes[k]; got != v {
			t.Errorf("R1 attribute %q = %q, want %q", k, got, v)
		}
		if got := r1.Sections[0].Attributes[k]; got != v {
			t.Errorf("R1 section attribute %q = %q, want %q", k, got, v)
		}
	}
}

func TestReadSchematicNets(t *testing.T) {
	// sch.kicad_sch: the four component pins land on one net — a net a rule can actually traverse.
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "sch.kicad_sch")), "sch.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Nets) != 1 {
		t.Fatalf("sch.kicad_sch nets = %d, want 1", len(d.Nets))
	}
	// The #PWR symbol's pin is now a typed VIRTUAL connection (WS1-014): it joins the
	// net's members carrying its power direction as a connection attribute, while the
	// symbol itself stays out of Components.
	if got := connKeys(d.Nets[0]); !equalStrs(got, []string{"#PWR01.1", "R1.1", "R1.2", "U1.1", "U1.2"}) {
		t.Errorf("net connections = %v, want [#PWR01.1 R1.1 R1.2 U1.1 U1.2]", got)
	}
	for _, c := range d.Nets[0].Connections {
		if c.ComponentRef == "#PWR01" {
			if c.Attributes["direction"] != "power_in" {
				t.Errorf("#PWR01.1 direction attribute = %q, want power_in", c.Attributes["direction"])
			}
		} else if len(c.Attributes) != 0 {
			t.Errorf("real connection %s.%s grew attributes: %v", c.ComponentRef, c.PinRef, c.Attributes)
		}
	}
	for _, comp := range d.Components {
		if comp.RefDes == "#PWR01" {
			t.Error("virtual power symbol entered Components")
		}
	}

	// geom.kicad_sch has a symbol rotated 270 deg. The placement transform must use KiCad's raw
	// angle (see pinTransform): with the render-frame angle the rotated pins land swapped, which
	// mis-groups connectivity. Regression guard: this fixture yielded 4 nets (incl. a spurious
	// label-only net) before the fix; kicad-cli confirms 3.
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "geom.kicad_sch")), "geom.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nets) != 3 {
		t.Errorf("geom.kicad_sch nets = %d, want 3", len(g.Nets))
	}
}

// TestReadSchematicDangling checks the reader surfaces wire endpoints that terminate on nothing,
// and only those: a two-wire corner (degree 2), an endpoint on a junction dot, and one on a label
// anchor are all connected, not dangling. Points are in the geometry frame (nm, Y-up), and each
// dangle carries its wire uuid.
func TestReadSchematicDangling(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "dangling.kicad_sch")), "dangling.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	type pt struct{ x, y int64 }
	got := map[pt]string{}
	for _, e := range d.GetInputDiagnostics().GetDanglingEndpoints() {
		got[pt{e.X, e.Y}] = e.Prov.GetNativeId()
	}
	want := map[pt]string{
		{0, 0}:                "w1", // floats
		{10000000, -10000000}: "w2", // corner's far end floats
		{20000000, 0}:         "w3", // floats; its other end is on the junction dot
		{30000000, 0}:         "w4", // floats; its other end is on the SIG label
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dangling endpoints = %v, want %v", got, want)
	}
}

// TestReadSchematicRefDesCollision checks the reader flags a genuine duplicate (two symbols
// claiming U1 unit 1) as a RefDesCollision with both placements' uuids, while a distinct ref-des is
// clean and a legitimate multi-unit part (sch.kicad_sch's U1, units 1+2) does not collide.
func TestReadSchematicRefDesCollision(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "dup_refdes.kicad_sch")), "dup.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	cols := d.GetInputDiagnostics().GetRefDesCollisions()
	if len(cols) != 1 || cols[0].RefDes != "U1" {
		t.Fatalf("collisions = %+v, want one for U1", cols)
	}
	ids := map[string]bool{}
	for _, p := range cols[0].Instances {
		ids[p.GetNativeId()] = true
	}
	if !ids["aaa"] || !ids["bbb"] {
		t.Errorf("collision instances = %v, want the two colliding uuids aaa+bbb", ids)
	}

	clean, err := ReadSchematic(bytes.NewReader(readFixture(t, "sch.kicad_sch")), "sch.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if c := clean.GetInputDiagnostics().GetRefDesCollisions(); len(c) != 0 {
		t.Errorf("multi-unit U1 must not collide, got %+v", c)
	}
}

// TestReadSchematicUnannotatedComponents (agni issue 311): a KiCad schematic KEEPS its
// placeholder-designated symbols, so it is the layer that has to say they are unannotated.
// One entry per placeholder rather than per part, carrying each placement, because "2 parts are
// still called R?" is the reviewable fact.
//
// The same fixture pins the collision boundary: two symbols sharing "R?" are not two parts
// claiming one designator, so they must not ALSO be reported as a duplicate ref-des. That would
// be an error-severity finding whose remedy (rename one) is wrong for a sheet nobody has
// annotated yet, over the same two placements.
func TestReadSchematicUnannotatedComponents(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "unannotated.kicad_sch")), "unannotated.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	byRef := map[string][]string{}
	for _, u := range d.GetInputDiagnostics().GetUnannotatedComponents() {
		for _, p := range u.GetInstances() {
			byRef[u.GetRefDes()] = append(byRef[u.GetRefDes()], p.GetNativeId())
		}
	}
	if len(byRef) != 2 {
		t.Fatalf("unannotated designators = %v, want R? and C?1845", byRef)
	}
	if got := byRef["R?"]; !equalStrs(got, []string{"r-un-1", "r-un-2"}) {
		t.Errorf("R? placements = %v, want both symbols' uuids", got)
	}
	if got := byRef["C?1845"]; !equalStrs(got, []string{"c-un-1"}) {
		t.Errorf("C?1845 placements = %v, want the one symbol's uuid", got)
	}
	// The parts are kept, not skipped: they are real circuitry that is merely unnamed, and
	// dropping them would make the design read short.
	kept := map[string]bool{}
	for _, c := range d.Components {
		kept[c.RefDes] = true
	}
	for _, want := range []string{"R?", "C?1845", "R1"} {
		if !kept[want] {
			t.Errorf("component %q was dropped; a schematic keeps unannotated parts", want)
		}
	}
	if c := d.GetInputDiagnostics().GetRefDesCollisions(); len(c) != 0 {
		t.Errorf("unannotated parts must not report as a ref-des collision, got %+v", c)
	}
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestNoConnectMarkerNamesPinStub (WS1-019): a no_connect marker on a pin's connect
// point makes that pin's lone stub synthesize with the tool-marker name instead of N$,
// so no-connect awareness (single-pin-net's skip, the NC channel) keys on it.
func TestNoConnectMarkerNamesPinStub(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "ncmark.kicad_sch")), "ncmark.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, n := range d.Nets {
		names[n.Name] = true
	}
	for _, want := range []string{"SIG", "unconnected-(U1-Pad2)", "unconnected-(R1-Pad2)"} {
		if !names[want] {
			t.Errorf("missing net %q; have %v", want, names)
		}
	}
	for n := range names {
		if strings.HasPrefix(n, "N$") {
			t.Errorf("bare stub %q remains; NC-marked pins should not synthesize N$ nets", n)
		}
	}
}

// TestSymbolRefPrefersInstances (WS1-020): the instances block's per-project reference is
// post-annotation truth and beats the Reference property — an unannotated "R?" property
// resolves to the instance's "R5", and a stale property ("R1") yields to the instance's
// "R7". A symbol with no instances block keeps the property.
func TestSymbolRefPrefersInstances(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "instref.kicad_sch")), "instref.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, c := range d.Components {
		got[c.RefDes] = true
	}
	for _, want := range []string{"R5", "R7", "R2"} {
		if !got[want] {
			t.Errorf("missing component %q; have %v", want, got)
		}
	}
	if got["R1"] {
		t.Errorf("property-derived ref %q survived; instances should win", "R1")
	}
	// An instances entry that is itself a placeholder is not post-annotation truth, so it is
	// passed over like the "R?" property form. "R?1845" is the partly-assigned shape a
	// suffix-only predicate reads as a real designator (agni issue 311): the symbol must land
	// on the property's "R?" and stay visibly unannotated, not acquire "R?1845" as an identity.
	if got["R?1845"] {
		t.Errorf("partly-assigned instance ref %q was taken as an identity; have %v", "R?1845", got)
	}
	if !got["R?"] {
		t.Errorf("the unannotated symbol should fall back to its %q property; have %v", "R?", got)
	}
}

// TestNoJunctionEndpoint (WS1-012): a wire endpoint mid-span on another wire's body with
// no junction dot emits the no-junction diagnostic (sheet frame, tap wire's uuid) and is
// NOT double-reported as dangling — the endpoint touches something, the wrong way.
func TestNoJunctionEndpoint(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "tjunc.kicad_sch")), "tjunc.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	nj := d.GetInputDiagnostics().GetNoJunctionEndpoints()
	if len(nj) != 1 {
		t.Fatalf("no-junction endpoints = %d, want 1: %+v", len(nj), nj)
	}
	if nj[0].X != 65000000 || nj[0].Y != -50000000 || nj[0].Prov.GetNativeId() != "wtap" {
		t.Errorf("endpoint = (%d,%d) wire %q, want (65000000,-50000000) wtap", nj[0].X, nj[0].Y, nj[0].Prov.GetNativeId())
	}
	for _, dg := range d.GetInputDiagnostics().GetDanglingEndpoints() {
		if dg.X == nj[0].X && dg.Y == nj[0].Y {
			t.Error("the on-body endpoint must not also report as dangling")
		}
	}
}

// TestJoinedTapRecorded (agni issue 420): the same T-tap with a junction dot on it is recorded as a
// JOINED tap rather than vanishing. It used to vanish, and that is the whole point: splitWiresAt runs
// at the dot before the detection pass, so a joined tap is an endpoint of both wires by the time
// anything looks and is indistinguishable from a point where no wire ever crossed. A schematic whose
// every tap carried its dot then reported what a schematic with no tap in it reported.
//
// The dot fixture and the dotless one differ by exactly one line, so the pass and the fail are the
// same geometry under one changed fact.
func TestJoinedTapRecorded(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "tjunc_dotted.kicad_sch")), "tjunc_dotted.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	diag := d.GetInputDiagnostics()
	if !slices.Contains(diag.GetSupplied(), "junction_taps") {
		t.Fatal("junction_taps not declared, so a consumer cannot tell a clean sheet from a format that never looked at wire geometry")
	}
	if nj := diag.GetNoJunctionEndpoints(); len(nj) != 0 {
		t.Errorf("no-junction endpoints = %+v, want none once the dot is placed", nj)
	}
	jt := diag.GetJoinedTaps()
	if len(jt) != 1 {
		t.Fatalf("joined taps = %d, want 1: %+v", len(jt), jt)
	}
	if jt[0].GetX() != 65000000 || jt[0].GetY() != -50000000 {
		t.Errorf("joined tap at (%d,%d), want the same point the dotless fixture fails on", jt[0].GetX(), jt[0].GetY())
	}
	if jt[0].GetJoinKind() != "junction" {
		t.Errorf("join kind = %q, want junction", jt[0].GetJoinKind())
	}
	// Three, not two: the split cuts the through-wire at the dot, so the tap meets two halves.
	if jt[0].GetSegments() != 3 {
		t.Errorf("segments = %d, want 3 (both halves of the through-wire plus the tap)", jt[0].GetSegments())
	}
	if jt[0].GetProv().GetNativeId() != "wtap" {
		t.Errorf("prov native id = %q, want the tap wire's uuid", jt[0].GetProv().GetNativeId())
	}
}

// TestJoinedTapByLabel: a mid-span LABEL joins the wires too, and it is the case worth telling apart
// from a dot. KiCad connects there just as firmly, but nobody placed a connection symbol, so the join
// is a side effect of naming the net and is much easier to delete by accident. The label TEXT is the
// net the tap resolves to, which is what a reviewer opens the schematic to confirm.
func TestJoinedTapByLabel(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "tjunc_labeled.kicad_sch")), "tjunc_labeled.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	jt := d.GetInputDiagnostics().GetJoinedTaps()
	if len(jt) != 1 {
		t.Fatalf("joined taps = %d, want 1: %+v", len(jt), jt)
	}
	if jt[0].GetJoinKind() != "label" || jt[0].GetLabel() != "BUS" {
		t.Errorf("join = %q %q, want label BUS", jt[0].GetJoinKind(), jt[0].GetLabel())
	}
}

// TestJoinedAndSilentTapsArePartition: the two lists are one partition of the same detection, run at
// two points in the pipeline. A point on both would let the rule report it as passed and failed at
// once, and the two runs are far enough apart in the reader for that to happen quietly.
func TestJoinedAndSilentTapsArePartition(t *testing.T) {
	for _, f := range []string{"tjunc.kicad_sch", "tjunc_dotted.kicad_sch", "tjunc_labeled.kicad_sch", "sch.kicad_sch", "hier_root.kicad_sch"} {
		d, err := ReadSchematic(bytes.NewReader(readFixture(t, f)), f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		diag := d.GetInputDiagnostics()
		joined := map[[2]int64]bool{}
		for _, j := range diag.GetJoinedTaps() {
			joined[[2]int64{j.GetX(), j.GetY()}] = true
		}
		for _, n := range diag.GetNoJunctionEndpoints() {
			if joined[[2]int64{n.GetX(), n.GetY()}] {
				t.Errorf("%s: the tap at (%d,%d) is on both lists", f, n.GetX(), n.GetY())
			}
		}
	}
}

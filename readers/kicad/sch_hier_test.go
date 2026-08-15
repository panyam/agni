package kicad

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/internal/netgraph"
)

func hierOpen(t *testing.T) func(string) ([]byte, error) {
	return func(rel string) ([]byte, error) {
		if strings.Contains(rel, "/") {
			return nil, fmt.Errorf("opener got a non-relative path %q", rel)
		}
		return readFixture(t, rel), nil
	}
}

// TestReadSchematicHierarchyNets pins the whole-design semantics of the netlist walk
// (WS1-018) on a two-sheet fixture whose child file is instantiated twice:
//   - per-instance ref-des resolution (the instances paths give R101/R102 in amp1 and
//     R201/R202 in amp2 from ONE file),
//   - local-label scoping ("SIG" exists on the root and in both instances as three
//     separate nets, KiCad-qualified),
//   - hierarchical label <-> sheet pin binding per instance edge (root wire joins
//     /amp1/CTRL; amp2's port is unwired on the parent so /amp2/CTRL stays child-only),
//   - design-wide rail unification (VCC power symbols on all three instances are one
//     net, PWR_FLAG-driven, with the WS1-014 virtual pins deduped),
//   - hierarchical sheet ids matching the geometry walk, and
//   - dangling diagnostics translated back to sheet-frame coordinates with the child
//     file as their source.
func TestReadSchematicHierarchyNets(t *testing.T) {
	d, complete, err := ReadSchematicHierarchyNets("hier_root.kicad_sch", readFixture(t, "hier_root.kicad_sch"), hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Error("both sub-sheets open, walk must report complete")
	}

	var refs []string
	for _, c := range d.Components {
		refs = append(refs, c.RefDes)
	}
	sort.Strings(refs)
	// MH? is the child sheet's unannotated mounting hole: pinless, so it joins no net, and
	// unannotated, so both sheet placements merge under the one placeholder.
	if got, want := strings.Join(refs, " "), "MH? R1 R101 R102 R201 R202"; got != want {
		t.Errorf("components = %q, want %q", got, want)
	}
	// The hierarchical walk assembles its own InputDiagnostics, so the unannotated diagnostic
	// has to be wired there too — an unwired one is a silent no-op that reads as a clean design.
	un := d.GetInputDiagnostics().GetUnannotatedComponents()
	if len(un) != 1 || un[0].GetRefDes() != "MH?" || len(un[0].GetInstances()) != 2 {
		t.Errorf("unannotated components = %+v, want one MH? entry carrying both sheet placements", un)
	}

	nets := map[string]string{}
	for _, n := range d.Nets {
		nets[n.Name] = strings.Join(connKeys(n), " ")
	}
	want := map[string]string{
		"SIG":        "R1.2",
		"/amp1/SIG":  "R101.2 R102.2",
		"/amp2/SIG":  "R201.2 R202.2",
		"/amp1/CTRL": "R1.1 R101.1",
		"/amp2/CTRL": "R201.1",
		"VCC":        "#FLG01.1 #PWR01.1 #PWR02.1 R102.1 R202.1",
	}
	for name, conns := range want {
		if nets[name] != conns {
			t.Errorf("net %q = %q, want %q (have %v)", name, nets[name], conns, nets)
		}
	}
	if len(nets) != len(want) {
		t.Errorf("nets = %v, want exactly %d", nets, len(want))
	}

	for _, n := range d.Nets {
		if n.Name == "VCC" {
			if n.Attributes["power_driven"] != "true" {
				t.Error("VCC carries a PWR_FLAG, want power_driven")
			}
			if n.Attributes["external"] != "true" {
				t.Error("bare hierarchy read keeps VCC external (this root may be someone's sub-sheet)")
			}
		}
		if strings.Contains(n.Name, "CTRL") && n.Attributes["external"] == "true" {
			t.Errorf("bound hierarchical port %q must not stay external", n.Name)
		}
	}

	var ids []string
	for _, s := range d.Sheets {
		ids = append(ids, s.Id)
	}
	if got, want := strings.Join(ids, " "), "/ /amp1 /amp2"; got != want {
		t.Errorf("sheet ids = %q, want %q (geometry-walk identity)", got, want)
	}

	// Net->sheet membership (WS9-028): the walk attributes each net the sheet instances it
	// touches, in sheet order, so a net-subject finding gets a sheet badge. A purely local
	// sub-sheet net lists only its instance; a hierarchical port also lists the parent whose
	// sheet pin sits on it (even /amp2/CTRL, wired on no parent wire, touches the root where its
	// port symbol lives); the design-wide rail lists every instance. A single-sheet design would
	// carry no attribute (guarded below two sheets).
	wantSheets := map[string]string{
		"SIG":        "/",
		"/amp1/SIG":  "/amp1",
		"/amp2/SIG":  "/amp2",
		"/amp1/CTRL": "/ /amp1",
		"/amp2/CTRL": "/ /amp2",
		"VCC":        "/ /amp1 /amp2",
	}
	for _, n := range d.Nets {
		got := strings.Join(netgraph.ParseSheets(n.Attributes[netgraph.AttrSheets]), " ")
		if want := wantSheets[n.Name]; got != want {
			t.Errorf("net %q sheets = %q, want %q", n.Name, got, want)
		}
	}

	// The child's stray wire dangles at both ends, once per instance, in SHEET-frame
	// coordinates (the offset bands must not leak into diagnostics).
	dangles := d.GetInputDiagnostics().GetDanglingEndpoints()
	if len(dangles) != 4 {
		t.Fatalf("dangles = %d, want 4 (2 ends x 2 instances)", len(dangles))
	}
	for _, dg := range dangles {
		if dg.Prov.GetSourceFile() != "hier_child.kicad_sch" {
			t.Errorf("dangle source = %q, want hier_child.kicad_sch", dg.Prov.GetSourceFile())
		}
		if dg.X != 200000000 && dg.X != 205000000 {
			t.Errorf("dangle X = %d, want sheet-frame 200/205mm", dg.X)
		}
	}
}

// TestReadProjectResolvesWalkedHierarchy: a complete walk is the WS1-017 completeness
// witness, so the project read downgrades the rail's external marking to global; the
// bare hierarchy read above keeps it.
func TestReadProjectResolvesWalkedHierarchy(t *testing.T) {
	d, err := ReadProject(strings.NewReader(string(readFixture(t, "hier_root.kicad_sch"))), nil,
		"hier_root.kicad_sch", "", hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range d.Nets {
		if n.Name == "VCC" {
			if n.Attributes["external"] == "true" || n.Attributes["global"] != "true" {
				t.Errorf("VCC attrs = %v, want external resolved to global (complete walk)", n.Attributes)
			}
			return
		}
	}
	t.Fatal("VCC net not found")
}

// TestReadProjectPartialWalkStaysExternal: a sub-sheet that fails to open leaves the
// design partial, so external markings must survive.
func TestReadProjectPartialWalkStaysExternal(t *testing.T) {
	failOpen := func(string) ([]byte, error) { return nil, fmt.Errorf("gone") }
	d, err := ReadProject(strings.NewReader(string(readFixture(t, "hier_root.kicad_sch"))), nil,
		"hier_root.kicad_sch", "", failOpen)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range d.Nets {
		if n.Name == "VCC" {
			if n.Attributes["external"] != "true" {
				t.Errorf("VCC attrs = %v, want external kept (partial walk)", n.Attributes)
			}
			return
		}
	}
	t.Fatal("VCC net not found")
}

// TestLabelBindsMidSpan pins the KiCad connection-point rules (verified against
// kicad-cli): a label anywhere ALONG a wire names and joins that wire's net; a pin whose
// connect point lies mid-span does NOT connect (only a wire END on the pin joins); and a
// second same-named label connects by name.
func TestLabelBindsMidSpan(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "midspan.kicad_sch")), "midspan.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range d.Nets {
		conns := strings.Join(connKeys(n), " ")
		if n.Name == "MID" && conns != "R1.2 R2.2" {
			t.Errorf("MID = %q, want R1.2 R2.2 (mid-span label binds; mid-span pin R3.1 does not)", conns)
		}
		for _, c := range n.Connections {
			if c.ComponentRef == "R3" && len(n.Connections) > 1 {
				t.Errorf("R3.%s joined %q: a pin mid-span on a wire must not connect", c.PinRef, n.Name)
			}
		}
	}
}

// TestNetNameUnescape: KiCad stores "/" in a net name as {slash} and unescapes on load,
// so labels in either spelling are one net under the unescaped name.
func TestNetNameUnescape(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, "escapes.kicad_sch")), "escapes.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range d.Nets {
		if n.Name == "VPP/MCLR" {
			if got := strings.Join(connKeys(n), " "); got != "R1.2 R2.2" {
				t.Errorf("VPP/MCLR = %q, want both spellings' wires joined", got)
			}
			return
		}
	}
	t.Fatalf("no VPP/MCLR net; nets: %v", d.Nets)
}

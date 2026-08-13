package formats

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/core/classify"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestSymbolOpenerRecursive: a --symbol-path pointed at a library ROOT resolves a symbol that
// lives in a subdirectory (gEDA/Lepton libs categorize symbols into analog/, power/, ...), and
// a top-level file in an earlier dir still wins over a subtree match.
func TestSymbolOpenerRecursive(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "analog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "analog", "resistor-1.sym"), []byte("SUBDIR"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A same-named top-level file in an earlier dir must take precedence over the subtree match.
	top := t.TempDir()
	if err := os.WriteFile(filepath.Join(top, "resistor-1.sym"), []byte("TOP"), 0o644); err != nil {
		t.Fatal(err)
	}

	open := (&Loader{SymbolPaths: []string{root}}).symbolOpener(filepath.Join(t.TempDir(), "x.sch"))
	data, err := open("resistor-1.sym")
	if err != nil || string(data) != "SUBDIR" {
		t.Fatalf("bare ref against a lib root = (%q, %v), want the subdir symbol", data, err)
	}
	if _, err := open("missing.sym"); err == nil {
		t.Error("missing symbol resolved; want an error")
	}

	openPrec := (&Loader{SymbolPaths: []string{top, root}}).symbolOpener(filepath.Join(t.TempDir(), "x.sch"))
	if data, err := openPrec("resistor-1.sym"); err != nil || string(data) != "TOP" {
		t.Errorf("precedence = (%q, %v), want the top-level file in the earlier dir (TOP)", data, err)
	}
}

// TestEDIFExtensionsRegistered pins that all three conventional EDIF-netlist suffixes resolve
// to the EDIF reader. Our fixtures use .edn, but real exports (and the corpus) use .edf/.edif;
// matching is case-insensitive, so .EDF resolves too. Regression guard for the corpus being
// invisible in the tree/CLI because only .edn was wired.
func TestEDIFExtensionsRegistered(t *testing.T) {
	for _, name := range []string{"x.edn", "x.edf", "x.edif", "x.EDF"} {
		f := ByExt(name)
		if f == nil || f.Name != "edif" || f.Design == nil {
			t.Errorf("%s: want the EDIF netlist reader, got %+v", name, f)
		}
	}
}

// TestRegistryConsistency guards the one-entry-per-format invariant: every entry has a
// label and at least one reader, and the derived capability sets stay in sync with the
// dispatch (an extension is readable iff the registry says so).
func TestRegistryConsistency(t *testing.T) {
	if len(byExt) == 0 {
		t.Fatal("empty format registry")
	}
	for ext, f := range byExt {
		if f.Ext != ext {
			t.Errorf("%s: registered under %q but Ext=%q", ext, ext, f.Ext)
		}
		if f.Name == "" {
			t.Errorf("%s: no UI label", ext)
		}
		if f.Design == nil && f.Geometry == nil {
			t.Errorf("%s: neither a netlist nor a geometry reader", ext)
		}
		if HasNetlist("x"+ext) != (f.Design != nil) || HasFaithful("x"+ext) != (f.Geometry != nil) {
			t.Errorf("%s: capability accessors disagree with the entry", ext)
		}
	}
}

// TestKicadProjectNetClass covers WS1-037: net-class membership lives only in the .kicad_pro
// net_settings (not the sch/pcb), so a project read stamps ir.Net.net_classes. It also covers the
// WS1-050 cardinality end to end: SIG is named by an explicit assignment AND matched by two
// patterns, and all three memberships must survive the loader, not just whichever resolved first.
// wirenet.kicad_sch yields a net named "SIG".
func TestKicadProjectNetClass(t *testing.T) {
	sch, err := os.ReadFile("../kicad/testdata/wirenet.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "board.kicad_sch"), sch, 0o644); err != nil {
		t.Fatal(err)
	}
	pro := `{"net_settings":{
	  "netclass_assignments": {"SIG": ["Critical"]},
	  "netclass_patterns": [
	    {"netclass":"HighSpeed","pattern":"SIG"},
	    {"netclass":"Diagnostic","pattern":"S*"}
	  ]}}`
	if err := os.WriteFile(filepath.Join(dir, "board.kicad_pro"), []byte(pro), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := (&Loader{}).ReadDesign(filepath.Join(dir, "board.kicad_pro"))
	if err != nil {
		t.Fatal(err)
	}
	var sig *ir.Net
	for _, n := range d.Nets {
		if n.Name == "SIG" {
			sig = n
		}
	}
	if sig == nil {
		t.Fatalf("SIG net not found in %v", d.Nets)
	}
	want := []string{"Critical", "Diagnostic", "HighSpeed"}
	if !slices.Equal(sig.NetClasses, want) {
		t.Errorf("SIG net_classes = %v, want %v (populated from .kicad_pro)", sig.NetClasses, want)
	}
}

// TestUnknownExtensionErrorListsAll pins the unknown-extension error to the generated
// extension list, so it can no longer drift from the registry (it used to omit .sch).
func TestUnknownExtensionErrorListsAll(t *testing.T) {
	_, err := (&Loader{}).ReadDesign("board.bogus")
	if err == nil {
		t.Fatal("want an error for an unknown extension")
	}
	// The exact generated list: substring checks would pass ".sch" via ".kicad_sch", which
	// is precisely the drift the old hand-written message had.
	want := "(have: " + strings.Join(NetlistExts(), ", ") + ")"
	if !strings.Contains(err.Error(), want) || !strings.Contains(want, ", .sch,") {
		t.Errorf("unknown-extension error = %v, want it to carry %q including a bare .sch entry", err, want)
	}
}

// TestReadDesignStampsDeviceClasses proves the ingestion classify pass runs inside ReadDesign
// (WS3-071): a design read through the Loader carries the normalized device_classes set with no
// separate call, so every format is classified at the same edge. basic.edn's R1/R2 read as resistors
// and U1 as an IC.
func TestReadDesignStampsDeviceClasses(t *testing.T) {
	d, err := (&Loader{}).ReadDesign("../edif/testdata/basic.edn")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := map[string]string{"R1": "resistor", "R2": "resistor", "U1": "ic"}
	got := map[string][]string{}
	for _, c := range d.Components {
		got[c.RefDes] = c.DeviceClasses
	}
	for ref, cls := range want {
		if len(got[ref]) != 1 || got[ref][0] != cls {
			t.Errorf("%s device_classes = %v, want [%s]", ref, got[ref], cls)
		}
	}
}

func TestFaithfulUnavailableSuggestsGrid(t *testing.T) {
	// Every non-geometry extension must fail faithful with a pointer to an auto-layout.
	for _, ext := range []string{".edn", ".xml", ".cvg", ".kicad_pcb", ".foo"} {
		err := faithfulUnavailable(ext)
		if err == nil || !strings.Contains(err.Error(), "--layout=grid") {
			t.Errorf("%s: want an error suggesting --layout=grid, got %v", ext, err)
		}
	}
}

func TestResolveGeometryRouting(t *testing.T) {
	// Auto-layout on a netlist works; faithful on the same netlist fails with guidance;
	// faithful on a geometry-bearing .eds works. Uses sibling-package fixtures.
	l := &Loader{}
	if g, err := l.ResolveGeometry("../edif/testdata/basic.edn", "grid", nil, SymbolsGlyph); err != nil || len(g.Sheets) != 1 {
		t.Errorf("grid on .edn: want 1 sheet no error, got sheets=%d err=%v", sheetCount(g), err)
	}
	if _, err := l.ResolveGeometry("../edif/testdata/basic.edn", "faithful", nil, SymbolsGlyph); err == nil || !strings.Contains(err.Error(), "--layout=grid") {
		t.Errorf("faithful on .edn: want guidance to --layout=grid, got %v", err)
	}
	if g, err := l.ResolveGeometry("../edif/testdata/sample.eds", "faithful", nil, SymbolsGlyph); err != nil || len(g.Sheets) == 0 {
		t.Errorf("faithful on .eds: want sheets no error, got sheets=%d err=%v", sheetCount(g), err)
	}
}

// An EDIF schematic (.eds) is dual-capability: its schematic view carries explicit netlist
// connectivity, so ReadDesign yields a netlist (queryable/checkable/diffable) AND the same file
// renders faithfully — the "query any format that carries a netlist" property, wired in the
// registry rather than gated by extension.
func TestEdsIsDualCapability(t *testing.T) {
	l := &Loader{}
	d, err := l.ReadDesign("../edif/testdata/sample.eds")
	if err != nil {
		t.Fatalf("ReadDesign(.eds): want a netlist, got err %v", err)
	}
	if len(d.GetComponents()) == 0 || len(d.GetNets()) == 0 {
		t.Errorf(".eds netlist empty: components=%d nets=%d, want both non-zero", len(d.GetComponents()), len(d.GetNets()))
	}
	if g, err := l.ResolveGeometry("../edif/testdata/sample.eds", "faithful", nil, SymbolsGlyph); err != nil || len(g.Sheets) == 0 {
		t.Errorf("same .eds must still render faithfully: sheets=%d err=%v", sheetCount(g), err)
	}
}

// TestResolveGeometryFaithfulSymbols covers partial-faithful mode (WS7-031): an auto-layout that
// draws the design's own symbols when they exist, and a helpful error when the format has none.
func TestResolveGeometryFaithfulSymbols(t *testing.T) {
	l := &Loader{}
	g, err := l.ResolveGeometry("../kicad/testdata/geom.kicad_sch", "grid", nil, SymbolsFaithful)
	if err != nil {
		t.Fatalf("faithful symbols on .kicad_sch: %v", err)
	}
	real := false
	for _, s := range g.GetSymbols() {
		if !strings.HasPrefix(s.GetCellRef(), "__node") { // synthetic glyphs/box are __node*
			real = true
		}
	}
	if !real {
		t.Error("faithful symbols: expected the design's own library symbol in the layout, got only synthetic")
	}

	// A netlist-only format has no symbols to be faithful to; faithful GRACEFULLY FALLS BACK to
	// glyphs rather than erroring, so a viewer that still has faithful selected from a previous
	// file draws the netlist graph instead of failing the request.
	g2, err := l.ResolveGeometry("../edif/testdata/basic.edn", "grid", nil, SymbolsFaithful)
	if err != nil {
		t.Fatalf("faithful on .edn should fall back to glyphs, not error: %v", err)
	}
	for _, s := range g2.GetSymbols() {
		if !strings.HasPrefix(s.GetCellRef(), "__node") {
			t.Errorf(".edn faithful fallback drew a non-synthetic symbol %q; want only glyphs", s.GetCellRef())
		}
	}
}

func sheetCount(g *geom.SchematicGeometry) int {
	if g == nil {
		return -1
	}
	return len(g.Sheets)
}

// TestIPCDeclaredNetRoleEndToEnd covers WS1-051 through the whole ingestion path: the ipc2581
// reader translates LogicalNet/@netClass into the role vocabulary, the loader's shared
// StampNetRoles pass unions it with the naming lexicon, and the result reaches ir.Net.roles —
// which is what every ground/rail-scoped rule reads. N$17 is the case that only works because the
// source declared it: no naming convention can recover a role from that name.
func TestIPCDeclaredNetRoleEndToEnd(t *testing.T) {
	d, err := (&Loader{}).ReadDesign("../ipc2581/testdata/board.xml")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"N$17":        {"ground"}, // declared only; the name says nothing
		"GND":         {"ground"}, // declared and name-derived agree, recorded once
		"VCC":         {"rail"},
		"SIGNAL_ONLY": nil, // netClass SIGNAL names no role, and neither does the name
	}
	for _, n := range d.Nets {
		w, tracked := want[n.Name]
		if !tracked {
			continue
		}
		if !slices.Equal(classify.RoleTokens(n), w) {
			t.Errorf("roles(%q) = %v, want %v", n.Name, classify.RoleTokens(n), w)
		}
		delete(want, n.Name)
	}
	if len(want) > 0 {
		t.Errorf("nets never seen in the read: %v", want)
	}
}

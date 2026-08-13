package formats

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// mountFS builds an in-memory fs.FS from real on-disk fixtures, keyed by the name the FS read
// will use. Every FS name here is deliberately one that does NOT exist relative to the test's
// working directory, so any read that still reaches the host filesystem fails outright instead of
// quietly succeeding — that is what makes the parity tests below evidence of the seam rather than
// evidence that both reads found the same file on disk.
func mountFS(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	for name, disk := range files {
		data, err := os.ReadFile(disk)
		if err != nil {
			t.Fatalf("fixture %s: %v", disk, err)
		}
		if _, err := os.Stat(name); err == nil {
			t.Fatalf("FS name %q also exists on disk; pick a name that cannot resolve via os", name)
		}
		m[name] = &fstest.MapFile{Data: data}
	}
	return m
}

// summarize renders the parts of a Design that must not depend on WHERE the bytes came from:
// components with their stamped classes, and every net with its sorted pin membership and
// tool-assigned classes. Provenance is deliberately excluded — a read is stamped with the name it
// was asked for, so the two reads legitimately differ there (TestReadDesignFSProvenance covers
// that separately).
func summarize(d *ir.Design) string {
	var b strings.Builder
	fmt.Fprintf(&b, "format=%s\n", d.SourceFormat)

	comps := make([]string, 0, len(d.Components))
	for _, c := range d.Components {
		comps = append(comps, c.RefDes+" ["+strings.Join(c.DeviceClasses, ",")+"]")
	}
	sort.Strings(comps)
	fmt.Fprintf(&b, "components(%d):\n  %s\n", len(comps), strings.Join(comps, "\n  "))

	nets := make([]string, 0, len(d.Nets))
	for _, n := range d.Nets {
		conns := make([]string, 0, len(n.Connections))
		for _, c := range n.Connections {
			conns = append(conns, c.ComponentRef+"."+c.PinRef)
		}
		sort.Strings(conns)
		classes := append([]string(nil), n.NetClasses...)
		sort.Strings(classes)
		roles := append([]string(nil), classify.RoleTokens(n)...)
		sort.Strings(roles)
		nets = append(nets, fmt.Sprintf("%s classes=%v roles=%v {%s}", n.Name, classes, roles, strings.Join(conns, ",")))
	}
	sort.Strings(nets)
	fmt.Fprintf(&b, "nets(%d):\n  %s\n", len(nets), strings.Join(nets, "\n  "))
	return b.String()
}

// TestReadDesignFSParity is the core WS1-049 acceptance: the same bytes read through an fs.FS
// produce the same design as an on-disk read, for every registered netlist format including both
// sniffed extensions (.sch picks its dialect by header, .xml must see an IPC-2581 root). The FS
// names are unreachable on disk, so this also proves no read silently fell back to os.
func TestReadDesignFSParity(t *testing.T) {
	cases := []struct {
		name   string
		disk   string
		fsName string
		// siblings are the files the design resolves BESIDE itself, mounted in the same FS
		// directory: an xschem/gEDA schematic gets its pins from .sym artwork in its own dir, so
		// mounting the schematic alone would compare a pin-bearing disk read against a pin-less FS
		// one and blame the seam for a missing fixture.
		siblings []string
	}{
		{name: "edif netlist", disk: "../edif/testdata/basic.edn", fsName: "designs/basic.edn"},
		{name: "edif schematic", disk: "../edif/testdata/sample.eds", fsName: "designs/sample.eds"},
		{name: "kicad schematic", disk: "../kicad/testdata/wirenet.kicad_sch", fsName: "designs/wirenet.kicad_sch"},
		{name: "kicad board", disk: "../kicad/testdata/board.kicad_pcb", fsName: "designs/board.kicad_pcb"},
		{name: "xschem sch", disk: "../xschem/testdata/divider.sch", fsName: "designs/divider.sch",
			siblings: []string{"../xschem/testdata/res.sym"}},
		{name: "geda sch", disk: "../geda/testdata/divider.sch", fsName: "designs/divider.sch",
			siblings: []string{"../geda/testdata/resistor.sym", "../geda/testdata/gnd.sym"}},
		{name: "ipc2581 xml", disk: "../ipc2581/testdata/board.xml", fsName: "designs/board.xml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			onDisk, err := (&Loader{}).ReadDesign(tc.disk)
			if err != nil {
				t.Fatalf("os read: %v", err)
			}
			// The xschem and gEDA fixtures share a basename, so each subtest gets its own FS.
			files := map[string]string{tc.fsName: tc.disk}
			for _, s := range tc.siblings {
				files["designs/"+filepath.Base(s)] = s
			}
			l := &Loader{FS: mountFS(t, files)}
			inMem, err := l.ReadDesign(tc.fsName)
			if err != nil {
				t.Fatalf("fs read: %v", err)
			}
			if got, want := summarize(inMem), summarize(onDisk); got != want {
				t.Errorf("fs read differs from os read\n--- fs ---\n%s\n--- os ---\n%s", got, want)
			}
			if len(inMem.Components) == 0 {
				t.Error("read produced no components; the fixture cannot witness parity")
			}
		})
	}
}

// TestReadDesignFSNoOSFallback pins the negative: with an FS installed, a name that exists ON DISK
// but not in the FS must fail. Without it, a missed call site would keep reading from the host and
// every parity test above would still pass, which is exactly the bug this seam exists to prevent.
func TestReadDesignFSNoOSFallback(t *testing.T) {
	const onDisk = "../edif/testdata/basic.edn"
	if _, err := (&Loader{}).ReadDesign(onDisk); err != nil {
		t.Fatalf("fixture must be readable from disk for this test to mean anything: %v", err)
	}
	l := &Loader{FS: fstest.MapFS{"designs/other.edn": &fstest.MapFile{Data: []byte("(edif)")}}}
	if _, err := l.ReadDesign(onDisk); err == nil {
		t.Error("a loader with an FS read a host path; reads must resolve against the FS only")
	}
}

// TestReadDesignFSStampsRunOnFSReads proves the FS entry reaches the SAME post-read pass sequence
// as the path entry, not a parallel one: the classify/net-role stamps are what a rule reads, and a
// second entry point that skipped them would produce a design that looks fine and checks wrong.
// basic.edn's R1/R2 classify as resistors, and VCC/GND carry rail/ground roles.
func TestReadDesignFSStampsRunOnFSReads(t *testing.T) {
	l := &Loader{FS: mountFS(t, map[string]string{"designs/basic.edn": "../edif/testdata/basic.edn"})}
	d, err := l.ReadDesign("designs/basic.edn")
	if err != nil {
		t.Fatal(err)
	}
	var classed, roled int
	for _, c := range d.Components {
		if len(c.DeviceClasses) > 0 {
			classed++
		}
	}
	for _, n := range d.Nets {
		if len(n.Roles) > 0 {
			roled++
		}
		if n.Id == "" && len(n.Connections) > 0 {
			t.Errorf("net %q has connections but no stamped id", n.Name)
		}
	}
	if classed == 0 {
		t.Error("no component carries device_classes; the ingestion classify pass did not run on the FS read")
	}
	if roled == 0 {
		t.Error("no net carries roles; the net-role pass did not run on the FS read")
	}
}

// TestReadDesignFSProvenance: a read is stamped with the name it was ASKED for, so an FS read
// carries the FS name rather than anything host-shaped. A consumer joining findings back to a file
// (the web mount+path key) depends on this being the caller's own name space.
func TestReadDesignFSProvenance(t *testing.T) {
	l := &Loader{FS: mountFS(t, map[string]string{"designs/basic.edn": "../edif/testdata/basic.edn"})}
	d, err := l.ReadDesign("designs/basic.edn")
	if err != nil {
		t.Fatal(err)
	}
	if got := d.GetProv().GetSourceFile(); got != "designs/basic.edn" {
		t.Errorf("prov.source_file = %q, want the FS name the caller passed", got)
	}
}

// TestKicadHierarchyOverFS is the multi-file case, and the reason this ticket wanted an fs.FS
// rather than a bytes entry point: a KiCad root resolves its sub-sheet through the loader's sheet
// opener, so the whole tree has to live in the same name space. Sibling resolution is relative to
// the root's directory, here a nested one, so it also pins that the FS join is not accidentally
// anchored at the FS root.
func TestKicadHierarchyOverFS(t *testing.T) {
	onDisk, err := (&Loader{}).ReadDesign("../kicad/testdata/hier_root.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	l := &Loader{FS: mountFS(t, map[string]string{
		"proj/sheets/hier_root.kicad_sch":  "../kicad/testdata/hier_root.kicad_sch",
		"proj/sheets/hier_child.kicad_sch": "../kicad/testdata/hier_child.kicad_sch",
	})}
	inMem, err := l.ReadDesign("proj/sheets/hier_root.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := summarize(inMem), summarize(onDisk); got != want {
		t.Errorf("hierarchy over FS differs from on disk\n--- fs ---\n%s\n--- os ---\n%s", got, want)
	}
}

// TestKicadHierarchyOverFSDegradesLikeOS: an FS missing the sub-sheet must fail exactly the way a
// disk read missing it fails — the root still reads, the sub-sheet's components are simply absent.
// The parity is the assertion. A host that supplies an incomplete FS gets the same partial read
// (and the same completeness signal) as one with an incomplete directory, rather than a new
// failure mode nobody has rules for.
func TestKicadHierarchyOverFSDegradesLikeOS(t *testing.T) {
	dir := t.TempDir()
	root, err := os.ReadFile("../kicad/testdata/hier_root.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/hier_root.kicad_sch", root, 0o644); err != nil {
		t.Fatal(err)
	}
	onDisk, err := (&Loader{}).ReadDesign(dir + "/hier_root.kicad_sch") // no sibling child written
	if err != nil {
		t.Fatalf("a root whose sub-sheet is missing must still read: %v", err)
	}

	l := &Loader{FS: mountFS(t, map[string]string{"proj/hier_root.kicad_sch": "../kicad/testdata/hier_root.kicad_sch"})}
	inMem, err := l.ReadDesign("proj/hier_root.kicad_sch")
	if err != nil {
		t.Fatalf("fs read of a root with no sibling: %v", err)
	}
	if got, want := summarize(inMem), summarize(onDisk); got != want {
		t.Errorf("missing sub-sheet degrades differently over FS than on disk\n--- fs ---\n%s\n--- os ---\n%s", got, want)
	}
}

// TestKicadSymbolLibOverFS covers the other multi-file resolver: an external .kicad_sym reached
// through the project's own sym-lib-table (${KIPRJMOD} = the schematic's directory). Symbols carry
// the pins, so a lib that fails to resolve reads as a pin-less design — the parity check is what
// keeps that failure mode from arriving only on FS hosts.
func TestKicadSymbolLibOverFS(t *testing.T) {
	onDisk, err := (&Loader{}).ReadDesign("../kicad/testdata/extlib.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	l := &Loader{FS: mountFS(t, map[string]string{
		"proj/extlib.kicad_sch": "../kicad/testdata/extlib.kicad_sch",
		"proj/ext.kicad_sym":    "../kicad/testdata/ext.kicad_sym",
		"proj/sym-lib-table":    "../kicad/testdata/extlib-sym-lib-table",
	})}
	inMem, err := l.ReadDesign("proj/extlib.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := summarize(inMem), summarize(onDisk); got != want {
		t.Errorf("external symbol lib over FS differs from on disk\n--- fs ---\n%s\n--- os ---\n%s", got, want)
	}
	var pins int
	for _, n := range inMem.Nets {
		pins += len(n.Connections)
	}
	if pins == 0 {
		t.Error("no pin connections; the symbol library did not resolve through the FS")
	}
}

// TestSymbolPathOverFS: --symbol-path entries resolve in the loader's name space too, including
// the recursive subtree search a gEDA/Lepton library root needs. An xschem/gEDA schematic gets its
// pins from .sym artwork, so this is the same pin-bearing dependency as the KiCad case above.
func TestSymbolPathOverFS(t *testing.T) {
	onDisk, err := (&Loader{SymbolPaths: []string{"../xschem/testdata"}}).ReadDesign("../xschem/testdata/divider.sch")
	if err != nil {
		t.Fatal(err)
	}
	l := &Loader{
		SymbolPaths: []string{"libs"}, // a library ROOT; res.sym sits in a subdirectory of it
		FS: mountFS(t, map[string]string{
			"designs/divider.sch":       "../xschem/testdata/divider.sch",
			"libs/passive/res.sym":      "../xschem/testdata/res.sym",
			"libs/probe/title.sym":      "../xschem/testdata/title.sym",
			"libs/probe/code_shown.sym": "../xschem/testdata/code_shown.sym",
		}),
	}
	inMem, err := l.ReadDesign("designs/divider.sch")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := summarize(inMem), summarize(onDisk); got != want {
		t.Errorf("symbol-path subtree search over FS differs from on disk\n--- fs ---\n%s\n--- os ---\n%s", got, want)
	}
}

// TestKicadProjectOverFS covers the .kicad_pro entry, which merges two siblings AND re-reads the
// project file twice for net-class membership and definitions. The second pass used to depend on
// seeking the open file back to the start; an fs.File is not required to be an io.Seeker, so this
// pins that both passes still land.
func TestKicadProjectOverFS(t *testing.T) {
	sch, err := os.ReadFile("../kicad/testdata/wirenet.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	const pro = `{"net_settings":{
	  "netclass_assignments": {"SIG": ["Critical"]},
	  "classes": [{"name":"Critical","clearance":0.3,"track_width":0.5}]}}`
	l := &Loader{FS: fstest.MapFS{
		"proj/board.kicad_sch": &fstest.MapFile{Data: sch},
		"proj/board.kicad_pro": &fstest.MapFile{Data: []byte(pro)},
	}}
	d, err := l.ReadDesign("proj/board.kicad_pro")
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
	if len(sig.NetClasses) != 1 || sig.NetClasses[0] != "Critical" {
		t.Errorf("SIG net_classes = %v, want [Critical] from the .kicad_pro read over the FS", sig.NetClasses)
	}
}

// TestGeometryAndBoardOverFS: the geometry and board sidecar entries take the same seam, so a
// host with no filesystem can DRAW a design, not just netlist it.
func TestGeometryAndBoardOverFS(t *testing.T) {
	l := &Loader{FS: mountFS(t, map[string]string{
		"designs/sample.eds":      "../edif/testdata/sample.eds",
		"designs/board.kicad_pcb": "../kicad/testdata/board.kicad_pcb",
	})}
	g, err := l.FaithfulGeometry("designs/sample.eds")
	if err != nil || len(g.Sheets) == 0 {
		t.Errorf("FaithfulGeometry over FS = (%v sheets, %v), want a drawable schematic", len(g.GetSheets()), err)
	}
	b, err := l.BoardGeometry("designs/board.kicad_pcb")
	if err != nil || b == nil {
		t.Errorf("BoardGeometry over FS = (%v, %v), want the board sidecar", b, err)
	}
}

package project

import (
	"strings"
	"testing"
	"testing/fstest"
)

// demoTree is the layout a review project takes:
//
//	project.yaml                      the project
//	designs/gateway/design.yaml       entry gateway.edn, board + schematic companions
//	designs/gateway/symbols/          a subfolder of the design, not a design of its own
//	scratch/loose.edn                 belongs to no design
func demoTree() Tree {
	f := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	return Tree{FS: fstest.MapFS{
		"project.yaml":                              f("name: gateway\ntitle: Gateway program\n"),
		"conventions.yaml":                          f("name: gateway\n"),
		"designs/gateway/design.yaml":               f("name: gateway\ntitle: Gateway ECU\nentry: gateway.edn\ncompanions: [gateway.kicad_pcb, gateway.kicad_sch]\n"),
		"designs/gateway/gateway.edn":               f("x"),
		"designs/gateway/gateway.kicad_pcb":         f("x"),
		"designs/gateway/gateway.kicad_sch":         f("x"),
		"designs/gateway/gateway-rev-b.edn":         f("x"),
		"designs/gateway/symbols/gateway.kicad_sym": f("x"),
		"scratch/loose.edn":                         f("x"),
	}}
}

func TestTreeProjectsAndDesigns(t *testing.T) {
	tree := demoTree()

	ps, err := tree.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Name != "gateway" || ps[0].Dir != "" {
		t.Fatalf("projects = %+v, want one at the tree root", ps)
	}

	ds, err := tree.Designs(ps[0].Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Dir != "designs/gateway" {
		t.Fatalf("designs = %+v", ds)
	}
	// Descriptor-relative names become tree-relative refs here, once, because every consumer above
	// this package addresses files relative to the tree and none knows where the design folder sits.
	if ds[0].EntryRef() != "designs/gateway/gateway.edn" {
		t.Errorf("entry ref = %q", ds[0].EntryRef())
	}
	want := []string{"designs/gateway/gateway.kicad_pcb", "designs/gateway/gateway.kicad_sch"}
	got := ds[0].CompanionRefs()
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("companion refs = %v, want %v in declared order", got, want)
	}
}

func TestTreeResolve(t *testing.T) {
	tree := demoTree()
	for _, ref := range []string{
		"designs/gateway/gateway.edn",
		"designs/gateway/gateway.kicad_pcb",
		"designs/gateway",
		"designs/gateway/",
		"designs/gateway/symbols/gateway.kicad_sym", // a subfolder still resolves to its design
	} {
		d, p, ok, err := tree.Resolve(ref)
		if err != nil || !ok {
			t.Fatalf("Resolve(%q) = ok %v, err %v", ref, ok, err)
		}
		if d.Name != "gateway" || p.Name != "gateway" {
			t.Errorf("Resolve(%q) = design %q, project %q", ref, d.Name, p.Name)
		}
	}

	_, _, ok, err := tree.Resolve("scratch/loose.edn")
	if err != nil {
		t.Fatalf("a ref under no design is the ordinary case, not an error: %v", err)
	}
	if ok {
		t.Error("scratch/loose.edn should resolve to no design")
	}
}

// TestTreeResolveNeedsAProjectParent: a design is addressable only under a parent, so a design.yaml
// with no enclosing project.yaml has no resource name and resolves to nothing.
func TestTreeResolveNeedsAProjectParent(t *testing.T) {
	tree := Tree{FS: fstest.MapFS{
		"board/design.yaml": &fstest.MapFile{Data: []byte("name: board\nentry: board.edn\n")},
		"board/board.edn":   &fstest.MapFile{Data: []byte("x")},
	}}
	if _, _, ok, err := tree.Resolve("board/board.edn"); err != nil || ok {
		t.Fatalf("Resolve = ok %v, err %v; want a miss", ok, err)
	}
}

// TestTreeRejectsDuplicateIDs: two projects claiming one name means one is unreachable through its
// own resource name, and serving the other would answer a question about A with B's designs.
func TestTreeRejectsDuplicateIDs(t *testing.T) {
	f := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	tree := Tree{FS: fstest.MapFS{
		"a/project.yaml": f("name: same\n"),
		"b/project.yaml": f("name: same\n"),
	}}
	if _, err := tree.Projects(); err == nil || !strings.Contains(err.Error(), "duplicate project id") {
		t.Fatalf("error = %v, want a duplicate-id error", err)
	}
}

// TestTreeMalformedDescriptorFailsLoudly: a skipped descriptor would leave an operator reading
// default behaviour as the engine agreeing with what they wrote.
func TestTreeMalformedDescriptorFailsLoudly(t *testing.T) {
	tree := Tree{FS: fstest.MapFS{"project.yaml": &fstest.MapFile{Data: []byte("name: Gateway\n")}}}
	if _, err := tree.Projects(); err == nil {
		t.Fatal("an invalid project id should fail the listing, not be skipped")
	}
}

// TestTreeSkipsDotDirsAndStopsAtDepth keeps the walk off a `.git` and off however deep a
// bind-mounted home directory happens to be.
func TestTreeSkipsDotDirsAndStopsAtDepth(t *testing.T) {
	f := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	tree := Tree{FS: fstest.MapFS{
		".git/modules/project.yaml": f("name: hidden\n"),
		"a/b/c/d/e/project.yaml":    f("name: toodeep\n"),
		"ok/project.yaml":           f("name: shallow\n"),
	}}
	ps, err := tree.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Name != "shallow" {
		t.Fatalf("projects = %+v, want only the shallow one", ps)
	}
}

// TestTreeNestedProjectsDoNotCompound: a project inside a project is an ambiguity nobody meant, so
// the walk stops at the outer one.
func TestTreeNestedProjectsDoNotCompound(t *testing.T) {
	f := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	tree := Tree{FS: fstest.MapFS{
		"outer/project.yaml":       f("name: outer\n"),
		"outer/inner/project.yaml": f("name: inner\n"),
	}}
	ps, err := tree.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Name != "outer" {
		t.Fatalf("projects = %+v, want only the outer one", ps)
	}
}

// TestTreeCannotEscapeItsRoot is the containment property, and it is STRUCTURAL rather than checked:
// an fs.FS has no parent to climb into, so an upward walk stops at the root and a ref carrying `..`
// never opens a file at all.
func TestTreeCannotEscapeItsRoot(t *testing.T) {
	tree := demoTree()
	if _, _, ok, err := tree.Resolve("../elsewhere/design.yaml"); ok || err != nil {
		t.Fatalf("Resolve(escaping) = ok %v, err %v; want a miss with no error", ok, err)
	}
}

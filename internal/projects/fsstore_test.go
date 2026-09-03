package projects

import (
	"context"
	"github.com/panyam/agni/artifact"
	"strings"
	"testing"
	"testing/fstest"
)

func mapFS(files map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

// demoStore is the layout a review project takes:
//
//	project.yaml                      the project
//	designs/gateway/design.yaml       entry gateway.edn, board + schematic companions
//	designs/gateway/symbols/          a subfolder of the design, not a design of its own
//	scratch/loose.edn                 belongs to no design
func demoStore() *FSStore {
	return NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"project.yaml":                              "name: gateway\ntitle: Gateway program\n",
		"conventions.yaml":                          "name: gateway\n",
		"designs/gateway/design.yaml":               "name: gateway\ntitle: Sample Board\nentry: gateway.edn\ncompanions: [gateway.kicad_pcb, gateway.kicad_sch]\n",
		"designs/gateway/gateway.edn":               "x",
		"designs/gateway/gateway.kicad_pcb":         "x",
		"designs/gateway/gateway.kicad_sch":         "x",
		"designs/gateway/gateway-rev-b.edn":         "x",
		"designs/gateway/symbols/gateway.kicad_sym": "x",
		"scratch/loose.edn":                         "x",
	})})
}

func TestFSStoreProjectsAndDesigns(t *testing.T) {
	s := demoStore()
	ctx := context.Background()

	ps, err := s.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].GetName() != "projects/gateway" || uriOf(ps[0].GetUri()).Path != "" {
		t.Fatalf("projects = %+v, want one named at the tree root", ps)
	}
	if ps[0].GetTitle() != "Gateway program" || uriOf(ps[0].GetUri()).Mount != "m" {
		t.Errorf("project = %+v, want the store to fill title and mount", ps[0])
	}

	ds, err := s.Designs(ctx, "projects/gateway")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].GetName() != "projects/gateway/designs/gateway" {
		t.Fatalf("designs = %+v", ds)
	}
	// Descriptor-relative names become mount-relative refs HERE, once, because every consumer above
	// the port addresses files by (mount, ref) and none knows where the design folder sits.
	if ds[0].GetEntryUri() != "mount://m/designs/gateway/gateway.edn" {
		t.Errorf("entry ref = %q", ds[0].GetEntryUri())
	}
	want := []string{"mount://m/designs/gateway/gateway.kicad_pcb", "mount://m/designs/gateway/gateway.kicad_sch"}
	got := ds[0].GetCompanionUris()
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("companion refs = %v, want %v in declared order", got, want)
	}

	// Get by name goes through the same discovery, so a listed resource is always fetchable.
	if p, err := s.Project(ctx, "projects/gateway"); err != nil || p.GetTitle() != "Gateway program" {
		t.Errorf("Project = %+v, %v", p, err)
	}
	if d, err := s.Design(ctx, "projects/gateway/designs/gateway"); err != nil || d.GetEntryUri() == "" {
		t.Errorf("Design = %+v, %v", d, err)
	}
}

func TestFSStoreResolve(t *testing.T) {
	s := demoStore()
	ctx := context.Background()
	for _, ref := range []string{
		"designs/gateway/gateway.edn",
		"designs/gateway/gateway.kicad_pcb",
		"designs/gateway",
		"designs/gateway/",
		"designs/gateway/symbols/gateway.kicad_sym", // a subfolder still resolves to its design
	} {
		d, p, err := s.ResolveDesign(ctx, testURI(t, "m", ref))
		if err != nil || d == nil {
			t.Fatalf("ResolveDesign(%q) = %v, err %v", ref, d, err)
		}
		if d.GetName() != "projects/gateway/designs/gateway" || p.GetName() != "projects/gateway" {
			t.Errorf("ResolveDesign(%q) = design %q, project %q", ref, d.GetName(), p.GetName())
		}
	}

	d, _, err := s.ResolveDesign(ctx, testURI(t, "m", "scratch/loose.edn"))
	if err != nil {
		t.Fatalf("a ref under no design is the ordinary case, not an error: %v", err)
	}
	if d != nil {
		t.Error("scratch/loose.edn should resolve to no design")
	}
}

// TestFSStoreResolveWithoutAProjectKeepsTheDeclaration: a design with no project above it is a real
// design, not a half-resolved one. Its declaration still says which file analysis reads; what it
// lacks is a resource NAME, since a name needs a parent.
func TestFSStoreResolveWithoutAProjectKeepsTheDeclaration(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"board/design.yaml": "name: board\nentry: board.edn\n",
		"board/board.edn":   "x",
	})})
	d, p, err := s.ResolveDesign(context.Background(), testURI(t, "m", "board/board.edn"))
	if err != nil {
		t.Fatal(err)
	}
	if d == nil {
		t.Fatal("a design with no project still declares its entry and must resolve")
	}
	if d.GetEntryUri() != "mount://m/board/board.edn" {
		t.Errorf("entry ref = %q", d.GetEntryUri())
	}
	if d.GetName() != "" {
		t.Errorf("name = %q, want empty: a resource name needs a parent", d.GetName())
	}
	if p != nil {
		t.Errorf("project = %+v, want none", p)
	}
}

// TestFSStoreRejectsDuplicateIDs: two projects claiming one name means one is unreachable through
// its own resource name, and serving the other would answer a question about A with B's designs.
func TestFSStoreRejectsDuplicateIDs(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"a/project.yaml": "name: same\n",
		"b/project.yaml": "name: same\n",
	})})
	if _, err := s.Projects(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate project id") {
		t.Fatalf("error = %v, want a duplicate-id error", err)
	}
}

// TestFSStoreRejectsDuplicateIDsAcrossTrees: the check has to span trees, since each tree only ever
// sees its own descriptors and a resource name is global to the store.
func TestFSStoreRejectsDuplicateIDsAcrossTrees(t *testing.T) {
	one := mapFS(map[string]string{"project.yaml": "name: same\n"})
	s := NewFSStore(Tree{Mount: "a", FS: one}, Tree{Mount: "b", FS: one})
	if _, err := s.Projects(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate project id") {
		t.Fatalf("error = %v, want a duplicate-id error across mounts", err)
	}
}

// TestFSStoreMalformedDescriptorFailsLoudly: a skipped descriptor would leave an operator reading
// default behaviour as the engine agreeing with what they wrote.
func TestFSStoreMalformedDescriptorFailsLoudly(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{"project.yaml": "name: Gateway\n"})})
	if _, err := s.Projects(context.Background()); err == nil {
		t.Fatal("an invalid project id should fail the listing, not be skipped")
	}
}

// TestFSStoreSkipsDotDirsAndStopsAtDepth keeps the walk off a `.git` and off however deep a
// bind-mounted home directory happens to be.
func TestFSStoreSkipsDotDirsAndStopsAtDepth(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		".git/modules/project.yaml": "name: hidden\n",
		"a/b/c/d/e/project.yaml":    "name: toodeep\n",
		"ok/project.yaml":           "name: shallow\n",
	})})
	ps, err := s.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].GetName() != "projects/shallow" {
		t.Fatalf("projects = %+v, want only the shallow one", ps)
	}
}

// TestFSStoreNestedProjectsDoNotCompound: a project inside a project is an ambiguity nobody meant,
// so the walk stops at the outer one.
func TestFSStoreNestedProjectsDoNotCompound(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"outer/project.yaml":       "name: outer\n",
		"outer/inner/project.yaml": "name: inner\n",
	})})
	ps, err := s.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].GetName() != "projects/outer" {
		t.Fatalf("projects = %+v, want only the outer one", ps)
	}
}

// TestFSStoreCannotEscapeItsTree is the containment property, and it is STRUCTURAL rather than
// checked: an fs.FS has no parent to climb into, so an upward walk stops at the root and a ref
// carrying `..` never opens a file at all.
func TestFSStoreCannotEscapeItsTree(t *testing.T) {
	d, _, err := demoStore().ResolveDesign(context.Background(), testURI(t, "m", "elsewhere/design.yaml"))
	if d != nil || err != nil {
		t.Fatalf("ResolveDesign(escaping) = %v, err %v; want a miss with no error", d, err)
	}
}

// TestFSStoreUnknownMount is classified so the service can hand the transport a code.
func TestFSStoreUnknownMount(t *testing.T) {
	if _, _, err := demoStore().ResolveDesign(context.Background(), testURI(t, "nope", "a.edn")); err == nil {
		t.Fatal("an unknown mount should be an error, not a miss")
	}
}

// testURI builds an artifact URI for a test, failing rather than returning an error: a hard-coded
// fixture URI that will not parse is a broken test, not a condition under test.
func testURI(t *testing.T, mount, p string) artifact.URI {
	t.Helper()
	u, err := artifact.New(mount, p)
	if err != nil {
		t.Fatalf("artifact.New(%q, %q): %v", mount, p, err)
	}
	return u
}

// TestFSStoreNamesTheConventionsFile: the conventions VALUE is what composes a run, and the URI is
// what a client needs to offer the project's convention back as a choice. A picker has to pass
// something, and a resolved value is not a ref, so without this a viewer can say which convention is
// in effect but cannot let a reader re-select it after trying another.
func TestFSStoreNamesTheConventionsFile(t *testing.T) {
	p, err := demoStore().Project(context.Background(), "projects/gateway")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.GetConfig().GetConventionsUri(); got != "mount://m/conventions.yaml" {
		t.Errorf("conventions uri = %q, want the file the value was read from", got)
	}
}

// TestFSStoreConventionsUriAbsentWhenUndeclared keeps the URI honest: a project with no conventions
// file must not advertise one, or a picker offers a ref that resolves to nothing.
func TestFSStoreConventionsUriAbsentWhenUndeclared(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"project.yaml": "name: bare\n",
	})})
	p, err := s.Project(context.Background(), "projects/bare")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.GetConfig().GetConventionsUri(); got != "" {
		t.Errorf("conventions uri = %q, want empty for a project that declares none", got)
	}
}

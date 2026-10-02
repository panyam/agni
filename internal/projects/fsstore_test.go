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
		"project.yaml":                              "name: gateway\ntitle: Gateway program\nconventions: {name: gateway}\n",
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
	// Descriptor-relative names become mount:// URIs HERE, once, because every consumer above
	// the port addresses files by artifact.URI and none knows where the design folder sits.
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

// TestFSStoreResolveWithoutAProjectKeepsTheDeclaration pins that a design with no project above it
// is a real design, not a half-resolved one. Its declaration still says which file analysis reads;
// what it lacks is a resource NAME, since a name needs a parent.
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

// TestFSStoreRejectsDuplicateIDs exists because two projects claiming one name means one is
// unreachable through its own resource name, and serving the other would answer a question about A
// with B's designs.
func TestFSStoreRejectsDuplicateIDs(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"a/project.yaml": "name: same\n",
		"b/project.yaml": "name: same\n",
	})})
	if _, err := s.Projects(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate project id") {
		t.Fatalf("error = %v, want a duplicate-id error", err)
	}
}

// TestFSStoreRejectsDuplicateIDsAcrossTrees pins that the check spans trees, since each tree only ever
// sees its own descriptors and a resource name is global to the store.
func TestFSStoreRejectsDuplicateIDsAcrossTrees(t *testing.T) {
	one := mapFS(map[string]string{"project.yaml": "name: same\n"})
	s := NewFSStore(Tree{Mount: "a", FS: one}, Tree{Mount: "b", FS: one})
	if _, err := s.Projects(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate project id") {
		t.Fatalf("error = %v, want a duplicate-id error across mounts", err)
	}
}

// TestFSStoreMalformedDescriptorFailsLoudly exists because a skipped descriptor would leave an
// operator reading default behaviour as the engine agreeing with what they wrote.
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

// TestFSStoreNestedProjectsDoNotCompound exists because a project inside a project is an ambiguity
// nobody meant, so the walk stops at the outer one.
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
// checked. An fs.FS has no parent to climb into, so an upward walk stops at the root and a ref
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

// testURI builds an artifact URI for a test, failing rather than returning an error, because a
// hard-coded fixture URI that will not parse is a broken test, not a condition under test.
func testURI(t *testing.T, mount, p string) artifact.URI {
	t.Helper()
	u, err := artifact.New(mount, p)
	if err != nil {
		t.Fatalf("artifact.New(%q, %q): %v", mount, p, err)
	}
	return u
}

// A project's conventions and checklists are sections of its descriptor (agni issue 828), so they
// reach the config as values, the checklists in the order they are written, since the first is the
// project's default.
func TestFSStoreCarriesTheProjectsSections(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"project.yaml": "name: p\nconventions:\n  name: house\n  lexicon: {net: {rail: {patterns: ['_V$']}}}\n" +
			"checklists:\n" +
			"  zeta: {name: Z, areas: [{name: A, items: [{id: Z1, title: z, rule: bulk-cap}]}]}\n" +
			"  alpha: {name: A, areas: [{name: A, items: [{id: A1, title: a, note: by hand}]}]}\n",
	})})
	p, err := s.Project(context.Background(), "projects/p")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.GetConfig().GetConventions().GetName(); got != "house" {
		t.Errorf("conventions name = %q, want house", got)
	}
	if got := p.GetConfig().GetConventions().GetLexicon().GetNet().GetRail().GetPatterns(); len(got) != 1 || got[0] != "_V$" {
		t.Errorf("conventions lexicon = %v, want the declared rail pattern", got)
	}
	var names []string
	for _, c := range p.GetConfig().GetChecklists() {
		names = append(names, c.GetName())
	}
	// Written order, not sorted, which is what makes zeta the default here.
	if strings.Join(names, ",") != "zeta,alpha" {
		t.Errorf("checklists = %v, want [zeta alpha] in the order written", names)
	}
	if item := p.GetConfig().GetChecklists()[0].GetManifest().GetAreas()[0].GetItems()[0]; item.GetId() != "Z1" {
		t.Errorf("first checklist's first item = %q, want Z1", item.GetId())
	}
}

// A project declaring neither section carries neither, so a client offers no checklist and the
// engine's vocabulary applies.
func TestFSStoreProjectWithNoSections(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{"project.yaml": "name: bare\n"})})
	p, err := s.Project(context.Background(), "projects/bare")
	if err != nil {
		t.Fatal(err)
	}
	if p.GetConfig().GetConventions() != nil || len(p.GetConfig().GetChecklists()) != 0 {
		t.Errorf("a bare project must carry no conventions and no checklists, got %+v", p.GetConfig())
	}
}

// A conventions.yaml or review.yaml beside project.yaml is the layout before agni issue 828. Reading
// the project without it would drop a tier and read as a team that declared none, so the load fails
// and names where the content goes.
func TestFSStoreRefusesFormerProjectFiles(t *testing.T) {
	ctx := context.Background()
	if _, err := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{"project.yaml": "name: p\n"})}).Project(ctx, "projects/p"); err != nil {
		t.Fatalf("positive control: a project with neither file must load: %v", err)
	}
	for file, want := range map[string]string{"conventions.yaml": "under conventions:", "review.yaml": "under checklists:"} {
		s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{"project.yaml": "name: p\n", file: "name: x\n"})})
		_, err := s.Projects(ctx)
		if err == nil || !strings.Contains(err.Error(), file+" is no longer read") || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %v, want it refused and pointed %s", file, err, want)
		}
	}
}

func TestParseProjectRefusesTheFileForms(t *testing.T) {
	for label, c := range map[string]struct{ doc, want string }{
		"conventions naming a file": {"name: p\nconventions: house.yaml\n", "written inline under conventions:"},
		"checklist key":             {"name: p\nchecklist: review.yaml\n", "written inline under checklists:"},
		"checklist naming a file":   {"name: p\nchecklists: {review: review.yaml}\n", "not a file name"},
		"checklists as a list":      {"name: p\nchecklists: [review]\n", "must map a name to a checklist"},
		"unknown conventions key":   {"name: p\nconventions: {nmae: x}\n", `unknown key "nmae"`},
		"invalid checklist":         {"name: p\nchecklists: {review: {name: R, areas: [{name: A, items: [{title: no id}]}]}}\n", `checklist "review"`},
	} {
		_, _, _, err := ParseProject(strings.NewReader(c.doc))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", label, err, c.want)
		}
	}
}

// TestFSStoreDiscoversALibraryAndHonoursAnOptOut covers the lib/ tier (agni issue 773): a project
// with a lib/ directory carries it as a library URI with nothing declared, and an empty `lib:`
// declaration turns it off with the directory still in place, the way every discovered tier is
// turned off.
func TestFSStoreDiscoversALibraryAndHonoursAnOptOut(t *testing.T) {
	for name, c := range map[string]struct {
		descriptor string
		want       []string
	}{
		"discovered": {"name: p\n", []string{"mount://m/lib"}},
		"declared":   {"name: p\nlib: lib\n", []string{"mount://m/lib"}},
		"opted out":  {"name: p\nlib: \"\"\n", nil},
		"elsewhere":  {"name: p\nlib: shared/dl\n", nil},
	} {
		t.Run(name, func(t *testing.T) {
			s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
				"project.yaml": c.descriptor,
				"lib/house.dl": "x(?n: net) :- net.rail(?n);\n",
			})})
			p, err := s.Project(context.Background(), "projects/p")
			if err != nil {
				t.Fatal(err)
			}
			if got := p.GetConfig().GetLibraryUris(); strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("library uris = %v, want %v", got, c.want)
			}
		})
	}
}

// A design's intent is a section of its design.yaml (agni issue 824). An intent.yaml left beside the
// descriptor fails the read and names where its declarations go, because reading the design without
// it would drop every intent rule and look like a board that declares nothing.
func TestFSStoreRefusesAFormerIntentFile(t *testing.T) {
	files := map[string]string{
		"project.yaml":                "name: gateway\n",
		"designs/gateway/design.yaml": "name: gateway\nentry: gateway.edn\n",
		"designs/gateway/gateway.edn": "x",
	}
	ctx := context.Background()
	if _, _, err := NewFSStore(Tree{Mount: "m", FS: mapFS(files)}).ResolveDesign(ctx, testURI(t, "m", "designs/gateway")); err != nil {
		t.Fatalf("positive control: the design without intent.yaml must resolve: %v", err)
	}
	files["designs/gateway/intent.yaml"] = "name: gateway\nmodules: []\n"
	_, _, err := NewFSStore(Tree{Mount: "m", FS: mapFS(files)}).ResolveDesign(ctx, testURI(t, "m", "designs/gateway"))
	if err == nil || !strings.Contains(err.Error(), "intent.yaml is no longer read") || !strings.Contains(err.Error(), "design.yaml") {
		t.Fatalf("a stray intent.yaml must fail the read and say where its declarations go, got %v", err)
	}
}

// A design's intent section reaches its config as a value (agni issue 828), so nothing above the
// store opens design.yaml again, and a design without one carries none.
func TestFSStoreCarriesTheDesignsIntent(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"project.yaml":          "name: gateway\n",
		"designs/a/design.yaml": "name: a\nentry: a.edn\nintent:\n  modules:\n  - {name: X, class: soc}\n",
		"designs/a/a.edn":       "x",
		"designs/b/design.yaml": "name: b\nentry: b.edn\n",
		"designs/b/b.edn":       "x",
	})})
	ctx := context.Background()
	a, _, err := s.ResolveDesign(ctx, testURI(t, "m", "designs/a"))
	if err != nil {
		t.Fatal(err)
	}
	if got := a.GetConfig().GetIntent().GetModules(); len(got) != 1 || got[0].GetClass() != "soc" {
		t.Errorf("intent modules = %v, want the declared module", got)
	}
	b, _, err := s.ResolveDesign(ctx, testURI(t, "m", "designs/b"))
	if err != nil {
		t.Fatal(err)
	}
	if b.GetConfig().GetIntent() != nil {
		t.Errorf("a design declaring no intent carries none, got %v", b.GetConfig().GetIntent())
	}
}

func TestParseDesignRefusesIntentNamingAFile(t *testing.T) {
	_, _, err := ParseDesign(strings.NewReader("name: a\nentry: a.edn\nintent: intent.yaml\n"))
	if err == nil || !strings.Contains(err.Error(), "declared inline") {
		t.Fatalf("intent naming a file must fail and say it is declared inline now, got %v", err)
	}
}

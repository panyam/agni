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
//	designs/gateway/design.yaml       extends it, entry gateway.edn, board + schematic companions
//	designs/gateway/symbols/          a subfolder of the design, not a design of its own
//	scratch/loose.edn                 belongs to no design
func demoStore() *FSStore {
	return NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"project.yaml":                              "name: gateway\ntitle: Gateway program\nconventions: {name: gateway}\n",
		"designs/gateway/design.yaml":               "name: gateway\ntitle: Sample Board\nextends: ../../project.yaml\nentry: gateway.edn\ncompanions: [gateway.kicad_pcb, gateway.kicad_sch]\n",
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

// A section is written inline, so a section holding a file name fails at its line rather than reading
// as a project that declared none.
func TestParseProjectRefusesMalformedSections(t *testing.T) {
	for label, c := range map[string]struct{ doc, want string }{
		"conventions naming a file": {"name: p\nconventions: house.yaml\n", "must be a mapping"},
		"checklist key":             {"name: p\nchecklist: review.yaml\n", "field checklist not found"},
		"checklist naming a file":   {"name: p\nchecklists: {review: review.yaml}\n", "must be a checklist (name, areas)"},
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

// A design's intent section reaches its config as a value (agni issue 824), so nothing above the
// store opens design.yaml again, and a design without one carries none.
func TestFSStoreCarriesTheDesignsIntent(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"project.yaml":          "name: gateway\n",
		"designs/a/design.yaml": "name: a\nextends: ../../project.yaml\nentry: a.edn\nintent:\n  modules:\n  - {name: X, class: soc}\n",
		"designs/a/a.edn":       "x",
		"designs/b/design.yaml": "name: b\nextends: ../../project.yaml\nentry: b.edn\n",
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

// Every tier a run reads is named in a descriptor (agni issue 1136). A directory or a single file
// becomes a URI, a folder with a tier's conventional name that nothing names is not read, and a path
// naming nothing fails the load rather than reading as a team that declared no such tier.
func TestFSStoreReadsOnlyTheSourcesADescriptorNames(t *testing.T) {
	files := map[string]string{
		"lib/house.dl":         "x(?n: net) :- net.rail(?n);\n",
		"queries/power.yaml":   "queries: []\n",
		"profiles/can.yaml":    "x",
		"shared/params/a.json": "x",
	}
	for name, c := range map[string]struct {
		descriptor string
		lib, qs    []string
		profiles   []string
		params     []string
	}{
		"nothing named":   {descriptor: "name: p\n"},
		"directories":     {descriptor: "name: p\nlib: [lib]\nqueries: [queries/]\n", lib: []string{"mount://m/lib"}, qs: []string{"mount://m/queries"}},
		"a single file":   {descriptor: "name: p\nprofiles: [profiles/can.yaml]\n", profiles: []string{"mount://m/profiles/can.yaml"}},
		"a mount uri":     {descriptor: "name: p\nparams: ['mount://m/shared/params']\n", params: []string{"mount://m/shared/params"}},
		"several sources": {descriptor: "name: p\nlib: [lib, lib/house.dl]\n", lib: []string{"mount://m/lib", "mount://m/lib/house.dl"}},
	} {
		t.Run(name, func(t *testing.T) {
			fs := map[string]string{"project.yaml": c.descriptor}
			for k, v := range files {
				fs[k] = v
			}
			p, err := NewFSStore(Tree{Mount: "m", FS: mapFS(fs)}).Project(context.Background(), "projects/p")
			if err != nil {
				t.Fatal(err)
			}
			cfg := p.GetConfig()
			for tier, pair := range map[string][2][]string{
				"lib": {cfg.GetLibraryUris(), c.lib}, "queries": {cfg.GetQuerySetUris(), c.qs},
				"profiles": {cfg.GetProfileUris(), c.profiles}, "params": {cfg.GetParamUris(), c.params},
			} {
				if strings.Join(pair[0], ",") != strings.Join(pair[1], ",") {
					t.Errorf("%s = %v, want %v", tier, pair[0], pair[1])
				}
			}
		})
	}
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{"project.yaml": "name: p\nprofiles: [ifaces]\n"})})
	if _, err := s.Projects(context.Background()); err == nil || !strings.Contains(err.Error(), `profiles "ifaces" names nothing`) {
		t.Errorf("a source naming nothing: error = %v, want the load refused naming it", err)
	}
	s = NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{"project.yaml": "name: p\nlib: [../outside]\n"})})
	if _, err := s.Projects(context.Background()); err == nil || !strings.Contains(err.Error(), `lib "../outside" is outside mount "m"`) {
		t.Errorf("a source leaving the mount: error = %v, want it refused", err)
	}
}

// A design names its project (agni issue 1136), so its project is the one it names wherever the two
// sit, and a project's designs are the ones naming it, not the ones under its folder.
func TestFSStoreADesignBelongsToTheProjectItNames(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{
		"teams/house/project.yaml":  "name: house\n",
		"teams/other/project.yaml":  "name: other\n",
		"boards/a/design.yaml":      "name: a\nextends: ../../teams/house/project.yaml\nentry: a.edn\n",
		"boards/a/a.edn":            "x",
		"teams/other/b/design.yaml": "name: b\nextends: ../../house/project.yaml\nentry: b.edn\n",
		"teams/other/b/b.edn":       "x",
		"teams/other/c/design.yaml": "name: c\nextends: ../project.yaml\nentry: c.edn\n",
		"teams/other/c/c.edn":       "x",
	})})
	ctx := context.Background()
	for team, want := range map[string]string{"projects/house": "projects/house/designs/a,projects/house/designs/b", "projects/other": "projects/other/designs/c"} {
		ds, err := s.Designs(ctx, team)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, d := range ds {
			got = append(got, d.GetName())
		}
		if strings.Join(got, ",") != want {
			t.Errorf("Designs(%s) = %v, want %s", team, got, want)
		}
	}
	d, p, err := s.ResolveDesign(ctx, testURI(t, "m", "teams/other/b/b.edn"))
	if err != nil {
		t.Fatal(err)
	}
	if p.GetName() != "projects/house" || d.GetName() != "projects/house/designs/b" {
		t.Errorf("a design under one project's folder naming another resolved to %q in %q, want house's", d.GetName(), p.GetName())
	}
}

// A design that names no parent while a project descriptor sits above it is the shape of a forgotten
// `extends`, and reading on would run the board on the built-in config with nothing said.
func TestFSStoreRefusesAForgottenExtends(t *testing.T) {
	files := map[string]string{
		"project.yaml":          "name: p\n",
		"designs/a/design.yaml": "name: a\nextends: ../../project.yaml\nentry: a.edn\n",
		"designs/a/a.edn":       "x",
	}
	ctx := context.Background()
	if _, p, err := NewFSStore(Tree{Mount: "m", FS: mapFS(files)}).ResolveDesign(ctx, testURI(t, "m", "designs/a")); err != nil || p.GetName() != "projects/p" {
		t.Fatalf("positive control: a design naming its project resolves to it, got %v, %v", p, err)
	}
	files["designs/a/design.yaml"] = "name: a\nentry: a.edn\n"
	_, _, err := NewFSStore(Tree{Mount: "m", FS: mapFS(files)}).ResolveDesign(ctx, testURI(t, "m", "designs/a"))
	if err == nil || !strings.Contains(err.Error(), "extends nothing, and m:project.yaml sits above it") {
		t.Fatalf("error = %v, want the forgotten extends named", err)
	}
}

// A project names the descriptor it extends by path or by mount:// URI, and the store hands it on as
// that project's resource name, which is what the service walks the chain by.
func TestFSStoreNamesAProjectsParentByResourceName(t *testing.T) {
	s := NewFSStore(
		Tree{Mount: "m", FS: mapFS(map[string]string{
			"house/project.yaml": "name: house\nconventions: {name: house}\n",
			"a/project.yaml":     "name: a\nextends: ../house/project.yaml\n",
			"b/project.yaml":     "name: b\nextends: 'mount://shared/project.yaml'\n",
		})},
		Tree{Mount: "shared", FS: mapFS(map[string]string{"project.yaml": "name: shared\n"})},
	)
	ctx := context.Background()
	for name, want := range map[string]string{"projects/a": "projects/house", "projects/b": "projects/shared"} {
		p, err := s.Project(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.GetConfig().GetExtends(); got != want {
			t.Errorf("%s extends %q, want %q", name, got, want)
		}
	}
	s = NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{"a/project.yaml": "name: a\nextends: ../house/project.yaml\n"})})
	if _, err := s.Projects(ctx); err == nil || !strings.Contains(err.Error(), "names nothing") {
		t.Errorf("extending a descriptor that is not there: error = %v, want it refused", err)
	}
}

// A design copied out of its project, or mounted or dropped without it, names a parent that is not
// there. The error says what to do: bring the project along, or remove extends to read the design on
// its own.
func TestFSStoreSaysHowToReadADesignWithoutItsProject(t *testing.T) {
	ctx := context.Background()
	for name, c := range map[string]struct {
		files map[string]string
		at    string
		want  string
	}{
		"mounted alone": {
			files: map[string]string{"design.yaml": "name: g\nextends: ../../project.yaml\nentry: g.edn\n", "g.edn": "x"},
			at:    "g.edn",
			want:  `extends "../../project.yaml" is outside mount "m", so it cannot be read; mount or drop the folder holding that project.yaml as well, or remove extends`,
		},
		"dropped alone": {
			files: map[string]string{"drop-1/g/design.yaml": "name: g\nextends: ../../project.yaml\nentry: g.edn\n", "drop-1/g/g.edn": "x"},
			at:    "drop-1/g/g.edn",
			want:  `extends "../../project.yaml" names nothing (looked for mount://m/project.yaml); mount or drop the folder holding that project.yaml as well, or remove extends`,
		},
	} {
		_, _, err := NewFSStore(Tree{Mount: "m", FS: mapFS(c.files)}).ResolveDesign(ctx, testURI(t, "m", c.at))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v\nwant it to say %q", name, err, c.want)
		}
	}
}

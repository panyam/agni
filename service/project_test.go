package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/goapplib/storage/mounts"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// memProjects is an in-memory ProjectStore, enough to drive the service without a filesystem. It
// needs no tree, no descriptor and no parent directory, and the port exists to guarantee that.
type memProjects struct {
	projects []*webapi.Project
	resolve  map[string]*webapi.Design // "mount:ref" -> design
}

func (m *memProjects) Project(_ context.Context, name string) (*webapi.Project, error) {
	for _, p := range m.projects {
		if p.GetName() == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("%w: no project %q", ErrNotFound, name)
}

func (m *memProjects) Projects(context.Context) ([]*webapi.Project, error) { return m.projects, nil }

func (m *memProjects) ResolveDesign(ctx context.Context, uri mounts.URI) (*webapi.Design, *webapi.Project, error) {
	d, ok := m.resolve[uri.String()]
	if !ok {
		return nil, nil, nil
	}
	p, err := m.Project(ctx, d.GetConfig().GetExtends())
	return d, p, err
}

func fixtureStore() *memProjects {
	gw := &webapi.Design{
		Name: "designs/gateway", Title: "Sample Board",
		Uri: "mount://boards/designs/gateway", EntryUri: "mount://boards/designs/gateway/gateway.edn",
		CompanionUris: []string{"mount://boards/designs/gateway/gateway.kicad_pcb"},
		Config:        &webapi.AnalysisConfig{Extends: "projects/gateway"},
	}
	return &memProjects{
		projects: []*webapi.Project{
			{Name: "projects/gateway", Title: "Gateway program", Uri: "mount://boards", Designs: []string{"designs/gateway"}},
			{Name: "projects/sensor", Title: "sensor", Uri: "mount://other/sensor"},
		},
		resolve: map[string]*webapi.Design{"mount://boards/designs/gateway/gateway.kicad_pcb": gw},
	}
}

func TestGetProjectListsItsDesigns(t *testing.T) {
	svc := NewProjectService(fixtureStore())
	ctx := context.Background()

	p, err := svc.GetProject(ctx, &webapi.GetProjectRequest{Name: "projects/gateway"})
	if err != nil {
		t.Fatal(err)
	}
	if p.GetName() != "projects/gateway" || p.GetTitle() != "Gateway program" || p.GetUri() != "mount://boards" {
		t.Fatalf("project = %+v", p)
	}
	if len(p.GetDesigns()) != 1 || p.GetDesigns()[0] != "designs/gateway" {
		t.Errorf("designs = %v, want designs/gateway", p.GetDesigns())
	}
}

// TestGetClassifiesMalformedNameApartFromAbsent checks that a malformed name and an absent project
// map to different codes, because a client has to tell a typo from a project it does not have.
func TestGetClassifiesMalformedNameApartFromAbsent(t *testing.T) {
	svc := NewProjectService(fixtureStore())
	ctx := context.Background()
	for _, name := range []string{"gateway", "projects/", "projects/../etc", "projects/a/b"} {
		if _, err := svc.GetProject(ctx, &webapi.GetProjectRequest{Name: name}); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("GetProject(%q) error = %v, want ErrInvalidArgument", name, err)
		}
	}
	if _, err := svc.GetProject(ctx, &webapi.GetProjectRequest{Name: "projects/nope"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetProject(absent) error = %v, want ErrNotFound", err)
	}
}

func TestListProjectsFilter(t *testing.T) {
	svc := NewProjectService(fixtureStore())
	ctx := context.Background()

	all, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.GetProjects()) != 2 {
		t.Fatalf("unfiltered = %d projects, want 2", len(all.GetProjects()))
	}
	one, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{Filter: `mount="other"`})
	if err != nil {
		t.Fatal(err)
	}
	if len(one.GetProjects()) != 1 || one.GetProjects()[0].GetName() != "projects/sensor" {
		t.Fatalf("filtered = %+v", one.GetProjects())
	}
	// A filter the service does not implement must be refused, not ignored. A client that believed it
	// had narrowed to its own mount and silently got every mount would read another team's projects
	// as its own.
	if _, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{Filter: `title="x"`}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("unsupported filter error = %v, want ErrInvalidArgument", err)
	}
	if _, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{Filter: `mount=""`}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty-mount filter error = %v, want ErrInvalidArgument", err)
	}
}

// TestResolveDesignMissIsEmptyNotError pins a contract, and the project surfaces are safe to apply
// because of it. Most files on a mount belong to no declared design, and that answer is a normal empty
// response. A client reading it shows the plain viewer rather than some other project's config.
func TestResolveDesignMissIsEmptyNotError(t *testing.T) {
	svc := NewProjectService(fixtureStore())
	ctx := context.Background()

	hit, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Uri: "mount://boards/designs/gateway/gateway.kicad_pcb"})
	if err != nil {
		t.Fatal(err)
	}
	if hit.GetDesign().GetName() != "designs/gateway" {
		t.Fatalf("resolved design = %+v", hit.GetDesign())
	}
	if hit.GetProject().GetName() != "projects/gateway" {
		t.Errorf("project = %+v, want the design's parent", hit.GetProject())
	}

	miss, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Uri: "mount://boards/scratch/notes.edn"})
	if err != nil {
		t.Fatalf("an unresolved ref is the ordinary case, not an error: %v", err)
	}
	if miss.GetDesign() != nil || miss.GetProject() != nil {
		t.Errorf("miss = %+v, want both unset", miss)
	}
}

// TestNilStoreAnswersAsUnconfigured pins that a server started with no mounts carrying descriptors
// must still serve, so resolution reports nothing rather than failing.
func TestNilStoreAnswersAsUnconfigured(t *testing.T) {
	svc := NewProjectService(nil)
	ctx := context.Background()
	resp, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Uri: "mount://m/a.edn"})
	if err != nil || resp.GetDesign() != nil {
		t.Fatalf("ResolveDesign = %+v, %v; want an empty response and no error", resp, err)
	}
	list, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{})
	if err != nil || len(list.GetProjects()) != 0 {
		t.Fatalf("ListProjects = %+v, %v; want an empty list and no error", list, err)
	}
	if _, err := svc.GetProject(ctx, &webapi.GetProjectRequest{Name: "projects/x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetProject error = %v, want ErrNotFound", err)
	}
}

func TestResourceNameRoundTrip(t *testing.T) {
	if got := ProjectName("gateway"); got != "projects/gateway" {
		t.Errorf("ProjectName = %q", got)
	}
	if got := StandaloneDesignName("rev-b"); got != "designs/rev-b" {
		t.Errorf("StandaloneDesignName = %q", got)
	}
	if id, ok := ProjectID(ProjectName("gateway")); !ok || id != "gateway" {
		t.Errorf("ProjectID round trip = %q, %v", id, ok)
	}
	if id, ok := StandaloneDesignID(StandaloneDesignName("rev-b")); !ok || id != "rev-b" {
		t.Errorf("StandaloneDesignID round trip = %q, %v", id, ok)
	}
	// An id reaching a store that may resolve it against a filesystem must not be able to escape.
	for _, bad := range []string{"projects/..", "projects/a/b", "projects/", `projects/a\b`} {
		if _, ok := ProjectID(bad); ok {
			t.Errorf("ProjectID(%q) accepted an id that could escape the tree", bad)
		}
	}
	for _, bad := range []string{"designs/..", "designs/a/b", "designs/", "projects/p/designs/d"} {
		if _, ok := StandaloneDesignID(bad); ok {
			t.Errorf("StandaloneDesignID(%q) accepted a name that is not designs/{design}", bad)
		}
	}
}

// TestSourcesFor covers the one place that decides the per-tier answer, so CLI and web cannot
// disagree about which companion supplies the board.
func TestSourcesFor(t *testing.T) {
	d := &webapi.Design{
		EntryUri:      "mount://m/d/board.edn",
		CompanionUris: []string{"mount://m/d/board.kicad_sch", "mount://m/d/board.kicad_pcb"},
	}

	// Naming the design itself, every tier comes from the declaration.
	s := SourcesFor(d, "")
	if s.NetlistURI != "mount://m/d/board.edn" || s.BoardURI != "mount://m/d/board.kicad_pcb" || s.GeometryURI != "mount://m/d/board.kicad_sch" {
		t.Fatalf("design-named sources = %+v", s)
	}

	// Naming a companion, only the NETLIST tier moves. The named artifact keeps whatever it alone
	// supplies, because the caller pointed at it for that.
	s = SourcesFor(d, "mount://m/d/board.kicad_pcb")
	if s.NetlistURI != "mount://m/d/board.edn" || s.BoardURI != "mount://m/d/board.kicad_pcb" {
		t.Fatalf("companion-named sources = %+v", s)
	}

	// A design with no companions leaves every tier on the entry rather than inventing one.
	bare := &webapi.Design{EntryUri: "mount://m/d/board.edn"}
	s = SourcesFor(bare, "")
	if s.BoardURI != "mount://m/d/board.edn" || s.GeometryURI != "mount://m/d/board.edn" {
		t.Fatalf("bare sources = %+v", s)
	}
}

func TestIsCompanion(t *testing.T) {
	d := &webapi.Design{EntryUri: "mount://m/d/a.edn", CompanionUris: []string{"mount://m/d/a.kicad_pcb"}}
	if !IsCompanion(d, "mount://m/d/a.kicad_pcb") {
		t.Error("a declared companion should be recognised")
	}
	// Companions are declared rather than inferred because an undeclared sibling is a legitimate
	// analysis source in its own right.
	if IsCompanion(d, "mount://m/d/a-rev-b.edn") || IsCompanion(d, "mount://m/d/a.edn") {
		t.Error("only declared companions are companions")
	}
}

// TestPaginateResumesByName checks that the page token is the next item's resource name rather than
// an offset, so a project added or removed between calls cannot make a listing skip or repeat a
// neighbour.
func TestPaginateResumesByName(t *testing.T) {
	names := []string{"a", "b", "c", "d"}
	at := func(i int) string { return names[i] }

	page, next := paginate(len(names), 2, "", at)
	if len(page) != 2 || page[0] != 0 || next != "c" {
		t.Fatalf("first page = %v, next = %q", page, next)
	}
	page, next = paginate(len(names), 2, next, at)
	if len(page) != 2 || names[page[0]] != "c" || next != "" {
		t.Fatalf("second page = %v, next = %q", page, next)
	}
	// With "c" removed between the two calls, resuming by name reaches "d" rather than skipping it.
	shrunk := []string{"a", "b", "d"}
	page, _ = paginate(len(shrunk), 2, "c", func(i int) string { return shrunk[i] })
	if len(page) != 1 || shrunk[page[0]] != "d" {
		t.Fatalf("resume after a removed item = %v, want [d]", page)
	}
}

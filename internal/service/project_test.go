package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// memProjects is an in-memory ProjectStore: enough to drive the service without a filesystem, which
// is the point of the port. That it needs no tree, no descriptor, and no parent directory is the
// property the port exists to guarantee.
type memProjects struct {
	projects []*webapi.Project
	designs  map[string][]*webapi.Design
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

func (m *memProjects) Design(ctx context.Context, name string) (*webapi.Design, error) {
	parent, _, ok := SplitDesignName(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidArgument, name)
	}
	ds, err := m.Designs(ctx, parent)
	if err != nil {
		return nil, err
	}
	for _, d := range ds {
		if d.GetName() == name {
			return d, nil
		}
	}
	return nil, fmt.Errorf("%w: no design %q", ErrNotFound, name)
}

func (m *memProjects) Designs(ctx context.Context, parent string) ([]*webapi.Design, error) {
	if _, err := m.Project(ctx, parent); err != nil {
		return nil, err
	}
	return m.designs[parent], nil
}

func (m *memProjects) ResolveDesign(ctx context.Context, mount, ref string) (*webapi.Design, *webapi.Project, error) {
	d, ok := m.resolve[mount+":"+ref]
	if !ok {
		return nil, nil, nil
	}
	parent, _, _ := SplitDesignName(d.GetName())
	p, err := m.Project(ctx, parent)
	return d, p, err
}

func fixtureStore() *memProjects {
	gw := &webapi.Design{
		Name: "projects/gateway/designs/gateway", Title: "Gateway ECU", Mount: "boards",
		DirRef: "designs/gateway", EntryRef: "designs/gateway/gateway.edn",
		CompanionRefs: []string{"designs/gateway/gateway.kicad_pcb"},
	}
	return &memProjects{
		projects: []*webapi.Project{
			{Name: "projects/gateway", Title: "Gateway program", Mount: "boards"},
			{Name: "projects/sensor", Title: "sensor", Mount: "other", DirRef: "sensor"},
		},
		designs: map[string][]*webapi.Design{"projects/gateway": {gw}},
		resolve: map[string]*webapi.Design{"boards:designs/gateway/gateway.kicad_pcb": gw},
	}
}

func TestGetProjectAndDesign(t *testing.T) {
	svc := NewProjectService(fixtureStore())
	ctx := context.Background()

	p, err := svc.GetProject(ctx, &webapi.GetProjectRequest{Name: "projects/gateway"})
	if err != nil {
		t.Fatal(err)
	}
	if p.GetName() != "projects/gateway" || p.GetTitle() != "Gateway program" || p.GetMount() != "boards" {
		t.Fatalf("project = %+v", p)
	}

	d, err := svc.GetDesign(ctx, &webapi.GetProjectDesignRequest{Name: "projects/gateway/designs/gateway"})
	if err != nil {
		t.Fatal(err)
	}
	if d.GetEntryRef() != "designs/gateway/gateway.edn" {
		t.Errorf("entry_ref = %q, want the declared entry", d.GetEntryRef())
	}
}

// TestGetClassifiesMalformedNameApartFromAbsent: a client has to be able to tell a typo from a
// project it does not have, so the two map to different codes.
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
	for _, name := range []string{"projects/gateway", "projects/gateway/designs/", "designs/x"} {
		if _, err := svc.GetDesign(ctx, &webapi.GetProjectDesignRequest{Name: name}); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("GetDesign(%q) error = %v, want ErrInvalidArgument", name, err)
		}
	}
}

// TestListDesignsAbsentParentIsNotFound: "this project has no designs" and "there is no such
// project" are different answers, and only one means the client's parent was wrong.
func TestListDesignsAbsentParentIsNotFound(t *testing.T) {
	svc := NewProjectService(fixtureStore())
	_, err := svc.ListDesigns(context.Background(), &webapi.ListProjectDesignsRequest{Parent: "projects/nope"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	resp, err := svc.ListDesigns(context.Background(), &webapi.ListProjectDesignsRequest{Parent: "projects/sensor"})
	if err != nil {
		t.Fatalf("a project with no designs should list empty, got %v", err)
	}
	if len(resp.GetDesigns()) != 0 {
		t.Errorf("designs = %v, want none", resp.GetDesigns())
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
	// A filter the service does not implement must be refused, not ignored: a client that believed it
	// had narrowed to its own mount and silently got every mount would read another team's projects
	// as its own.
	if _, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{Filter: `title="x"`}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("unsupported filter error = %v, want ErrInvalidArgument", err)
	}
	if _, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{Filter: `mount=""`}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty-mount filter error = %v, want ErrInvalidArgument", err)
	}
}

// TestResolveDesignMissIsEmptyNotError pins the contract that makes the project surfaces safe to
// apply: most files on a mount belong to no declared design, and that answer is a normal empty
// response. A client reading it shows the plain viewer rather than some other project's config.
func TestResolveDesignMissIsEmptyNotError(t *testing.T) {
	svc := NewProjectService(fixtureStore())
	ctx := context.Background()

	hit, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Mount: "boards", Ref: "designs/gateway/gateway.kicad_pcb"})
	if err != nil {
		t.Fatal(err)
	}
	if hit.GetDesign().GetName() != "projects/gateway/designs/gateway" {
		t.Fatalf("resolved design = %+v", hit.GetDesign())
	}
	if hit.GetProject().GetName() != "projects/gateway" {
		t.Errorf("project = %+v, want the design's parent", hit.GetProject())
	}

	miss, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Mount: "boards", Ref: "scratch/notes.edn"})
	if err != nil {
		t.Fatalf("an unresolved ref is the ordinary case, not an error: %v", err)
	}
	if miss.GetDesign() != nil || miss.GetProject() != nil {
		t.Errorf("miss = %+v, want both unset", miss)
	}
}

// TestNilStoreAnswersAsUnconfigured: a server started with no mounts carrying descriptors must still
// serve, so resolution reports nothing rather than failing.
func TestNilStoreAnswersAsUnconfigured(t *testing.T) {
	svc := NewProjectService(nil)
	ctx := context.Background()
	resp, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Mount: "m", Ref: "a.edn"})
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
	if got := DesignName("projects/gateway", "rev-b"); got != "projects/gateway/designs/rev-b" {
		t.Errorf("DesignName = %q", got)
	}
	if id, ok := ProjectID(ProjectName("gateway")); !ok || id != "gateway" {
		t.Errorf("ProjectID round trip = %q, %v", id, ok)
	}
	parent, id, ok := SplitDesignName(DesignName("projects/gateway", "rev-b"))
	if !ok || parent != "projects/gateway" || id != "rev-b" {
		t.Errorf("SplitDesignName round trip = %q, %q, %v", parent, id, ok)
	}
	// An id reaching a store that may resolve it against a filesystem must not be able to escape.
	for _, bad := range []string{"projects/..", "projects/a/b", "projects/", `projects/a\b`} {
		if _, ok := ProjectID(bad); ok {
			t.Errorf("ProjectID(%q) accepted an id that could escape the tree", bad)
		}
	}
}

// TestSourcesFor is the one place the per-tier answer is decided, so CLI and web cannot disagree
// about which companion supplies the board.
func TestSourcesFor(t *testing.T) {
	d := &webapi.Design{
		EntryRef:      "d/board.edn",
		CompanionRefs: []string{"d/board.kicad_sch", "d/board.kicad_pcb"},
	}

	// Naming the design itself: every tier comes from the declaration.
	s := SourcesFor(d, "")
	if s.NetlistRef != "d/board.edn" || s.BoardRef != "d/board.kicad_pcb" || s.GeometryRef != "d/board.kicad_sch" {
		t.Fatalf("design-named sources = %+v", s)
	}

	// Naming a companion: only the NETLIST tier moves. The named artifact keeps whatever it alone
	// supplies, because that is why the caller pointed at it.
	s = SourcesFor(d, "d/board.kicad_pcb")
	if s.NetlistRef != "d/board.edn" || s.BoardRef != "d/board.kicad_pcb" {
		t.Fatalf("companion-named sources = %+v", s)
	}

	// A design with no companions leaves every tier on the entry rather than inventing one.
	bare := &webapi.Design{EntryRef: "d/board.edn"}
	s = SourcesFor(bare, "")
	if s.BoardRef != "d/board.edn" || s.GeometryRef != "d/board.edn" {
		t.Fatalf("bare sources = %+v", s)
	}
}

func TestIsCompanion(t *testing.T) {
	d := &webapi.Design{EntryRef: "d/a.edn", CompanionRefs: []string{"d/a.kicad_pcb"}}
	if !IsCompanion(d, "d/a.kicad_pcb") {
		t.Error("a declared companion should be recognised")
	}
	// The reason companions are declared rather than inferred: an undeclared sibling is a legitimate
	// analysis source in its own right.
	if IsCompanion(d, "d/a-rev-b.edn") || IsCompanion(d, "d/a.edn") {
		t.Error("only declared companions are companions")
	}
}

// TestPaginateResumesByName: the page token is the next item's resource name rather than an offset,
// so a project added or removed between calls cannot make a listing skip or repeat a neighbour.
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
	// "c" removed between the two calls: resuming by name lands on "d" rather than skipping it.
	shrunk := []string{"a", "b", "d"}
	page, _ = paginate(len(shrunk), 2, "c", func(i int) string { return shrunk[i] })
	if len(page) != 1 || shrunk[page[0]] != "d" {
		t.Fatalf("resume after a removed item = %v, want [d]", page)
	}
}

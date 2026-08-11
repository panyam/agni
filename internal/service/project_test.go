package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// memProjects is an in-memory ProjectStore: enough to drive the service without a filesystem, which
// is the point of the port.
type memProjects struct {
	projects []ProjectInfo
	designs  map[string][]DesignInfo
	resolve  map[string]DesignInfo // "mount:ref" -> design
}

func (m *memProjects) Project(_ context.Context, id string) (ProjectInfo, error) {
	for _, p := range m.projects {
		if p.ID == id {
			return p, nil
		}
	}
	return ProjectInfo{}, fmt.Errorf("%w: no project %q", ErrNotFound, id)
}

func (m *memProjects) Projects(context.Context) ([]ProjectInfo, error) { return m.projects, nil }

func (m *memProjects) Design(ctx context.Context, projectID, designID string) (DesignInfo, error) {
	ds, err := m.Designs(ctx, projectID)
	if err != nil {
		return DesignInfo{}, err
	}
	for _, d := range ds {
		if d.ID == designID {
			return d, nil
		}
	}
	return DesignInfo{}, fmt.Errorf("%w: no design %q", ErrNotFound, designID)
}

func (m *memProjects) Designs(ctx context.Context, projectID string) ([]DesignInfo, error) {
	if _, err := m.Project(ctx, projectID); err != nil {
		return nil, err
	}
	return m.designs[projectID], nil
}

func (m *memProjects) Resolve(ctx context.Context, mount, ref string) (DesignInfo, ProjectInfo, bool, error) {
	d, ok := m.resolve[mount+":"+ref]
	if !ok {
		return DesignInfo{}, ProjectInfo{}, false, nil
	}
	p, err := m.Project(ctx, d.ProjectID)
	return d, p, true, err
}

func fixtureStore() *memProjects {
	gw := DesignInfo{
		ProjectID: "gateway", ID: "gateway", Title: "Gateway ECU", Mount: "boards",
		DirRef: "designs/gateway", EntryRef: "designs/gateway/gateway.edn",
		CompanionRefs: []string{"designs/gateway/gateway.kicad_pcb"},
	}
	return &memProjects{
		projects: []ProjectInfo{
			{ID: "gateway", Title: "Gateway program", Mount: "boards", DirRef: ""},
			{ID: "sensor", Title: "sensor", Mount: "other", DirRef: "sensor"},
		},
		designs: map[string][]DesignInfo{"gateway": {gw}},
		resolve: map[string]DesignInfo{"boards:designs/gateway/gateway.kicad_pcb": gw},
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
	if d.GetName() != "projects/gateway/designs/gateway" {
		t.Errorf("design name = %q", d.GetName())
	}
	if d.GetEntryRef() != "designs/gateway/gateway.edn" {
		t.Errorf("entry_ref = %q, want the declared entry", d.GetEntryRef())
	}
	if got := d.GetCompanionRefs(); len(got) != 1 || got[0] != "designs/gateway/gateway.kicad_pcb" {
		t.Errorf("companion_refs = %v", got)
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
		t.Errorf("design and project are set together, got project %+v", hit.GetProject())
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
	if got := DesignName("gateway", "rev-b"); got != "projects/gateway/designs/rev-b" {
		t.Errorf("DesignName = %q", got)
	}
	if id, ok := ProjectID(ProjectName("gateway")); !ok || id != "gateway" {
		t.Errorf("ProjectID round trip = %q, %v", id, ok)
	}
	p, d, ok := DesignID(DesignName("gateway", "rev-b"))
	if !ok || p != "gateway" || d != "rev-b" {
		t.Errorf("DesignID round trip = %q, %q, %v", p, d, ok)
	}
	// An id reaching a filesystem-backed adapter must not be able to steer out of the store's tree.
	for _, bad := range []string{"projects/..", "projects/a/b", "projects/", `projects/a\b`} {
		if _, ok := ProjectID(bad); ok {
			t.Errorf("ProjectID(%q) accepted an id that could escape the tree", bad)
		}
	}
}

// TestPaginateResumesByID: the page token is the next item's id rather than an offset, so a project
// added or removed between calls cannot make a listing skip or repeat a neighbour.
func TestPaginateResumesByID(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	at := func(i int) string { return ids[i] }

	page, next := paginate(len(ids), 2, "", at)
	if len(page) != 2 || page[0] != 0 || next != "c" {
		t.Fatalf("first page = %v, next = %q", page, next)
	}
	page, next = paginate(len(ids), 2, next, at)
	if len(page) != 2 || ids[page[0]] != "c" || next != "" {
		t.Fatalf("second page = %v, next = %q", page, next)
	}
	// "c" removed between the two calls: resuming by id lands on "d" rather than skipping it.
	shrunk := []string{"a", "b", "d"}
	page, _ = paginate(len(shrunk), 2, "c", func(i int) string { return shrunk[i] })
	if len(page) != 1 || shrunk[page[0]] != "d" {
		t.Fatalf("resume after a removed item = %v, want [d]", page)
	}
}

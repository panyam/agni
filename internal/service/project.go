package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// ProjectStore resolves the project/design descriptors on a deployment (agni issue 170). It is the
// fourth injected port in this package, after PartSpecStore, AnnotationStore, and ReviewStore, and
// follows their shape: the interface lives here, the os-backed adapter lives in cmd/agni and owns all
// I/O (C1/C13).
//
// It is READ-ONLY, and deliberately so. Creating a project means authoring design intent from a
// design, which is a judgment step with a confidentiality boundary rather than a server operation, so
// there is nothing here that writes.
//
// The port is what keeps resolution an INTERFACE rather than a path convention. The adapter that
// ships walks a mount for `project.yaml` and `design.yaml`; another may consult an index, a database,
// or a PLM system. No caller can tell, because none of these methods names a file.
type ProjectStore interface {
	// Project returns one project by its declared id. An id naming nothing is ErrNotFound.
	Project(ctx context.Context, projectID string) (ProjectInfo, error)
	// Projects returns every visible project, ordered by id.
	Projects(ctx context.Context) ([]ProjectInfo, error)
	// Design returns one design within a project. Either id naming nothing is ErrNotFound.
	Design(ctx context.Context, projectID, designID string) (DesignInfo, error)
	// Designs returns a project's designs, ordered by id. A project id naming nothing is ErrNotFound,
	// distinct from a project that exists and holds no designs yet (an empty slice).
	Designs(ctx context.Context, projectID string) ([]DesignInfo, error)
	// Resolve maps a mount-relative ref (a design file, or the design folder itself) to the design
	// containing it and that design's project, reporting whether one was found.
	//
	// Not found is reported as ok=false with a nil error, never as ErrNotFound. A ref that belongs to
	// no declared design is the ORDINARY case on a mounted folder, and a store that raised an error
	// for it would make callers treat the common path as exceptional.
	Resolve(ctx context.Context, mount, ref string) (DesignInfo, ProjectInfo, bool, error)
}

// ProjectInfo is one project as the port reports it: the descriptor's declared identity plus where
// its files are. It is the runtime-neutral twin of webapi.Project, in the same way MountInfo is of
// webapi.Mount, so an adapter never constructs a wire message.
type ProjectInfo struct {
	// ID is the operator-declared id from `project.yaml`, the last segment of "projects/{id}".
	ID string
	// Title is the human-readable label; the adapter has already applied the id fallback.
	Title string
	// Mount is the workspace mount the project's files live in.
	Mount string
	// DirRef is the mount-relative folder holding the descriptor.
	DirRef string
}

// DesignInfo is one design as the port reports it.
type DesignInfo struct {
	// ProjectID is the id of the project this design belongs to. It is always set: a design is
	// addressable only under a parent, so a store never reports a parentless one.
	ProjectID string
	// ID is the operator-declared id from `design.yaml`.
	ID string
	// Title is the human-readable label, id fallback already applied.
	Title string
	// Mount is the workspace mount the design's files live in.
	Mount string
	// DirRef is the mount-relative design folder.
	DirRef string
	// EntryRef is the mount-relative ref of the file analysis reads (C21).
	EntryRef string
	// CompanionRefs are mount-relative refs to views of this same design: a schematic export, a
	// board, an IPC-2581. They are geometry to render and locate on, never a second component source.
	CompanionRefs []string
}

// Resource-name prefixes and separators, written once so a store dealing in ids and an API dealing in
// names cannot each spell the boundary by hand — which is how one of them ends up storing a name as
// an id.
const (
	projectNamePrefix = "projects/"
	designNameInfix   = "/designs/"
)

// ProjectName builds a project resource name from its declared id.
func ProjectName(id string) string { return projectNamePrefix + id }

// ProjectID extracts the declared id from a project resource name, reporting whether the name was
// well formed. A missing prefix, an empty id, or a separator inside the id is rejected: an id reaches
// a filesystem-backed adapter, so a caller must not be able to steer it out of the tree it was meant
// to address.
func ProjectID(name string) (string, bool) {
	id, ok := strings.CutPrefix(name, projectNamePrefix)
	if !ok {
		return "", false
	}
	return id, validResourceID(id)
}

// DesignName builds a design resource name, "projects/{project}/designs/{design}".
func DesignName(projectID, designID string) string {
	return projectNamePrefix + projectID + designNameInfix + designID
}

// DesignID splits a design resource name into its project and design ids, with the same containment
// rejections as ProjectID applied to both halves.
func DesignID(name string) (projectID, designID string, ok bool) {
	rest, found := strings.CutPrefix(name, projectNamePrefix)
	if !found {
		return "", "", false
	}
	projectID, designID, found = strings.Cut(rest, designNameInfix)
	if !found || !validResourceID(projectID) || !validResourceID(designID) {
		return "", "", false
	}
	return projectID, designID, true
}

// validResourceID rejects an id that could escape the store's tree or split one resource name into
// two. It is a containment check rather than a full syntax check: the descriptor loader already
// validated the id's shape when it parsed the file, and this guards the path from the wire.
func validResourceID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\")
}

// defaultProjectPageSize and maxProjectPageSize bound a listing. Projects and designs are counted in
// tens on a deployment, not thousands, so the default is generous enough that a client normally
// paginates once and never again.
const (
	defaultProjectPageSize = 50
	maxProjectPageSize     = 200
)

// ProjectService serves the project and design resources over an injected ProjectStore (C13). It does
// no file I/O itself: it maps the port's value types to proto, applies AIP-158 pagination and the one
// supported AIP-160 filter, and classifies errors for the transport.
//
// It is AIP-shaped with GET and LIST only, no mutators — the read-only-resource case in CONSTRAINTS
// C23. What earns it a resource name rather than a (mount, ref) pair is that a project's identity is
// DECLARED by an operator rather than derived from its path, so the name survives the folder being
// renamed or moved between mounts, and so reviews can later be parented by it.
type ProjectService struct {
	store ProjectStore
}

// NewProjectService returns a ProjectService backed by store. A nil store is legal and means this
// deployment declares no projects: every method then answers as though nothing resolved, rather than
// failing, because "no descriptors here" is a configuration a server is expected to run in.
func NewProjectService(store ProjectStore) *ProjectService {
	return &ProjectService{store: store}
}

// GetProject returns one project by resource name. A malformed name is ErrInvalidArgument and a
// well-formed name for a project that does not exist is ErrNotFound, so a client can tell a typo from
// a project it lacks.
func (s *ProjectService) GetProject(ctx context.Context, req *webapi.GetProjectRequest) (*webapi.Project, error) {
	id, ok := ProjectID(req.GetName())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a project resource name (want \"projects/{project}\")", ErrInvalidArgument, req.GetName())
	}
	if s.store == nil {
		return nil, fmt.Errorf("%w: no project %q", ErrNotFound, req.GetName())
	}
	p, err := s.store.Project(ctx, id)
	if err != nil {
		return nil, err
	}
	return projectProto(p), nil
}

// ListProjects returns the projects visible across the server's mounts, ordered by resource name.
func (s *ProjectService) ListProjects(ctx context.Context, req *webapi.ListProjectsRequest) (*webapi.ListProjectsResponse, error) {
	mount, err := parseProjectFilter(req.GetFilter())
	if err != nil {
		return nil, err
	}
	var all []ProjectInfo
	if s.store != nil {
		if all, err = s.store.Projects(ctx); err != nil {
			return nil, err
		}
	}
	var kept []ProjectInfo
	for _, p := range all {
		if mount == "" || p.Mount == mount {
			kept = append(kept, p)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].ID < kept[j].ID })
	page, next := paginate(len(kept), req.GetPageSize(), req.GetPageToken(), func(i int) string { return kept[i].ID })
	resp := &webapi.ListProjectsResponse{NextPageToken: next}
	for _, i := range page {
		resp.Projects = append(resp.Projects, projectProto(kept[i]))
	}
	return resp, nil
}

// GetDesign returns one design by resource name, with the same malformed-vs-absent split as
// GetProject.
func (s *ProjectService) GetDesign(ctx context.Context, req *webapi.GetProjectDesignRequest) (*webapi.Design, error) {
	projectID, designID, ok := DesignID(req.GetName())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a design resource name (want \"projects/{project}/designs/{design}\")", ErrInvalidArgument, req.GetName())
	}
	if s.store == nil {
		return nil, fmt.Errorf("%w: no design %q", ErrNotFound, req.GetName())
	}
	d, err := s.store.Design(ctx, projectID, designID)
	if err != nil {
		return nil, err
	}
	return designProto(d), nil
}

// ListDesigns returns one project's designs, ordered by resource name. A parent naming a project that
// does not exist is ErrNotFound rather than an empty list, because "this project has no designs" and
// "there is no such project" are different answers and only one of them means the client's parent was
// wrong.
func (s *ProjectService) ListDesigns(ctx context.Context, req *webapi.ListProjectDesignsRequest) (*webapi.ListProjectDesignsResponse, error) {
	projectID, ok := ProjectID(req.GetParent())
	if !ok {
		return nil, fmt.Errorf("%w: parent %q is not a project resource name (want \"projects/{project}\")", ErrInvalidArgument, req.GetParent())
	}
	mount, err := parseProjectFilter(req.GetFilter())
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("%w: no project %q", ErrNotFound, req.GetParent())
	}
	all, err := s.store.Designs(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var kept []DesignInfo
	for _, d := range all {
		if mount == "" || d.Mount == mount {
			kept = append(kept, d)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].ID < kept[j].ID })
	page, next := paginate(len(kept), req.GetPageSize(), req.GetPageToken(), func(i int) string { return kept[i].ID })
	resp := &webapi.ListProjectDesignsResponse{NextPageToken: next}
	for _, i := range page {
		resp.Designs = append(resp.Designs, designProto(kept[i]))
	}
	return resp, nil
}

// ResolveDesign answers whether a mount-relative ref belongs to a declared design, and if so which.
//
// A ref that resolves to nothing yields an EMPTY response, not an error. That is the load-bearing
// part of the contract: most files on a mount belong to no declared design, and a client reading an
// empty response shows the plain viewer and the built-in catalog rather than another project's
// config. Turning the ordinary case into an error would push every caller into treating a failure
// path as normal, which is how a real failure stops being noticed.
func (s *ProjectService) ResolveDesign(ctx context.Context, req *webapi.ResolveDesignRequest) (*webapi.ResolveDesignResponse, error) {
	if s.store == nil {
		return &webapi.ResolveDesignResponse{}, nil
	}
	d, p, ok, err := s.store.Resolve(ctx, req.GetMount(), req.GetRef())
	if err != nil {
		return nil, err
	}
	if !ok {
		return &webapi.ResolveDesignResponse{}, nil
	}
	return &webapi.ResolveDesignResponse{Design: designProto(d), Project: projectProto(p)}, nil
}

// parseProjectFilter reads the one supported AIP-160 filter, `mount="..."`, returning the mount or ""
// for an empty filter.
//
// An unsupported filter is an ERROR rather than an ignored argument, for the same reason
// parseReviewFilter refuses one: a client that believed it had narrowed to one mount and silently got
// every mount would read another team's projects as its own.
func parseProjectFilter(filter string) (string, error) {
	f := strings.TrimSpace(filter)
	if f == "" {
		return "", nil
	}
	value, ok := strings.CutPrefix(f, "mount=")
	if !ok {
		return "", fmt.Errorf("%w: filter %q is not supported; the only supported filter is mount=\"<name>\"", ErrInvalidArgument, filter)
	}
	value = strings.Trim(strings.TrimSpace(value), `"`)
	if value == "" {
		return "", fmt.Errorf("%w: filter %q has an empty mount", ErrInvalidArgument, filter)
	}
	return value, nil
}

// paginate applies AIP-158 page_size / page_token to a sorted collection of n items, returning the
// indexes on this page and the token for the next.
//
// The token is the id of the FIRST item on the next page rather than an offset, so a listing stays
// correct when a project is added or removed between calls: an offset would silently skip or repeat a
// neighbour, where an id resumes at the right place or, if that item is gone, at the next one after
// it. Ordering is by id and ids are unique within a collection, which is what makes that resumable.
func paginate(n int, pageSize int32, pageToken string, idAt func(int) string) (indexes []int, nextToken string) {
	size := int(pageSize)
	switch {
	case size <= 0:
		size = defaultProjectPageSize
	case size > maxProjectPageSize:
		size = maxProjectPageSize
	}
	start := 0
	if pageToken != "" {
		start = sort.Search(n, func(i int) bool { return idAt(i) >= pageToken })
	}
	end := min(start+size, n)
	for i := start; i < end; i++ {
		indexes = append(indexes, i)
	}
	if end < n {
		nextToken = idAt(end)
	}
	return indexes, nextToken
}

func projectProto(p ProjectInfo) *webapi.Project {
	return &webapi.Project{
		Name:   ProjectName(p.ID),
		Title:  p.Title,
		Mount:  p.Mount,
		DirRef: p.DirRef,
	}
}

func designProto(d DesignInfo) *webapi.Design {
	return &webapi.Design{
		Name:          DesignName(d.ProjectID, d.ID),
		Title:         d.Title,
		Mount:         d.Mount,
		DirRef:        d.DirRef,
		EntryRef:      d.EntryRef,
		CompanionRefs: d.CompanionRefs,
	}
}

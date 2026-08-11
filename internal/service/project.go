package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
)

// ProjectStore resolves the projects and designs a deployment declares (agni issue 170). It is the
// fourth injected port in this package, after PartSpecStore, AnnotationStore, and ReviewStore, and
// follows their shape: the interface lives here, an implementation lives behind it.
//
// The port is deliberately free of any notion of a filesystem. The implementation that ships
// (`internal/projects`) walks a directory tree for descriptor files, but a store backed by a
// database, with design files on object storage, answers all five of these without a tree, a
// descriptor, or a parent directory to walk up from. Nothing in these signatures names a file, a
// path, or a walk, which is what makes that substitution a wiring change rather than a redesign.
//
// It is READ-ONLY, and deliberately so. Creating a project means authoring design intent from a
// design, which is a judgment step with a confidentiality boundary rather than a server operation.
//
// Every method deals in the WIRE types. There is no parallel value type for a project here, and
// that is a decision rather than an omission: a `Project` is a resource whose whole content is the
// message, so a runtime-neutral twin would be a field-for-field copy and one more place for the two
// to disagree (CONSTRAINTS C2, C8). This differs from `MountInfo` / `DirEntry` above, which project
// a raw filesystem listing into something the service still has to interpret.
type ProjectStore interface {
	// Project returns one project by resource name. A name naming nothing is ErrNotFound.
	Project(ctx context.Context, name string) (*webapi.Project, error)
	// Projects returns every visible project, ordered by resource name.
	Projects(ctx context.Context) ([]*webapi.Project, error)
	// Design returns one design by resource name. A name naming nothing is ErrNotFound.
	Design(ctx context.Context, name string) (*webapi.Design, error)
	// Designs returns a project's designs, ordered by resource name. A parent naming nothing is
	// ErrNotFound, distinct from a project that exists and holds no designs (an empty slice).
	Designs(ctx context.Context, parent string) ([]*webapi.Design, error)
	// ResolveDesign maps an artifact ref to the design containing it and that design's project.
	//
	// A MISS is (nil, nil, nil), never ErrNotFound. A ref that belongs to no declared design is the
	// ORDINARY case on a mounted folder, and a store that raised an error for it would make callers
	// treat the common path as exceptional.
	ResolveDesign(ctx context.Context, mount, ref string) (*webapi.Design, *webapi.Project, error)
}

// Resource-name prefixes and separators, written once so a store dealing in ids and an API dealing
// in names cannot each spell the boundary by hand — which is how one of them ends up storing a name
// as an id.
const (
	projectNamePrefix = "projects/"
	designNameInfix   = "/designs/"
)

// ProjectName builds a project resource name from its declared id.
func ProjectName(id string) string { return projectNamePrefix + id }

// ProjectID extracts the declared id from a project resource name, reporting whether the name was
// well formed. A missing prefix, an empty id, or a separator inside the id is rejected: an id
// reaches a store that may resolve it against a filesystem, so a caller must not be able to steer it
// out of the tree it was meant to address.
func ProjectID(name string) (string, bool) {
	id, ok := strings.CutPrefix(name, projectNamePrefix)
	if !ok {
		return "", false
	}
	return id, validResourceID(id)
}

// DesignName builds a design resource name, "projects/{project}/designs/{design}".
func DesignName(parent, id string) string { return parent + designNameInfix + id }

// SplitDesignName splits a design resource name into its parent project name and its design id,
// with the same containment rejections as ProjectID applied to both halves.
func SplitDesignName(name string) (parent, id string, ok bool) {
	rest, found := strings.CutPrefix(name, projectNamePrefix)
	if !found {
		return "", "", false
	}
	projectID, designID, found := strings.Cut(rest, designNameInfix)
	if !found || !validResourceID(projectID) || !validResourceID(designID) {
		return "", "", false
	}
	return ProjectName(projectID), designID, true
}

// validResourceID rejects an id that could escape a store's tree or split one resource name into
// two. It is a containment check rather than a full syntax check: the descriptor loader already
// validated the id's shape when it parsed the file, and this guards the id arriving from the wire.
func validResourceID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\")
}

// DesignSources is which artifact each tier of a read comes from, once a design's declaration has
// had its say. They are REFS, not paths: a ref is a key the injected Loader resolves, and no caller
// above the Loader may treat one as a host path (CONSTRAINTS C22).
type DesignSources struct {
	// NetlistRef is the artifact component and connectivity ANALYSIS reads: the netlist the design
	// team produces (C21).
	NetlistRef string
	// BoardRef is where the board tier's copper comes from. Often the same artifact, and different
	// when a design declares a separate board companion.
	BoardRef string
	// GeometryRef is where schematic sheets are rendered and findings located from. A netlist entry
	// carries none of its own, so a design declaring a schematic companion locates on that
	// companion's sheets, which is what C21 means by a companion being a canvas rather than a source.
	GeometryRef string
}

// SourcesFor resolves a design's declaration into the artifact each tier reads.
//
// It lives here, above the store and below every client, because CLI and web must not each decide
// which companion supplies the board. Two implementations of "pick the companion with copper" is one
// more than the number of answers that can be right, and the failure mode is silent: a tier resolved
// differently in two places produces two findings lists with nothing to say why they differ.
//
// `named` is the ref the caller actually asked for, "" when they named the design itself. A named
// companion KEEPS whatever tier it alone can supply — a board file's copper, a schematic's sheets —
// because that is why the caller pointed at it; only the netlist tier ever moves.
func SourcesFor(d *webapi.Design, named string) DesignSources {
	entry := d.GetEntryRef()
	s := DesignSources{NetlistRef: entry, BoardRef: entry, GeometryRef: entry}
	if named != "" {
		if formats.HasBoard(named) {
			s.BoardRef = named
		}
		if formats.HasFaithful(named) {
			s.GeometryRef = named
		}
	}
	for _, c := range d.GetCompanionRefs() {
		if s.BoardRef == entry && formats.HasBoard(c) {
			s.BoardRef = c
		}
		if s.GeometryRef == entry && formats.HasFaithful(c) {
			s.GeometryRef = c
		}
	}
	return s
}

// IsCompanion reports whether ref is one of the views this design declared of itself.
func IsCompanion(d *webapi.Design, ref string) bool {
	for _, c := range d.GetCompanionRefs() {
		if c == ref {
			return true
		}
	}
	return false
}

// defaultProjectPageSize and maxProjectPageSize bound a listing. Projects and designs are counted in
// tens on a deployment, not thousands, so the default is generous enough that a client normally
// paginates once and never again.
const (
	defaultProjectPageSize = 50
	maxProjectPageSize     = 200
)

// ProjectService serves the project and design resources over an injected ProjectStore (C13). It
// does no I/O itself: it applies AIP-158 pagination and the one supported AIP-160 filter, and
// classifies errors for the transport.
//
// It is AIP-shaped with GET and LIST only, no mutators — the read-only-resource case in CONSTRAINTS
// C23. What earns it a resource name rather than a (mount, ref) pair is that a project's identity is
// DECLARED by an operator rather than derived from its path, so the name survives the folder being
// renamed or moved between mounts, and so reviews can later be parented by it.
//
// Every caller goes through here, including the CLI. That is not ceremony: the alternative is a
// second access path reaching the store directly, and the two then drift on exactly the questions
// that are invisible from the outside — which companion supplies the board, whether an unresolved
// ref is an error, what a malformed descriptor does.
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
// well-formed name for a project that does not exist is ErrNotFound, so a client can tell a typo
// from a project it lacks.
func (s *ProjectService) GetProject(ctx context.Context, req *webapi.GetProjectRequest) (*webapi.Project, error) {
	if _, ok := ProjectID(req.GetName()); !ok {
		return nil, fmt.Errorf("%w: %q is not a project resource name (want \"projects/{project}\")", ErrInvalidArgument, req.GetName())
	}
	if s.store == nil {
		return nil, fmt.Errorf("%w: no project %q", ErrNotFound, req.GetName())
	}
	return s.store.Project(ctx, req.GetName())
}

// ListProjects returns the projects visible across the server's mounts, ordered by resource name.
func (s *ProjectService) ListProjects(ctx context.Context, req *webapi.ListProjectsRequest) (*webapi.ListProjectsResponse, error) {
	mount, err := parseProjectFilter(req.GetFilter())
	if err != nil {
		return nil, err
	}
	var all []*webapi.Project
	if s.store != nil {
		if all, err = s.store.Projects(ctx); err != nil {
			return nil, err
		}
	}
	var kept []*webapi.Project
	for _, p := range all {
		if mount == "" || p.GetMount() == mount {
			kept = append(kept, p)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].GetName() < kept[j].GetName() })
	page, next := paginate(len(kept), req.GetPageSize(), req.GetPageToken(), func(i int) string { return kept[i].GetName() })
	resp := &webapi.ListProjectsResponse{NextPageToken: next}
	for _, i := range page {
		resp.Projects = append(resp.Projects, kept[i])
	}
	return resp, nil
}

// GetDesign returns one design by resource name, with the same malformed-vs-absent split as
// GetProject.
func (s *ProjectService) GetDesign(ctx context.Context, req *webapi.GetProjectDesignRequest) (*webapi.Design, error) {
	if _, _, ok := SplitDesignName(req.GetName()); !ok {
		return nil, fmt.Errorf("%w: %q is not a design resource name (want \"projects/{project}/designs/{design}\")", ErrInvalidArgument, req.GetName())
	}
	if s.store == nil {
		return nil, fmt.Errorf("%w: no design %q", ErrNotFound, req.GetName())
	}
	return s.store.Design(ctx, req.GetName())
}

// ListDesigns returns one project's designs, ordered by resource name. A parent naming a project
// that does not exist is ErrNotFound rather than an empty list, because "this project has no
// designs" and "there is no such project" are different answers and only one of them means the
// client's parent was wrong.
func (s *ProjectService) ListDesigns(ctx context.Context, req *webapi.ListProjectDesignsRequest) (*webapi.ListProjectDesignsResponse, error) {
	if _, ok := ProjectID(req.GetParent()); !ok {
		return nil, fmt.Errorf("%w: parent %q is not a project resource name (want \"projects/{project}\")", ErrInvalidArgument, req.GetParent())
	}
	mount, err := parseProjectFilter(req.GetFilter())
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, fmt.Errorf("%w: no project %q", ErrNotFound, req.GetParent())
	}
	all, err := s.store.Designs(ctx, req.GetParent())
	if err != nil {
		return nil, err
	}
	var kept []*webapi.Design
	for _, d := range all {
		if mount == "" || d.GetMount() == mount {
			kept = append(kept, d)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].GetName() < kept[j].GetName() })
	page, next := paginate(len(kept), req.GetPageSize(), req.GetPageToken(), func(i int) string { return kept[i].GetName() })
	resp := &webapi.ListProjectDesignsResponse{NextPageToken: next}
	for _, i := range page {
		resp.Designs = append(resp.Designs, kept[i])
	}
	return resp, nil
}

// ResolveDesign answers whether an artifact ref belongs to a declared design, and if so which.
//
// A ref that resolves to nothing yields an EMPTY response, not an error. That is the load-bearing
// part of the contract: most files on a mount belong to no declared design, and a client reading an
// empty response falls back to the plain built-in catalog rather than another project's config.
// Turning the ordinary case into an error would push every caller into treating a failure path as
// normal, which is how a real failure stops being noticed.
func (s *ProjectService) ResolveDesign(ctx context.Context, req *webapi.ResolveDesignRequest) (*webapi.ResolveDesignResponse, error) {
	if s.store == nil {
		return &webapi.ResolveDesignResponse{}, nil
	}
	d, p, err := s.store.ResolveDesign(ctx, req.GetMount(), req.GetRef())
	if err != nil {
		return nil, err
	}
	if d == nil {
		return &webapi.ResolveDesignResponse{}, nil
	}
	return &webapi.ResolveDesignResponse{Design: d, Project: p}, nil
}

// parseProjectFilter reads the one supported AIP-160 filter, `mount="..."`, returning the mount or
// "" for an empty filter.
//
// An unsupported filter is an ERROR rather than an ignored argument, for the same reason
// parseReviewFilter refuses one: a client that believed it had narrowed to one mount and silently
// got every mount would read another team's projects as its own.
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
// The token is the resource name of the FIRST item on the next page rather than an offset, so a
// listing stays correct when a project is added or removed between calls: an offset would silently
// skip or repeat a neighbour, where a name resumes at the right place or, if that item is gone, at
// the next one after it. Ordering is by name and names are unique, which is what makes that
// resumable.
func paginate(n int, pageSize int32, pageToken string, nameAt func(int) string) (indexes []int, nextToken string) {
	size := int(pageSize)
	switch {
	case size <= 0:
		size = defaultProjectPageSize
	case size > maxProjectPageSize:
		size = maxProjectPageSize
	}
	start := 0
	if pageToken != "" {
		start = sort.Search(n, func(i int) bool { return nameAt(i) >= pageToken })
	}
	end := min(start+size, n)
	for i := start; i < end; i++ {
		indexes = append(indexes, i)
	}
	if end < n {
		nextToken = nameAt(end)
	}
	return indexes, nextToken
}

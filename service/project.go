package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"

	"github.com/panyam/goapplib/storage/mounts"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
)

// ProjectStore resolves the projects and designs a deployment declares (agni issue 170). Like
// ReviewStore here, and DraftStore and AnnotationStore in datasheet/dsservice, the interface is a
// port and an implementation lives behind it.
//
// Nothing in these signatures names a file, a path or a walk. The shipped implementation
// (`internal/projects`) walks a directory tree for descriptors, and a database-backed store with
// designs on object storage should be a wiring change rather than a redesign.
//
// It reads the projects a deployment declares in its mounted folders, and each one's designs are
// the designs whose `extends` names it. The projects a server stores are ProjectRecords, and the
// designs themselves are read and written through a DesignCatalog (agni issues 1693, 1694).
//
// Every method deals in the WIRE types, with no parallel value type. A `Project` is a resource whose
// content is the message, so a twin would be a field-for-field copy that can disagree with it
// (CONSTRAINTS C2, C8). `MountInfo` and `DirEntry` in workspace.go differ because they project a raw
// filesystem listing the service still has to interpret.
type ProjectStore interface {
	// Project returns one project by resource name. A name naming nothing is ErrNotFound.
	Project(ctx context.Context, name string) (*webapi.Project, error)
	// Projects returns every visible project, ordered by resource name. Each lists its designs, the
	// ones whose `extends` names it.
	Projects(ctx context.Context) ([]*webapi.Project, error)
	// ResolveDesign maps an artifact ref to the design containing it and that design's project.
	//
	// A MISS is (nil, nil, nil), never ErrNotFound. A ref that belongs to no declared design is the
	// ORDINARY case on a mounted folder.
	ResolveDesign(ctx context.Context, uri mounts.URI) (*webapi.Design, *webapi.Project, error)
}

// Resource-name prefixes and separators, written once. A store deals in ids and an API deals in
// names. If each spelled the boundary by hand, one would end up storing a name as an id.
const projectNamePrefix = "projects/"

// ProjectName builds a project resource name from its declared id.
func ProjectName(id string) string { return projectNamePrefix + id }

// ProjectID extracts the declared id from a project resource name, reporting whether the name was
// well formed. A missing prefix, an empty id, or a separator inside the id is rejected, because the id
// reaches a store that may resolve it against a filesystem and must not be steerable out of its tree.
func ProjectID(name string) (string, bool) {
	id, ok := strings.CutPrefix(name, projectNamePrefix)
	if !ok {
		return "", false
	}
	return id, validResourceID(id)
}

// validResourceID rejects an id that could escape a store's tree or split one resource name into
// two. It checks containment only. The descriptor loader validated the id's shape when it parsed the
// file, and this guards the id arriving from the wire.
func validResourceID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\")
}

// DesignSources names the artifact each tier of a read comes from, after the design's declaration
// is applied. They are URIs, the injected Loader resolves them, and no caller above the Loader may
// treat one as a host path (CONSTRAINTS C22).
type DesignSources struct {
	// NetlistURI names the netlist the design team produces (C21). Component and connectivity
	// ANALYSIS reads it.
	NetlistURI string
	// BoardURI is where the board tier's copper comes from. It is often the same artifact, and it
	// differs when a design declares a separate board companion.
	BoardURI string
	// GeometryURI names the artifact that schematic sheets are rendered from and findings are located
	// on. A netlist entry has no geometry of its own, so a design declaring a schematic companion
	// locates on that companion's sheets. Under C21 a companion is a canvas and not a source.
	GeometryURI string
}

// SourcesFor resolves a design's declaration into the artifact each tier reads.
//
// It lives here, above the store and below every client, so CLI and web do not each decide which
// companion supplies the board. A tier resolved differently in two places gives two findings lists
// with nothing to say why they differ.
//
// `named` is the ref the caller actually asked for, "" when they named the design itself. A named
// companion KEEPS whatever tier it alone can supply (a board file's copper, a schematic's sheets),
// and only the netlist tier ever moves.
func SourcesFor(d *webapi.Design, named string) DesignSources {
	entry := d.GetEntryUri()
	s := DesignSources{NetlistURI: entry, BoardURI: entry, GeometryURI: entry}
	if named != "" {
		if formats.HasBoard(named) {
			s.BoardURI = named
		}
		if formats.HasFaithful(named) {
			s.GeometryURI = named
		}
	}
	for _, c := range d.GetCompanionUris() {
		if s.BoardURI == entry && formats.HasBoard(c) {
			s.BoardURI = c
		}
		if s.GeometryURI == entry && formats.HasFaithful(c) {
			s.GeometryURI = c
		}
	}
	return s
}

// ResolveSources decides which artifact each tier reads, for a ref that has already been resolved to
// a design. It is the DECISION half of resolution and does no I/O. The caller finds the design, and
// this says what to open. The CLI finds it over a store rooted at its argument's mount, and a server
// over the store it was built with. CLI and server both call it, so one design cannot read two ways
// (agni issue 656, constraint C32).
//
// The four cases:
//
//   - `d` is nil, so the ref belongs to no declared design. Read exactly what was named, which is
//     the ordinary case for any mounted folder without descriptors.
//   - The ref names the design itself (a folder) or its ENTRY. It gets the design's declared
//     companions, because naming the entry IS naming the design.
//   - The ref names a declared COMPANION. Analysis moves to the entry and the named artifact keeps
//     whatever tier it alone supplies.
//   - The ref names an UNDECLARED sibling. Read exactly what was named, since redirection is
//     confined to files an operator listed (docsite/content/architecture/projects-and-designs.md#the-netlist-is-the-source-the-rest-are-views).
//
// asNamed disables redirection, so the artifact named is read whatever the descriptor says. Reading
// a companion AS a netlist is a legitimate diagnostic.
func ResolveSources(d *webapi.Design, ref string, isDir, asNamed bool) Resolution {
	plain := Resolution{DesignSources: DesignSources{NetlistURI: ref, BoardURI: ref, GeometryURI: ref}}
	if d == nil {
		return plain
	}
	r := Resolution{
		NamedIsTheDesign: isDir && ref == d.GetUri(),
		NamedIsTheEntry:  !isDir && ref == d.GetEntryUri(),
	}
	// asNamed does NOT apply to the design itself. A folder is not a readable artifact, so "read
	// exactly what I named" has no meaning for one and would resolve every tier to a directory.
	if !r.NamedIsTheDesign && (asNamed || !(r.NamedIsTheEntry || IsCompanion(d, ref))) {
		return plain
	}
	// Naming the design or its entry asks for the design as declared, so no tier is pinned to the
	// ref. Naming a companion pins the tiers only that companion can supply.
	from := ref
	if r.NamedIsTheDesign || r.NamedIsTheEntry {
		from = ""
	}
	r.DesignSources = SourcesFor(d, from)
	r.FromDeclaration = true
	return r
}

// Resolution records what a ref resolved to. It names the artifact each tier reads and how the
// resolver chose it.
//
// The struct holds the flags so a caller does not recompute them. Imagine a caller re-deriving "was
// this the entry" against the path it typed instead of the ref it resolved. It compares unlike with
// unlike and narrates the wrong thing. The CLI's stderr note is written from these flags.
type Resolution struct {
	DesignSources
	// NamedIsTheDesign is set when the ref named the design itself, which is a folder.
	NamedIsTheDesign bool
	// NamedIsTheEntry is set when the ref named the design's declared entry file.
	NamedIsTheEntry bool
	// FromDeclaration is set when the tiers came from the design's declaration rather than from the
	// ref alone. It does not mean a tier moved. Imagine a ref naming the entry of a design that
	// declares no companion. The declaration applies and changes nothing. A caller narrating what it
	// read still tells that case apart from a ref that was never resolved.
	FromDeclaration bool
}

// IsCompanion reports whether ref is one of the views this design declared of itself.
func IsCompanion(d *webapi.Design, ref string) bool {
	for _, c := range d.GetCompanionUris() {
		if c == ref {
			return true
		}
	}
	return false
}

// defaultProjectPageSize and maxProjectPageSize bound a listing. Projects and designs are counted in
// tens on a deployment, so a client normally gets everything in one page.
const (
	defaultProjectPageSize = 50
	maxProjectPageSize     = 200
)

// ProjectService serves the project resource over an injected ProjectStore and ProjectRecords
// (C13). It does no I/O itself. It applies AIP-158 pagination and the one supported AIP-160 filter,
// and classifies errors for the transport.
//
// A project declared in a mounted folder is a read-only resource, and its id is DECLARED in its
// `project.yaml`, so the name survives the folder being renamed or moved between mounts. A project
// the server stores is a title and a list of designs, with the standard methods (CONSTRAINTS C23,
// agni issue 1694). A project id is unique across both.
//
// Every caller goes through here, including the CLI. A second path reaching the store directly would
// be a second place for the malformed-vs-absent rule to drift.
type ProjectService struct {
	store   ProjectStore
	records ProjectRecords
}

// ProjectRecords keeps the projects a server stores, each a title and a list of design names (agni
// issue 1694). Get of an id it does not hold is ErrNotFound, and so is Delete. Put creates or
// replaces. List returns every record ordered by name. Each record's name is "projects/{id}".
type ProjectRecords interface {
	Get(ctx context.Context, id string) (*webapi.Project, error)
	List(ctx context.Context) ([]*webapi.Project, error)
	Put(ctx context.Context, id string, p *webapi.Project) error
	Delete(ctx context.Context, id string) error
}

// NewProjectService returns a ProjectService backed by store. A nil store is legal and means this
// deployment declares no projects, so every method answers as though nothing resolved rather than
// failing.
func NewProjectService(store ProjectStore) *ProjectService {
	return &ProjectService{store: store}
}

// WithRecords keeps the projects callers create in records, and returns s. Without it, a write
// answers ErrFailedPrecondition.
func (s *ProjectService) WithRecords(records ProjectRecords) *ProjectService {
	s.records = records
	return s
}

// GetProject returns one project by resource name. A malformed name is ErrInvalidArgument and a
// well-formed name for a project that does not exist is ErrNotFound, so a client can tell a typo
// from a project it lacks.
func (s *ProjectService) GetProject(ctx context.Context, req *webapi.GetProjectRequest) (*webapi.Project, error) {
	id, ok := ProjectID(req.GetName())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a project resource name (want \"projects/{project}\")", ErrInvalidArgument, req.GetName())
	}
	if p, err := s.mounted(ctx, req.GetName()); err == nil || !errors.Is(err, ErrNotFound) {
		return p, err
	}
	if s.records != nil {
		return s.records.Get(ctx, id)
	}
	return nil, fmt.Errorf("%w: no project %q", ErrNotFound, req.GetName())
}

// mounted returns a project declared in a mounted folder, or ErrNotFound.
func (s *ProjectService) mounted(ctx context.Context, name string) (*webapi.Project, error) {
	if s.store == nil {
		return nil, fmt.Errorf("%w: no project %q", ErrNotFound, name)
	}
	return s.store.Project(ctx, name)
}

// ListProjects returns the projects declared in mounted folders and the ones the server stores,
// ordered by resource name. A stored project carries no URI, so a mount filter leaves it out.
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
	if s.records != nil {
		stored, err := s.records.List(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, stored...)
	}
	var kept []*webapi.Project
	seen := map[string]bool{}
	for _, p := range all {
		if seen[p.GetName()] {
			return nil, fmt.Errorf("%s is both declared in a mounted folder and stored; a project id is unique per server", p.GetName())
		}
		seen[p.GetName()] = true
		if mount == "" || uriMount(p.GetUri()) == mount {
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

// packNames checks a project's default packs, each "packs/{pack}" and named once (agni issue 1704). A
// pack named here need not exist yet, as a design a project lists need not.
func packNames(names []string) ([]string, error) {
	seen := map[string]bool{}
	for _, n := range names {
		if _, ok := PackID(n); !ok {
			return nil, fmt.Errorf("%w: %q is not a pack resource name, packs/{pack}", ErrInvalidArgument, n)
		}
		if seen[n] {
			return nil, fmt.Errorf("%w: %s is named twice", ErrInvalidArgument, n)
		}
		seen[n] = true
	}
	return names, nil
}

// CreateProject stores a new project. Its id must be one no mounted folder declares.
func (s *ProjectService) CreateProject(ctx context.Context, req *webapi.CreateProjectRequest) (*webapi.Project, error) {
	if s.records == nil {
		return nil, errNoProjectStore
	}
	id := req.GetProjectId()
	if !newIDPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: project id %q must be lowercase letters, digits, '.', '_' or '-', starting with a letter or digit", ErrInvalidArgument, id)
	}
	name := ProjectName(id)
	if _, err := s.mounted(ctx, name); err == nil {
		return nil, fmt.Errorf("%w: %s is declared in a mounted folder", ErrAlreadyExists, name)
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if _, err := s.records.Get(ctx, id); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyExists, name)
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	designs, err := designNames(req.GetProject().GetDesigns())
	if err != nil {
		return nil, err
	}
	packs, err := packNames(req.GetProject().GetDefaultPacks())
	if err != nil {
		return nil, err
	}
	p := &webapi.Project{Name: name, Title: orID(req.GetProject().GetTitle(), id), Designs: designs, DefaultPacks: packs}
	if err := s.records.Put(ctx, id, p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateProject changes a stored project's title, designs or default packs, the fields update_mask
// names.
func (s *ProjectService) UpdateProject(ctx context.Context, req *webapi.UpdateProjectRequest) (*webapi.Project, error) {
	id, p, err := s.stored(ctx, req.GetProject().GetName())
	if err != nil {
		return nil, err
	}
	paths := req.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		paths = []string{"title", "designs", "default_packs"}
	}
	for _, f := range paths {
		switch f {
		case "title":
			p.Title = orID(req.GetProject().GetTitle(), id)
		case "designs":
			if p.Designs, err = designNames(req.GetProject().GetDesigns()); err != nil {
				return nil, err
			}
		case "default_packs":
			if p.DefaultPacks, err = packNames(req.GetProject().GetDefaultPacks()); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%w: update_mask names %q; a stored project's title, designs and default_packs can change", ErrInvalidArgument, f)
		}
	}
	if err := s.records.Put(ctx, id, p); err != nil {
		return nil, err
	}
	return p, nil
}

// DeleteProject removes a stored project. The designs it lists stay.
func (s *ProjectService) DeleteProject(ctx context.Context, req *webapi.DeleteProjectRequest) error {
	id, _, err := s.stored(ctx, req.GetName())
	if err != nil {
		return err
	}
	return s.records.Delete(ctx, id)
}

var errNoProjectStore = fmt.Errorf("%w: this server keeps no projects; start it with --store", ErrFailedPrecondition)

// stored returns a project the server stores, refusing one a mounted folder declares.
func (s *ProjectService) stored(ctx context.Context, name string) (string, *webapi.Project, error) {
	id, ok := ProjectID(name)
	if !ok {
		return "", nil, fmt.Errorf("%w: %q is not a project resource name (want \"projects/{project}\")", ErrInvalidArgument, name)
	}
	if _, err := s.mounted(ctx, name); err == nil {
		return "", nil, fmt.Errorf("%w: %s is declared in a mounted folder, which is read-only", ErrFailedPrecondition, name)
	} else if !errors.Is(err, ErrNotFound) {
		return "", nil, err
	}
	if s.records == nil {
		return "", nil, errNoProjectStore
	}
	p, err := s.records.Get(ctx, id)
	return id, p, err
}

// newIDPattern is the shape of an id a caller picks, the shape a descriptor's `name` takes.
var newIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// designNames checks a project's design list. Each entry is "designs/{design}", and none repeats. A
// name need not name a design that exists, since a design can be deleted after a project lists it.
func designNames(names []string) ([]string, error) {
	seen := map[string]bool{}
	for _, n := range names {
		if _, ok := StandaloneDesignID(n); !ok {
			return nil, fmt.Errorf("%w: %q is not a design resource name, designs/{design}", ErrInvalidArgument, n)
		}
		if seen[n] {
			return nil, fmt.Errorf("%w: %s is listed twice", ErrInvalidArgument, n)
		}
		seen[n] = true
	}
	return slices.Clone(names), nil
}

func orID(title, id string) string {
	if strings.TrimSpace(title) == "" {
		return id
	}
	return title
}

// ResolveDesign answers whether an artifact ref belongs to a declared design, and if so which.
//
// A ref that resolves to nothing yields an EMPTY response, not an error. Most files on a mount belong
// to no declared design, and a client reading an empty response falls back to the plain built-in
// catalog rather than another project's config. Making that ordinary case an error would teach
// callers to ignore the failure path.
func (s *ProjectService) ResolveDesign(ctx context.Context, req *webapi.ResolveDesignRequest) (*webapi.ResolveDesignResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return &webapi.ResolveDesignResponse{}, nil
	}
	d, p, err := s.store.ResolveDesign(ctx, u)
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
// An unsupported filter is an ERROR rather than an ignored argument, as in parseReviewFilter. A
// client that believed it had narrowed to one mount and silently got every mount would read another
// team's projects as its own.
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
// project added or removed between calls does not skip or repeat a neighbour. If that item is gone,
// the page resumes at the next name after it. Callers must sort by name, and names are unique.
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

// uriMount reads the authority out of a resource's artifact URI, for the one filter that narrows by
// mount. A URI that will not parse yields "", which never matches. A listing does not fail on a
// stored value, and the resource itself carries the malformed URI for a client to see.
func uriMount(s string) string {
	u, err := mounts.ParseURI(s)
	if err != nil {
		return ""
	}
	return u.Mount
}

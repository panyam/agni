package service

import (
	"context"
	"fmt"
	"github.com/panyam/agni/artifact"
	"slices"
	"sort"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
)

// ProjectStore resolves the projects and designs a deployment declares (agni issue 170). Like
// PartSpecStore, AnnotationStore and ReviewStore, the interface lives here and an implementation
// lives behind it.
//
// Nothing in these signatures names a file, a path or a walk. The shipped implementation
// (`internal/projects`) walks a directory tree for descriptors, and a database-backed store with
// designs on object storage should be a wiring change rather than a redesign.
//
// It is READ-ONLY. Creating a project means authoring design intent from a design, which is a
// judgment step with a confidentiality boundary rather than a server operation.
//
// Every method deals in the WIRE types, with no parallel value type. A `Project` is a resource whose
// content is the message, so a twin would be a field-for-field copy that can disagree with it
// (CONSTRAINTS C2, C8). `MountInfo` and `DirEntry` in workspace.go differ because they project a raw
// filesystem listing the service still has to interpret.
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
	// ORDINARY case on a mounted folder.
	ResolveDesign(ctx context.Context, uri artifact.URI) (*webapi.Design, *webapi.Project, error)
}

// Resource-name prefixes and separators, written once so a store dealing in ids and an API dealing
// in names do not each spell the boundary by hand and end up storing a name as an id.
const (
	projectNamePrefix = "projects/"
	designNameInfix   = "/designs/"
)

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
// two. It checks containment only. The descriptor loader validated the id's shape when it parsed the
// file, and this guards the id arriving from the wire.
func validResourceID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\")
}

// DesignSources is which artifact each tier of a read comes from, once a design's declaration has
// had its say. They are URIs the injected Loader resolves, and no caller above the Loader may treat
// one as a host path (CONSTRAINTS C22).
type DesignSources struct {
	// NetlistURI is the artifact component and connectivity ANALYSIS reads, the netlist the design
	// team produces (C21).
	NetlistURI string
	// BoardURI is where the board tier's copper comes from. Often the same artifact, and different
	// when a design declares a separate board companion.
	BoardURI string
	// GeometryURI is where schematic sheets are rendered and findings located from. A netlist entry
	// carries none of its own, so a design declaring a schematic companion locates on that
	// companion's sheets (C21, a companion is a canvas and not a source).
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
// a design. It is the DECISION half of resolution and does no I/O. The caller finds the design (the
// CLI over a store rooted at its argument's mount, a server over the store it was built with) and
// this says what to open. CLI and server both call it, so one design cannot read two ways (agni
// issue 656, constraint C32).
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
	// A ref naming a declared revision's entry or one of its companions resolves against THAT
	// revision, as though it were the design, so it gets its own companions and never another
	// revision's (agni issue 848).
	if !isDir {
		if rev := RevisionOf(d, ref); rev != nil {
			d = rev
		}
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

// Resolution is what a ref resolved to, meaning the artifact each tier reads and how that was
// arrived at.
//
// The flags are carried so a caller does not recompute them. Re-deriving "was this the entry" against
// the path it typed instead of the ref it resolved compares unlike with unlike and narrates the wrong
// thing. The CLI's stderr note is written from these.
type Resolution struct {
	DesignSources
	// NamedIsTheDesign is set when the ref named the design itself, which is a folder.
	NamedIsTheDesign bool
	// NamedIsTheEntry is set when the ref named the design's declared entry file.
	NamedIsTheEntry bool
	// FromDeclaration is set when the tiers came from the design's declaration rather than from the
	// ref alone. It does not mean a tier moved. Naming the entry of a design that declares no
	// companion applies the declaration and changes nothing, which a caller narrating what it read
	// distinguishes from a ref that was never resolved.
	FromDeclaration bool
}

// RevisionOf returns the declared revision ref belongs to, as its entry or one of its companions,
// shaped as a Design whose entry and companions are that revision's. It returns nil when ref is the
// current revision's or belongs to none, so the design is resolved as it always was.
func RevisionOf(d *webapi.Design, ref string) *webapi.Design {
	for _, rev := range d.GetRevisions() {
		if rev.GetEntryUri() != ref && !slices.Contains(rev.GetCompanionUris(), ref) {
			continue
		}
		return &webapi.Design{
			Name: d.GetName(), Title: d.GetTitle(), Uri: d.GetUri(), Config: d.GetConfig(),
			EntryUri: rev.GetEntryUri(), CompanionUris: rev.GetCompanionUris(),
		}
	}
	return nil
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

// ProjectService serves the project and design resources over an injected ProjectStore (C13). It
// does no I/O itself. It applies AIP-158 pagination and the one supported AIP-160 filter, and
// classifies errors for the transport.
//
// It is AIP-shaped with GET and LIST only, the read-only-resource case in CONSTRAINTS C23. A project
// gets a resource name rather than an artifact URI because an operator DECLARES its identity, so the
// name survives the folder being renamed or moved between mounts and reviews can be parented by it.
//
// Every caller goes through here, including the CLI. A second path reaching the store directly would
// drift on questions invisible from outside, such as whether an unresolved ref is an error or what a
// malformed descriptor does.
type ProjectService struct {
	store ProjectStore
}

// NewProjectService returns a ProjectService backed by store. A nil store is legal and means this
// deployment declares no projects, so every method answers as though nothing resolved rather than
// failing.
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
// that does not exist is ErrNotFound rather than an empty list, so a client can tell a wrong parent
// from a project with no designs.
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
		if mount == "" || uriMount(d.GetUri()) == mount {
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
	u, err := artifact.Parse(s)
	if err != nil {
		return ""
	}
	return u.Mount
}

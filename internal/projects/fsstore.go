package projects

import (
	"context"
	"fmt"
	"github.com/panyam/agni/artifact"
	"io/fs"
	"maps"
	"path"
	"slices"
	"sort"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/proto"
)

// MaxDepth bounds the downward walk that discovers descriptors under a tree, counting the tree root
// as depth 0. A tree is whatever folder an operator handed the server, possibly a build output or a
// home directory, so an unbounded walk would stat all of it on every listing. Four levels reaches
// `<root>/<project>/designs/<design>/`, one deeper than a review project's layout.
const MaxDepth = 4

// Tree is one named filesystem the store looks in: a mount on a server, a bounded slice of the local
// filesystem for the CLI, an embedded FS in a test.
type Tree struct {
	// Mount is the name callers address this tree by, and the value that lands in Project.mount.
	Mount string
	FS    fs.FS
}

// FSStore is the filesystem-backed service.ProjectStore. Nothing above the port knows it exists,
// because the tree walking below is only true of keeping projects in directories.
//
// It caches, but every read revalidates against the filesystem before returning, so an operator's
// edit to a descriptor is visible on the very next request. See cache.go.
type FSStore struct {
	trees []Tree
	// See cache.go for why discovery and content are keyed on different things.
	walks    *walkCache
	projectC *parseCache[*webapi.Project]
	designC  *parseCache[*webapi.Design]
}

// NewFSStore returns a store over the given trees, searched in order.
func NewFSStore(trees ...Tree) *FSStore {
	return &FSStore{
		trees:    trees,
		walks:    newWalkCache(),
		projectC: newParseCache[*webapi.Project](),
		designC:  newParseCache[*webapi.Design](),
	}
}

// locatedProject pairs a parsed descriptor with where it was found.
type locatedProject struct {
	id  string
	msg *webapi.Project
	dir string
}

// Project returns one project by resource name, or ErrNotFound.
func (s *FSStore) Project(ctx context.Context, name string) (*webapi.Project, error) {
	all, err := s.Projects(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.GetName() == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("%w: no project %q on any mount", service.ErrNotFound, name)
}

// Projects discovers every project across the trees, ordered by resource name.
//
// A duplicate id is an ERROR rather than a first-wins pick, since first-wins would answer a question
// about project A with project B's designs.
func (s *FSStore) Projects(context.Context) ([]*webapi.Project, error) {
	var out []*webapi.Project
	seen := map[string]string{}
	for _, t := range s.trees {
		found, err := s.projectsIn(t)
		if err != nil {
			return nil, err
		}
		for _, lp := range found {
			where := t.Mount + ":" + displayDir(lp.dir)
			if first, dup := seen[lp.id]; dup {
				return nil, fmt.Errorf("duplicate project id %q, declared at %s and at %s: a project id is its resource name, so two projects cannot share one", lp.id, first, where)
			}
			seen[lp.id] = where
			out = append(out, lp.msg)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, nil
}

// Design returns one design by resource name, or ErrNotFound.
func (s *FSStore) Design(ctx context.Context, name string) (*webapi.Design, error) {
	parent, _, ok := service.SplitDesignName(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a design resource name", service.ErrInvalidArgument, name)
	}
	all, err := s.Designs(ctx, parent)
	if err != nil {
		return nil, err
	}
	for _, d := range all {
		if d.GetName() == name {
			return d, nil
		}
	}
	return nil, fmt.Errorf("%w: no design %q", service.ErrNotFound, name)
}

// Designs returns one project's designs, ordered by resource name, with a duplicate id rejected on
// the same reasoning as a duplicate project id.
func (s *FSStore) Designs(ctx context.Context, parent string) ([]*webapi.Design, error) {
	p, err := s.Project(ctx, parent)
	if err != nil {
		return nil, err
	}
	t, ok := s.tree(uriOf(p.GetUri()).Mount)
	if !ok {
		return nil, fmt.Errorf("%w: mount %q is no longer configured", service.ErrNotFound, uriOf(p.GetUri()).Mount)
	}
	dirs, err := s.walks.find(t.FS, walkRoot(uriOf(p.GetUri()).Path), DesignDescriptor)
	if err != nil {
		return nil, err
	}
	var out []*webapi.Design
	seen := map[string]string{}
	for _, dir := range dirs {
		id, d, err := s.loadDesign(t, dir)
		if err != nil {
			return nil, err
		}
		if first, dup := seen[id]; dup {
			return nil, fmt.Errorf("duplicate design id %q in %s, declared in %q and in %q", id, parent, first, displayDir(dir))
		}
		seen[id] = displayDir(dir)
		d.Name = service.DesignName(parent, id)
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, nil
}

// ResolveDesign maps a URI to the design containing it and that design's project. It walks UP from
// the URI's path, so the answer costs a few stats however many designs a tree holds.
//
// A miss is (nil, nil, nil), the ordinary state of a mounted folder.
//
// A design with no enclosing project is NOT a miss. It comes back with its declaration intact and an
// EMPTY `name`, since a resource name needs a parent, and the CLI still honours the declaration.
func (s *FSStore) ResolveDesign(_ context.Context, uri artifact.URI) (*webapi.Design, *webapi.Project, error) {
	t, ok := s.tree(uri.Mount)
	if !ok {
		return nil, nil, fmt.Errorf("no such mount %q: %w", uri.Mount, service.ErrNotFound)
	}
	dir := uri.Path
	if dir == "" {
		dir = "."
	} else if !isDir(t.FS, dir) {
		dir = path.Dir(dir)
	}
	designDir, found := findAbove(t.FS, dir, DesignDescriptor)
	if !found {
		return nil, nil, nil
	}
	designID, d, err := s.loadDesign(t, designDir)
	if err != nil {
		return nil, nil, err
	}
	projectDir, found := findAbove(t.FS, designDir, ProjectDescriptor)
	if !found {
		return d, nil, nil
	}
	_, p, err := s.loadProject(t, projectDir)
	if err != nil {
		return nil, nil, err
	}
	d.Name = service.DesignName(p.GetName(), designID)
	return d, p, nil
}

// projectsIn discovers the project descriptors in one tree.
func (s *FSStore) projectsIn(t Tree) ([]locatedProject, error) {
	dirs, err := s.walks.find(t.FS, ".", ProjectDescriptor)
	if err != nil {
		return nil, err
	}
	out := make([]locatedProject, 0, len(dirs))
	for _, dir := range dirs {
		id, p, err := s.loadProject(t, dir)
		if err != nil {
			return nil, err
		}
		out = append(out, locatedProject{id: id, msg: p, dir: dir})
	}
	return out, nil
}

// loadProject parses one project descriptor and fills in what only the store knows: its resource
// name, which tree it came from, and where in that tree.
func (s *FSStore) loadProject(t Tree, dir string) (string, *webapi.Project, error) {
	name := path.Join(walkRoot(dir), ProjectDescriptor)
	// The directory is a dependency too, so the existence probes in attachConfig are covered, since
	// adding params/ moves the directory's mtime without changing any file read here.
	deps := []string{walkRoot(dir), name}
	return s.projectC.get(t.FS, t.Mount+"\x00"+name, deps, func() (string, *webapi.Project, error) {
		return s.readProject(t, dir, name)
	}, cloneProject)
}

func (s *FSStore) readProject(t Tree, dir, name string) (string, *webapi.Project, error) {
	f, err := t.FS.Open(name)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	id, p, names, err := ParseProject(f)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	u, err := artifact.New(t.Mount, normalizeDir(dir))
	if err != nil {
		return "", nil, err
	}
	p.Name = service.ProjectName(id)
	p.Uri = u.String()
	// A conventions.yaml or review.yaml beside the descriptor is the layout before agni issue 828.
	// Reading the project without it would drop the team's vocabulary or its checklist and report as
	// though the team declared none, so it is an error that says where the content goes now.
	for _, f := range slices.Sorted(maps.Keys(formerProjectFiles)) {
		if exists(t.FS, path.Join(walkRoot(dir), f)) {
			return "", nil, fmt.Errorf("%s: %s is no longer read; move its content under %s in %s (agni issue 828)", name, f, formerProjectFiles[f], ProjectDescriptor)
		}
	}
	if err := s.attachConfig(t, dir, u, names, p); err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	return id, p, nil
}

// attachConfig fills in the directory tiers a project owns, as URIs for what exists. Its conventions
// and checklists are values ParseProject already read from the descriptor.
//
// A declared name that names nothing is SILENTLY ABSENT rather than an error. The names default
// (profiles/, params/, symbols/, lib/), so otherwise a project that declared nothing would fail for
// lacking directories. An explicitly declared missing name deserves an error, but the descriptor
// cannot tell the two apart yet.
func (s *FSStore) attachConfig(t Tree, dir string, base artifact.URI, names ProjectConfigNames, p *webapi.Project) error {
	rel := func(n string) (string, bool) {
		if n == "" {
			return "", false
		}
		full := path.Join(walkRoot(dir), n)
		if !exists(t.FS, full) {
			return "", false
		}
		u, err := base.Join(n)
		if err != nil {
			return "", false
		}
		return u.String(), true
	}
	if uri, ok := rel(names.Profiles); ok {
		p.Config.ProfileUris = []string{uri}
	}
	if uri, ok := rel(names.Params); ok {
		p.Config.ParamUris = []string{uri}
	}
	if uri, ok := rel(names.Symbols); ok {
		p.Config.SymbolPathUris = []string{uri}
	}
	if uri, ok := rel(names.Lib); ok {
		p.Config.LibraryUris = []string{uri}
	}
	return nil
}

// loadDesign parses one design descriptor and rewrites its design-folder-relative refs into the
// mount:// URIs the wire type promises. The join happens HERE, once, because no consumer above the
// port knows where the design folder sits. The caller sets Name, which needs the parent.
func (s *FSStore) loadDesign(t Tree, dir string) (string, *webapi.Design, error) {
	name := path.Join(walkRoot(dir), DesignDescriptor)
	deps := []string{walkRoot(dir), name}
	return s.designC.get(t.FS, t.Mount+"\x00"+name, deps, func() (string, *webapi.Design, error) {
		return s.readDesign(t, dir, name)
	}, cloneDesign)
}

func (s *FSStore) readDesign(t Tree, dir, name string) (string, *webapi.Design, error) {
	f, err := t.FS.Open(name)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	id, d, err := ParseDesign(f)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	base, err := artifact.New(t.Mount, normalizeDir(dir))
	if err != nil {
		return "", nil, err
	}
	entry, err := base.Join(d.GetEntryUri())
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	d.Uri = base.String()
	d.EntryUri = entry.String()
	// An intent.yaml beside the descriptor is the layout before agni issue 824. Reading the design
	// without it would quietly drop every intent rule, which reads as a board that declares nothing,
	// so it is an error that says where the declarations go now.
	if exists(t.FS, path.Join(walkRoot(dir), formerIntentFile)) {
		return "", nil, fmt.Errorf("%s: %s is no longer read; move its declarations under intent: in %s (agni issue 824)", name, formerIntentFile, DesignDescriptor)
	}
	// Symbols become URIs only for a directory that exists, so a design that never made one reads as
	// having none rather than naming a missing directory.
	if names := d.GetConfig().GetSymbolPathUris(); len(names) > 0 {
		var resolved []string
		for _, n := range names {
			if !exists(t.FS, path.Join(walkRoot(dir), n)) {
				continue
			}
			su, err := base.Join(n)
			if err != nil {
				return "", nil, err
			}
			resolved = append(resolved, su.String())
		}
		d.Config.SymbolPathUris = resolved
	}
	for i, c := range d.GetCompanionUris() {
		cu, err := base.Join(c)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", name, err)
		}
		d.CompanionUris[i] = cu.String()
	}
	return id, d, nil
}

func (s *FSStore) tree(mount string) (Tree, bool) {
	for _, t := range s.trees {
		if t.Mount == mount {
			return t, true
		}
	}
	return Tree{}, false
}

// findAbove walks up from dir looking for a descriptor, stopping at the tree root. An fs.FS has no
// parent to climb into, so resolution cannot reach a descriptor outside the tree.
func findAbove(fsys fs.FS, dir, name string) (string, bool) {
	for {
		if exists(fsys, path.Join(walkRoot(dir), name)) {
			return dir, true
		}
		if dir == "." || dir == "" {
			return "", false
		}
		parent := path.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func exists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func isDir(fsys fs.FS, name string) bool {
	fi, err := fs.Stat(fsys, walkRoot(name))
	return err == nil && fi.IsDir()
}

// walkRoot maps the empty tree-relative folder to the "." that fs.FS requires, since callers above
// this package spell the root as "".
func walkRoot(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}

// normalizeDir maps fs.FS's "." for the root back to the empty string callers use.
func normalizeDir(dir string) string {
	if dir == "." {
		return ""
	}
	return dir
}

// displayDir is normalizeDir for an error message, where "" would read as a missing value.
func displayDir(dir string) string {
	if d := normalizeDir(dir); d != "" {
		return d
	}
	return "the tree root"
}

// uriOf parses a resource's stored artifact URI. On a parse failure it returns the zero URI, which
// matches nothing rather than failing a listing.
func uriOf(s string) artifact.URI {
	u, err := artifact.Parse(s)
	if err != nil {
		return artifact.URI{}
	}
	return u
}

// cloneProject and cloneDesign keep a cached message from being handed out by pointer.
//
// The store MUTATES what it loads (resource names, refs rewritten into URIs), so handing out the
// cached value would make the second call join a URI onto a URI.
func cloneProject(p *webapi.Project) *webapi.Project { return proto.CloneOf(p) }

func cloneDesign(d *webapi.Design) *webapi.Design { return proto.CloneOf(d) }

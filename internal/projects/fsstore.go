package projects

import (
	"context"
	"fmt"
	"github.com/panyam/agni/artifact"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/proto"
)

// MaxDepth bounds the downward walk that discovers descriptors under a tree. The tree root is depth
// 0. An operator can hand the server any folder as a tree, such as a build output or a home
// directory, so an unbounded walk would stat all of it on every listing. Four levels reaches
// `<root>/<team>/<project>/<design>/` with one to spare.
const MaxDepth = 4

// Tree is one named filesystem for the store to look in. It is a mount on a server, a bounded slice
// of the local filesystem for the CLI, or an embedded FS in a test.
type Tree struct {
	// Mount is the name callers use to address this tree. It is also the value of Project.mount.
	Mount string
	FS    fs.FS
}

// FSStore is the filesystem-backed service.ProjectStore. Nothing above the port knows it exists.
// The tree walking below is only true of keeping projects in directories.
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
// A duplicate id is an ERROR. A first-wins pick could answer a question about project A with
// project B's designs.
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
			if err := s.nameParent(lp.msg, map[string]bool{descriptorURI(lp.msg): true}); err != nil {
				return nil, err
			}
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

// Designs returns one project's designs, ordered by resource name. A design belongs to the project
// when its descriptor names that project in `extends`. A duplicate design id is rejected, for the
// same reason as a duplicate project id. A design may sit anywhere a mount reaches, since it names
// its parent and is not found under it.
func (s *FSStore) Designs(ctx context.Context, parent string) ([]*webapi.Design, error) {
	p, err := s.Project(ctx, parent)
	if err != nil {
		return nil, err
	}
	var out []*webapi.Design
	seen := map[string]string{}
	for _, t := range s.trees {
		dirs, err := s.walks.find(t, ".", DesignDescriptor)
		if err != nil {
			return nil, err
		}
		for _, dir := range dirs {
			id, d, err := s.loadDesign(t, dir)
			if err != nil {
				return nil, err
			}
			if d.GetConfig().GetExtends() != descriptorURI(p) {
				continue
			}
			where := t.Mount + ":" + displayDir(dir)
			if first, dup := seen[id]; dup {
				return nil, fmt.Errorf("duplicate design id %q in %s, declared at %s and at %s", id, parent, first, where)
			}
			seen[id] = where
			d.Name = service.DesignName(parent, id)
			d.Config.Extends = parent
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, nil
}

// ResolveDesign maps a URI to the design containing it and the project that design extends. It walks
// UP from the URI's path to find the design, so the answer costs a few stats however many designs a
// tree holds. The design names its project, and ResolveDesign never takes one found above it.
//
// A miss is (nil, nil, nil), the ordinary state of a mounted folder.
//
// A design that extends nothing is NOT a miss. It comes back with its declaration intact and an
// EMPTY `name`, since a resource name needs a parent, and the CLI still honours the declaration.
// Suppose instead that a project descriptor sits above a design that extends nothing. The author
// most likely forgot the `extends`, and reading the design would run it on the built-in config with
// nothing said. So that case is an error.
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
	parent := d.GetConfig().GetExtends()
	if parent == "" {
		if projectDir, found := findAbove(t.FS, designDir, ProjectDescriptor); found {
			return nil, nil, fmt.Errorf("%w: %s:%s extends nothing, and %s:%s sits above it; add `extends: <path to that %s>` to the design, so it reads with that project's config",
				service.ErrInvalidArgument, t.Mount, path.Join(walkRoot(designDir), DesignDescriptor), t.Mount, path.Join(walkRoot(projectDir), ProjectDescriptor), ProjectDescriptor)
		}
		return d, nil, nil
	}
	p, err := s.projectAt(parent)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: extends: %w", path.Join(walkRoot(designDir), DesignDescriptor), err)
	}
	d.Name = service.DesignName(p.GetName(), designID)
	d.Config.Extends = p.GetName()
	return d, p, nil
}

// projectsIn discovers the project descriptors in one tree.
func (s *FSStore) projectsIn(t Tree) ([]locatedProject, error) {
	dirs, err := s.walks.find(t, ".", ProjectDescriptor)
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

// loadProject parses one project descriptor. It fills in three facts only the store knows, the
// resource name, the tree it came from and where in that tree. Its `extends` comes back as the
// PARENT'S descriptor URI, and nameParent turns that into a resource name. The cached value must
// not depend on a file outside its own dependencies.
func (s *FSStore) loadProject(t Tree, dir string) (string, *webapi.Project, error) {
	name := path.Join(walkRoot(dir), ProjectDescriptor)
	var deps []string
	return s.projectC.get(t.FS, t.Mount+"\x00"+name, func() ([]string, string, *webapi.Project, error) {
		id, p, err := s.readProject(t, dir, name, &deps)
		return deps, id, p, err
	}, cloneProject)
}

func (s *FSStore) readProject(t Tree, dir, name string, deps *[]string) (string, *webapi.Project, error) {
	*deps = []string{name}
	f, err := t.FS.Open(name)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	id, p, src, err := ParseProject(f)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	u, err := artifact.New(t.Mount, normalizeDir(dir))
	if err != nil {
		return "", nil, err
	}
	p.Name = service.ProjectName(id)
	p.Uri = u.String()
	if err := s.attachSources(t, u, src, p.Config, deps); err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	return id, p, nil
}

// attachSources resolves a descriptor's outside references into mount:// URIs, as the wire type
// promises. The parent descriptor goes into config.extends, and each tier's files go into that
// tier's field. The paths are relative to the descriptor's folder, base.
//
// A source naming nothing is an ERROR. Nothing is found by its folder's name any more. So each path
// in the descriptor is a tier the operator asked for. Reading on without it would report as though
// the team declared none. Each same-tree source joins deps, so removing one revalidates the descriptor.
func (s *FSStore) attachSources(t Tree, base artifact.URI, src Sources, cfg *webapi.AnalysisConfig, deps *[]string) error {
	// fix says what to do about a source that cannot be read. The advice differs for the parent. A
	// design copied or mounted without its project reads on its own once its extends line is
	// removed.
	fix := func(field string) string {
		if field == "extends" {
			return "mount or drop the folder holding that project.yaml as well, or remove extends to read this descriptor on its own"
		}
		return "fix the path, or delete it to run without that " + field
	}
	resolve := func(field, ref string) (string, error) {
		if !strings.HasPrefix(strings.TrimSpace(ref), artifact.Scheme+"://") {
			if joined := path.Join(base.Path, CleanRel(ref)); joined == ".." || strings.HasPrefix(joined, "../") {
				return "", fmt.Errorf("%s %q is outside mount %q, so it cannot be read; %s", field, ref, base.Mount, fix(field))
			}
		}
		u, err := resolveSource(base, ref)
		if err != nil {
			return "", fmt.Errorf("%s %q: %w", field, ref, err)
		}
		if u.Mount == t.Mount {
			*deps = append(*deps, walkRoot(u.Path))
		}
		owner, ok := s.tree(u.Mount)
		if !ok {
			return "", fmt.Errorf("%s %q: no mount %q is configured", field, ref, u.Mount)
		}
		if !exists(owner.FS, walkRoot(u.Path)) {
			return "", fmt.Errorf("%s %q names nothing (looked for %s); %s", field, ref, u, fix(field))
		}
		return u.String(), nil
	}
	if src.Extends != "" {
		uri, err := resolve("extends", src.Extends)
		if err != nil {
			return err
		}
		cfg.Extends = uri
	}
	for _, tier := range []struct {
		field string
		refs  []string
		into  *[]string
	}{
		{"profiles", src.Profiles, &cfg.ProfileUris},
		{"params", src.Params, &cfg.ParamUris},
		{"symbols", src.Symbols, &cfg.SymbolPathUris},
		{"lib", src.Lib, &cfg.LibraryUris},
		{"queries", src.Queries, &cfg.QuerySetUris},
	} {
		for _, ref := range tier.refs {
			uri, err := resolve(tier.field, ref)
			if err != nil {
				return err
			}
			*tier.into = append(*tier.into, uri)
		}
	}
	return nil
}

// resolveSource turns a descriptor's source into a URI. A mount:// URI is kept as written. A path
// is joined onto the descriptor's folder, and artifact refuses the join when it climbs out of the
// mount.
func resolveSource(base artifact.URI, ref string) (artifact.URI, error) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, artifact.Scheme+"://") {
		return artifact.Parse(ref)
	}
	return base.Join(CleanRel(ref))
}

// projectAt loads a project from the descriptor that a design or project names in `extends`. The
// loaded project names its own parent as a resource name.
func (s *FSStore) projectAt(descriptor string) (*webapi.Project, error) {
	u, err := artifact.Parse(descriptor)
	if err != nil {
		return nil, err
	}
	t, ok := s.tree(u.Mount)
	if !ok {
		return nil, fmt.Errorf("%w: no mount %q is configured", service.ErrNotFound, u.Mount)
	}
	_, p, err := s.loadProject(t, path.Dir(walkRoot(u.Path)))
	if err != nil {
		return nil, err
	}
	if err := s.nameParent(p, map[string]bool{descriptor: true}); err != nil {
		return nil, err
	}
	return p, nil
}

// nameParent turns a loaded project's `extends` into the parent's resource name. The `extends`
// holds the parent's descriptor URI, and service.ResolveExtends walks the chain by resource name.
// visiting guards against a cycle through descriptors with different ids.
func (s *FSStore) nameParent(p *webapi.Project, visiting map[string]bool) error {
	parent := p.GetConfig().GetExtends()
	if parent == "" || !strings.HasPrefix(parent, artifact.Scheme+"://") {
		return nil
	}
	if visiting[parent] {
		return fmt.Errorf("%w: extends cycle through %s", service.ErrInvalidArgument, parent)
	}
	u, err := artifact.Parse(parent)
	if err != nil {
		return err
	}
	t, ok := s.tree(u.Mount)
	if !ok {
		return fmt.Errorf("%w: %s extends %s, on no configured mount", service.ErrNotFound, p.GetName(), parent)
	}
	id, _, err := s.loadProject(t, path.Dir(walkRoot(u.Path)))
	if err != nil {
		return fmt.Errorf("%s extends %s: %w", p.GetName(), parent, err)
	}
	p.Config.Extends = service.ProjectName(id)
	return nil
}

// descriptorURI returns the URI of the descriptor a loaded project was read from. A design naming
// that project in `extends` resolves to this URI.
func descriptorURI(p *webapi.Project) string {
	u, err := artifact.Parse(p.GetUri())
	if err != nil {
		return ""
	}
	d, err := u.Join(ProjectDescriptor)
	if err != nil {
		return ""
	}
	return d.String()
}

// loadDesign parses one design descriptor and rewrites its design-folder-relative refs into
// mount:// URIs, as the wire type promises. The join happens HERE, once, because no consumer above
// the port knows where the design folder sits. The caller sets Name, which needs the parent.
// config.extends holds the project's descriptor URI, and the caller turns it into the project's
// resource name.
func (s *FSStore) loadDesign(t Tree, dir string) (string, *webapi.Design, error) {
	name := path.Join(walkRoot(dir), DesignDescriptor)
	var deps []string
	return s.designC.get(t.FS, t.Mount+"\x00"+name, func() ([]string, string, *webapi.Design, error) {
		id, d, err := s.readDesign(t, dir, name, &deps)
		return deps, id, d, err
	}, cloneDesign)
}

func (s *FSStore) readDesign(t Tree, dir, name string, deps *[]string) (string, *webapi.Design, error) {
	*deps = []string{name}
	f, err := t.FS.Open(name)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	id, d, src, err := ParseDesign(f)
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
	if err := s.attachSources(t, base, src, d.Config, deps); err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	join := func(rel string) (string, error) {
		u, err := base.Join(rel)
		if err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		return u.String(), nil
	}
	for i, c := range d.GetCompanionUris() {
		if d.CompanionUris[i], err = join(c); err != nil {
			return "", nil, err
		}
	}
	for _, rev := range d.GetRevisions() {
		if rev.EntryUri, err = join(rev.GetEntryUri()); err != nil {
			return "", nil, err
		}
		for i, c := range rev.GetCompanionUris() {
			if rev.CompanionUris[i], err = join(c); err != nil {
				return "", nil, err
			}
		}
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
// The store MUTATES what it loads. It sets resource names and rewrites refs into URIs. Handing out
// the cached value would make the second call join a URI onto a URI.
func cloneProject(p *webapi.Project) *webapi.Project { return proto.CloneOf(p) }

func cloneDesign(d *webapi.Design) *webapi.Design { return proto.CloneOf(d) }

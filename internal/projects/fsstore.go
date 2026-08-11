package projects

import (
	"context"
	"fmt"
	"github.com/panyam/agni/internal/artifact"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/service"
)

// MaxDepth bounds the downward walk that discovers descriptors under a tree, counting the tree root
// as depth 0.
//
// A bound rather than an unlimited walk because a tree is a folder an operator handed the server,
// not a curated one: it can contain a build output directory, a vendored library, or a home
// directory, and an unbounded walk would stat every file in it on every listing. Four levels reaches
// `<root>/<project>/designs/<design>/`, which is one deeper than the layout a review project takes,
// so the shipped shape has room to nest one level further without this needing to be raised.
const MaxDepth = 4

// Tree is one named filesystem the store looks in: a mount on a server, a bounded slice of the local
// filesystem for the CLI, an embedded FS in a test.
type Tree struct {
	// Mount is the name callers address this tree by, and the value that lands in Project.mount.
	Mount string
	FS    fs.FS
}

// FSStore is the filesystem-backed service.ProjectStore. It satisfies that port and nothing above
// the port knows it exists, which is the whole point: the tree walking below is a fact about keeping
// projects in a directory hierarchy, and a store backed by a database with design files on object
// storage answers the same five questions without any of it.
//
// It holds NO CACHE, deliberately. A descriptor is a small file an operator edits while a server is
// running, and a cached index would answer with a design's old entry after they fixed it, which is
// precisely the class of silent-wrong-answer this feature exists to remove. The cost is a bounded
// directory walk per call, which is nothing beside parsing a design.
type FSStore struct {
	trees []Tree
}

// NewFSStore returns a store over the given trees, searched in order.
func NewFSStore(trees ...Tree) *FSStore { return &FSStore{trees: trees} }

// located pairs a parsed descriptor with where it was found.
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
// A duplicate id is an ERROR rather than a first-wins pick: two projects claiming one name means one
// of them is unreachable through its own resource name, and silently serving the other would answer
// a client's question about project A with project B's designs.
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
	dirs, err := findDescriptors(t.FS, walkRoot(uriOf(p.GetUri()).Path), DesignDescriptor)
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

// ResolveDesign maps a (mount, ref) to the design containing it and that design's project.
//
// It walks UP from the ref rather than scanning every design, so the answer costs a few stats no
// matter how many designs a tree holds. That direction is an FS-store detail; the port only promises
// the mapping.
//
// A miss is (nil, nil, nil): a ref belonging to no declared design is the ordinary state of a
// mounted folder.
//
// A design with no enclosing project is NOT a miss. It comes back with its declaration intact and an
// EMPTY `name`, because a resource name needs a parent and there is none — but the declaration is
// still the truth about which file that folder's analysis reads, and a caller pointed at a path by a
// human (the CLI) needs no resource name to honour it. Only addressability is missing, not meaning.
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
	dirs, err := findDescriptors(t.FS, ".", ProjectDescriptor)
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
	if err := s.attachConfig(t, dir, u, names, p); err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	return id, p, nil
}

// attachConfig fills in the config a project owns, as URIs for what exists.
//
// A declared name that names nothing is SILENTLY ABSENT rather than an error, and that asymmetry is
// deliberate. The names default (conventions.yaml, profiles/, params/, review.yaml), so a project
// that never declared anything would otherwise fail for lacking files it never claimed to have.
// A project that DECLARES a name explicitly is a different case, and one worth failing on — but the
// descriptor cannot currently tell the two apart, so this errs toward serving the project.
//
// The conventions file is the one tier read HERE rather than handed on as a URI: it is small, it is
// a value under C22, and composing it must need no I/O.
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
	if uri, ok := rel(names.Conventions); ok {
		b, err := fs.ReadFile(t.FS, path.Join(walkRoot(dir), names.Conventions))
		if err != nil {
			return err
		}
		cfg, err := naming.Parse(b)
		if err != nil {
			return fmt.Errorf("%s: %w", uri, err)
		}
		p.Conventions = service.ConventionProto(cfg)
	}
	if uri, ok := rel(names.Profiles); ok {
		p.ProfileUris = []string{uri}
	}
	if uri, ok := rel(names.Params); ok {
		p.ParamUris = []string{uri}
	}
	if uri, ok := rel(names.Checklist); ok {
		p.ChecklistUri = uri
	}
	return nil
}

// loadDesign parses one design descriptor and rewrites its declared, design-folder-relative refs
// into the mount-relative ones the wire type promises. The join happens HERE, once, because every
// consumer above the port addresses files by (mount, ref) and none of them knows where the design
// folder sits. The caller sets Name, which needs the parent.
func (s *FSStore) loadDesign(t Tree, dir string) (string, *webapi.Design, error) {
	name := path.Join(walkRoot(dir), DesignDescriptor)
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
	// Intent is a NAME until here; it becomes a URI only if the file is actually there, so a design
	// that never wrote one reads as having none rather than as naming a file that is missing.
	if d.GetIntentUri() != "" {
		if exists(t.FS, path.Join(walkRoot(dir), d.GetIntentUri())) {
			iu, err := base.Join(d.GetIntentUri())
			if err != nil {
				return "", nil, err
			}
			d.IntentUri = iu.String()
		} else {
			d.IntentUri = ""
		}
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

// findDescriptors returns the tree-relative folders under root (root included) holding a descriptor
// of the given name, to MaxDepth.
//
// It does NOT descend into a folder that already holds the descriptor it is looking for: a project
// inside a project would be an ambiguity nobody meant, and stopping there also keeps the walk off a
// design's own symbol and sheet subfolders once the design has been found. Dot-directories are
// skipped so a `.git` or `.venv` never costs a traversal.
func findDescriptors(fsys fs.FS, root, name string) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			// A directory that cannot be read is not a reason to fail every listing; it simply holds
			// no descriptors as far as this caller is concerned.
			if e != nil && e.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !e.IsDir() {
			return nil
		}
		if base := path.Base(p); p != root && strings.HasPrefix(base, ".") {
			return fs.SkipDir
		}
		if depthUnder(root, p) > MaxDepth {
			return fs.SkipDir
		}
		if exists(fsys, path.Join(p, name)) {
			out = append(out, p)
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// findAbove walks up from dir looking for a descriptor, stopping at the tree root. The root is the
// hard stop, which is what makes containment structural: an fs.FS has no parent to climb into, so
// resolution cannot reach a descriptor outside the tree.
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

// depthUnder counts the path segments between root and p.
func depthUnder(root, p string) int {
	if root == p {
		return 0
	}
	rel := strings.TrimPrefix(p, walkRoot(root))
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return 0
	}
	return len(strings.Split(rel, "/"))
}

// uriOf parses a resource's stored artifact URI. A URI this package wrote is always well formed, so
// a parse failure means the value came from somewhere else; the zero URI then simply matches nothing
// rather than failing a listing.
func uriOf(s string) artifact.URI {
	u, err := artifact.Parse(s)
	if err != nil {
		return artifact.URI{}
	}
	return u
}

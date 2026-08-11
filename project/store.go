package project

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// The two descriptor file names. They are the only path convention in the whole feature, and they
// are confined to this file, because an index-backed or PLM-backed implementation of the same
// discovery would never open a file named either of these.
const (
	ProjectDescriptor = "project.yaml"
	DesignDescriptor  = "design.yaml"
)

// MaxDepth bounds the downward walk that discovers descriptors under a tree, counting the root as
// depth 0.
//
// A bound rather than an unlimited walk because a tree here is a folder an operator handed to the
// server, not a curated one: it can contain a build output directory, a vendored library, or a home
// directory, and an unbounded walk would stat every file in it on every listing. Four levels reaches
// `<root>/<project>/designs/<design>/`, which is one deeper than the layout a review project takes,
// so the shipped shape has room to nest one level further without this needing to be raised.
const MaxDepth = 4

// LocatedProject is a parsed project descriptor and the tree-relative folder it was found in ("" for
// the root).
type LocatedProject struct {
	Project
	Dir string
}

// LocatedDesign is a parsed design descriptor, the folder it was found in, and the folder of the
// project that contains it.
type LocatedDesign struct {
	Design
	Dir        string
	ProjectDir string
}

// Tree discovers descriptors in one filesystem: a mount on a server, a directory the CLI was pointed
// at, an embedded FS in a test. It is the tree-walking implementation of discovery, and the only one
// this package ships.
//
// It holds NO CACHE, deliberately. A descriptor is a small file an operator edits while a server is
// running, and a cached index would answer with a design's old entry after they fixed it, which is
// precisely the class of silent-wrong-answer this whole feature exists to remove. The cost is a
// bounded directory walk per call, which is nothing beside parsing a design.
type Tree struct {
	FS fs.FS
}

// Projects returns every project descriptor in the tree, ordered by declared id.
//
// A duplicate id is an ERROR rather than a first-wins pick: two projects claiming one name means one
// of them is unreachable through its own resource name, and silently serving the other would answer
// a client's question about project A with project B's designs.
func (t Tree) Projects() ([]LocatedProject, error) {
	dirs, err := t.findDescriptors(".", ProjectDescriptor)
	if err != nil {
		return nil, err
	}
	var out []LocatedProject
	seen := map[string]string{}
	for _, dir := range dirs {
		p, err := t.loadProject(dir)
		if err != nil {
			return nil, err
		}
		if first, dup := seen[p.Name]; dup {
			return nil, fmt.Errorf("duplicate project id %q, declared in %q and in %q: a project id is its resource name, so two projects cannot share one", p.Name, first, displayDir(dir))
		}
		seen[p.Name] = displayDir(dir)
		out = append(out, LocatedProject{Project: p, Dir: normalizeDir(dir)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Designs returns the design descriptors under one project's folder, ordered by declared id, with a
// duplicate id rejected on the same reasoning as a duplicate project id.
func (t Tree) Designs(projectDir string) ([]LocatedDesign, error) {
	dirs, err := t.findDescriptors(walkRoot(projectDir), DesignDescriptor)
	if err != nil {
		return nil, err
	}
	var out []LocatedDesign
	seen := map[string]string{}
	for _, dir := range dirs {
		d, err := t.loadDesign(dir)
		if err != nil {
			return nil, err
		}
		if first, dup := seen[d.Name]; dup {
			return nil, fmt.Errorf("duplicate design id %q, declared in %q and in %q", d.Name, first, displayDir(dir))
		}
		seen[d.Name] = displayDir(dir)
		out = append(out, LocatedDesign{Design: d, Dir: normalizeDir(dir), ProjectDir: normalizeDir(projectDir)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Resolve maps a tree-relative ref — a design file, or the design folder itself — to the design
// containing it and that design's project.
//
// It walks UP from the ref rather than scanning every design, so the answer costs a few stats no
// matter how many designs the tree holds.
//
// Not found is ok=false with a NIL ERROR. A ref belonging to no declared design is the ordinary state
// of a mounted folder, and a caller reading an error for it would end up treating the common path as
// exceptional. A design with no enclosing project is also ok=false: a design is addressable only
// under a parent, so one without a project has no resource name to return.
func (t Tree) Resolve(ref string) (LocatedDesign, LocatedProject, bool, error) {
	dir := path.Clean(strings.TrimSuffix(ref, "/"))
	if dir == "" || dir == "." {
		dir = "."
	} else if !t.isDir(dir) {
		dir = path.Dir(dir)
	}
	designDir, found := t.findAbove(dir, DesignDescriptor)
	if !found {
		return LocatedDesign{}, LocatedProject{}, false, nil
	}
	projectDir, found := t.findAbove(designDir, ProjectDescriptor)
	if !found {
		return LocatedDesign{}, LocatedProject{}, false, nil
	}
	design, err := t.loadDesign(designDir)
	if err != nil {
		return LocatedDesign{}, LocatedProject{}, false, err
	}
	p, err := t.loadProject(projectDir)
	if err != nil {
		return LocatedDesign{}, LocatedProject{}, false, err
	}
	return LocatedDesign{Design: design, Dir: normalizeDir(designDir), ProjectDir: normalizeDir(projectDir)},
		LocatedProject{Project: p, Dir: normalizeDir(projectDir)}, true, nil
}

// EntryRef returns the design's entry as a TREE-relative ref, and CompanionRefs does the same for its
// companion views. The join happens here, once, because every consumer above this package addresses
// files relative to the tree and none of them knows where the design folder sits.
func (l LocatedDesign) EntryRef() string { return joinRef(l.Dir, l.Entry) }

// CompanionRefs returns the design's companion views as tree-relative refs, in declared order.
func (l LocatedDesign) CompanionRefs() []string {
	if len(l.Companions) == 0 {
		return nil
	}
	out := make([]string, 0, len(l.Companions))
	for _, c := range l.Companions {
		out = append(out, joinRef(l.Dir, c))
	}
	return out
}

// findDescriptors returns the tree-relative folders under root (root included) holding a descriptor
// of the given name, to MaxDepth.
//
// It does NOT descend into a folder that already holds the descriptor it is looking for: a project
// inside a project would be an ambiguity nobody meant, and stopping there also keeps the walk off a
// design's own symbol and sheet subfolders once the design has been found. Dot-directories are
// skipped so a `.git` or `.venv` never costs a traversal.
func (t Tree) findDescriptors(root, name string) ([]string, error) {
	var out []string
	err := fs.WalkDir(t.FS, root, func(p string, e fs.DirEntry, err error) error {
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
		if t.exists(path.Join(p, name)) {
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

// findAbove walks up from dir looking for a descriptor, stopping at the tree root. It returns the
// folder holding it. The tree root is the hard stop, which is what makes containment structural: an
// fs.FS has no parent to climb into, so resolution cannot reach a descriptor outside the mount.
func (t Tree) findAbove(dir, name string) (string, bool) {
	for {
		if t.exists(path.Join(walkRoot(dir), name)) {
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

func (t Tree) loadProject(dir string) (Project, error) {
	name := path.Join(walkRoot(dir), ProjectDescriptor)
	f, err := t.FS.Open(name)
	if err != nil {
		return Project{}, err
	}
	defer f.Close()
	p, err := LoadProject(f)
	if err != nil {
		return Project{}, fmt.Errorf("%s: %w", name, err)
	}
	return p, nil
}

func (t Tree) loadDesign(dir string) (Design, error) {
	name := path.Join(walkRoot(dir), DesignDescriptor)
	f, err := t.FS.Open(name)
	if err != nil {
		return Design{}, err
	}
	defer f.Close()
	d, err := LoadDesign(f)
	if err != nil {
		return Design{}, fmt.Errorf("%s: %w", name, err)
	}
	return d, nil
}

func (t Tree) exists(name string) bool {
	f, err := t.FS.Open(name)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func (t Tree) isDir(name string) bool {
	fi, err := fs.Stat(t.FS, walkRoot(name))
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

// joinRef joins a tree-relative folder and a descriptor-relative file into a tree-relative ref.
func joinRef(dir, rel string) string {
	clean := CleanRel(rel)
	if dir == "" {
		return clean
	}
	return dir + "/" + clean
}

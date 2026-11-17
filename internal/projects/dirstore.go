package projects

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// designsDir is the folder inside a project that holds its designs, one folder each.
const designsDir = "designs"

// designExtends is what a stored design's `extends` says. A design sits at
// `<project>/designs/<design>/`, so its project's descriptor is two folders up.
const designExtends = "../../" + ProjectDescriptor

// DirStore is the filesystem adapter of service.ProjectWriter (agni issue 1693). It keeps each
// project as a folder under one root, `<project>/project.yaml` beside the project's own files, and
// each design as `<project>/designs/<design>/` holding `design.yaml` and its files. That is the
// layout a hand-made project already has, so an FSStore over the root reads what DirStore writes,
// and the CLI can read a stored project as a plain folder.
//
// It writes only under its root. Writes reach it through the server's project store, so a project
// or design that store reads from another tree, a mounted folder, is refused as read-only.
//
// A write builds the new files in a dot-folder under the root and renames it into place, so a
// reader sees the old folder or the new one. FSStore skips dot-folders, so a half-built one is never
// listed. One mutex serialises every write, since a write is rare beside reads.
type DirStore struct {
	root string
	read service.ProjectStore
	mu   sync.Mutex
}

// NewDirStore returns a writer over root, created if absent. read is the store the server answers
// reads from. It must include a tree over root, and DirStore answers each write by reading the result
// back through it.
func NewDirStore(root string, read service.ProjectStore) (*DirStore, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &DirStore{root: abs, read: read}, nil
}

var _ service.ProjectWriter = (*DirStore)(nil)

// CreateProject writes a new project folder.
func (s *DirStore) CreateProject(ctx context.Context, id string, files []*webapi.SnapshotFile, blobs service.BlobSource) (*webapi.Project, error) {
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: project id %q must be lowercase letters, digits, '.', '_' or '-'", service.ErrInvalidArgument, id)
	}
	name := service.ProjectName(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exists(id) {
		return nil, fmt.Errorf("%w: %s", service.ErrAlreadyExists, name)
	}
	if _, err := s.read.Project(ctx, name); err == nil {
		return nil, fmt.Errorf("%w: %s is declared in a mounted folder", service.ErrAlreadyExists, name)
	}
	contents, err := s.contents(ctx, files, blobs, ProjectDescriptor, id, "", true)
	if err != nil {
		return nil, err
	}
	if err := s.place(contents, id, ""); err != nil {
		return nil, err
	}
	return s.read.Project(ctx, name)
}

// ReplaceProject swaps a project's files and keeps its designs.
func (s *DirStore) ReplaceProject(ctx context.Context, name string, files []*webapi.SnapshotFile, blobs service.BlobSource) (*webapi.Project, error) {
	id, err := s.ownedProject(ctx, name)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	contents, err := s.contents(ctx, files, blobs, ProjectDescriptor, id, "", true)
	if err != nil {
		return nil, err
	}
	if err := s.place(contents, id, designsDir); err != nil {
		return nil, err
	}
	return s.read.Project(ctx, name)
}

// DeleteProject removes a project folder. A project holding designs needs cascade.
func (s *DirStore) DeleteProject(ctx context.Context, name string, cascade bool) error {
	id, err := s.ownedProject(ctx, name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !cascade {
		held, err := os.ReadDir(filepath.Join(s.root, id, designsDir))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if len(held) > 0 {
			return fmt.Errorf("%w: %s holds %d design(s); delete them first or ask for a cascade", service.ErrFailedPrecondition, name, len(held))
		}
	}
	return s.remove(id)
}

// CreateDesign writes a new design folder under a stored project.
func (s *DirStore) CreateDesign(ctx context.Context, parent, id string, files []*webapi.SnapshotFile, blobs service.BlobSource) (*webapi.Design, error) {
	pid, err := s.ownedProject(ctx, parent)
	if err != nil {
		return nil, err
	}
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: design id %q must be lowercase letters, digits, '.', '_' or '-'", service.ErrInvalidArgument, id)
	}
	rel := path.Join(pid, designsDir, id)
	name := service.DesignName(parent, id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exists(rel) {
		return nil, fmt.Errorf("%w: %s", service.ErrAlreadyExists, name)
	}
	contents, err := s.contents(ctx, files, blobs, DesignDescriptor, id, designExtends, false)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(s.root, pid, designsDir), 0o755); err != nil {
		return nil, err
	}
	if err := s.place(contents, rel, ""); err != nil {
		return nil, err
	}
	return s.read.Design(ctx, name)
}

// ReplaceDesign swaps a design's whole file set.
func (s *DirStore) ReplaceDesign(ctx context.Context, name string, files []*webapi.SnapshotFile, blobs service.BlobSource) (*webapi.Design, error) {
	rel, id, err := s.ownedDesign(ctx, name)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	contents, err := s.contents(ctx, files, blobs, DesignDescriptor, id, designExtends, false)
	if err != nil {
		return nil, err
	}
	if err := s.place(contents, rel, ""); err != nil {
		return nil, err
	}
	return s.read.Design(ctx, name)
}

// DeleteDesign removes a design folder.
func (s *DirStore) DeleteDesign(ctx context.Context, name string) error {
	rel, _, err := s.ownedDesign(ctx, name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remove(rel)
}

// ownedProject returns the id of a project this store wrote. A project the server reads from a
// mounted folder is ErrFailedPrecondition, and one it reads nowhere is ErrNotFound.
func (s *DirStore) ownedProject(ctx context.Context, name string) (string, error) {
	id, ok := service.ProjectID(name)
	if !ok {
		return "", fmt.Errorf("%w: %q is not a project resource name", service.ErrInvalidArgument, name)
	}
	if s.exists(path.Join(id, ProjectDescriptor)) {
		return id, nil
	}
	if _, err := s.read.Project(ctx, name); err == nil {
		return "", fmt.Errorf("%w: %s is declared in a mounted folder, which is read-only", service.ErrFailedPrecondition, name)
	}
	return "", fmt.Errorf("%w: no stored project %s", service.ErrNotFound, name)
}

// ownedDesign returns the folder, relative to the root, and the id of a design this store wrote.
func (s *DirStore) ownedDesign(ctx context.Context, name string) (rel, id string, err error) {
	parent, id, ok := service.SplitDesignName(name)
	if !ok {
		return "", "", fmt.Errorf("%w: %q is not a design resource name", service.ErrInvalidArgument, name)
	}
	pid, ok := service.ProjectID(parent)
	if !ok {
		return "", "", fmt.Errorf("%w: %q is not a design resource name", service.ErrInvalidArgument, name)
	}
	rel = path.Join(pid, designsDir, id)
	if s.exists(path.Join(rel, DesignDescriptor)) {
		return rel, id, nil
	}
	if _, err := s.read.Design(ctx, name); err == nil {
		return "", "", fmt.Errorf("%w: %s is declared in a mounted folder, which is read-only", service.ErrFailedPrecondition, name)
	}
	return "", "", fmt.Errorf("%w: no stored design %s", service.ErrNotFound, name)
}

func (s *DirStore) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(rel)))
	return err == nil
}

// contents checks a file set and returns each file's bytes by its clean path. The descriptor is
// rewritten to name id, and for a design to extend its project, then parsed, so a set that would not
// read back is refused before anything is written. A project's set may not reach into designs/.
func (s *DirStore) contents(ctx context.Context, files []*webapi.SnapshotFile, blobs service.BlobSource, descriptor, id, extends string, project bool) (map[string][]byte, error) {
	out := map[string][]byte{}
	var missing []string
	for _, f := range files {
		p, err := storedPath(f.GetPath())
		if err != nil {
			return nil, err
		}
		if project && (p == designsDir || strings.HasPrefix(p, designsDir+"/")) {
			return nil, fmt.Errorf("%w: %q is under %s/, which holds the project's designs; create each as a design", service.ErrInvalidArgument, f.GetPath(), designsDir)
		}
		if _, dup := out[p]; dup {
			return nil, fmt.Errorf("%w: %q is listed twice", service.ErrInvalidArgument, f.GetPath())
		}
		b, ok := blobs(ctx, f.GetSha256())
		if !ok {
			missing = append(missing, f.GetSha256())
			out[p] = nil
			continue
		}
		sum := sha256.Sum256(b)
		if "sha256:"+hex.EncodeToString(sum[:]) != f.GetSha256() || int64(len(b)) != f.GetSize() {
			return nil, fmt.Errorf("%w: the contents sent for %q do not match its hash and size", service.ErrInvalidArgument, f.GetPath())
		}
		out[p] = b
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, &service.BlobsMissingError{Hashes: slices.Compact(missing)}
	}
	raw, ok := out[descriptor]
	if !ok {
		return nil, fmt.Errorf("%w: the files hold no %s", service.ErrInvalidArgument, descriptor)
	}
	fixed, err := withIdentity(raw, id, extends)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", service.ErrInvalidArgument, descriptor, err)
	}
	if project {
		_, _, _, err = ParseProject(bytes.NewReader(fixed))
	} else {
		_, _, _, err = ParseDesign(bytes.NewReader(fixed))
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrInvalidArgument, err)
	}
	out[descriptor] = fixed
	return out, nil
}

// storedPath cleans a file's path and refuses one that leaves its folder or hides in a dot-folder.
// FSStore skips dot-folders, so a file there would be written and never read.
func storedPath(p string) (string, error) {
	if err := validRel("file", p); err != nil {
		return "", fmt.Errorf("%w: %v", service.ErrInvalidArgument, err)
	}
	clean := CleanRel(p)
	for _, seg := range strings.Split(clean, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", fmt.Errorf("%w: file %q is in a hidden folder or is hidden", service.ErrInvalidArgument, p)
		}
	}
	return clean, nil
}

// withIdentity sets a descriptor's `name` to id and, when extends is set, its `extends`. A name
// already naming another id is an error. It edits the YAML tree, so the descriptor's comments and
// order survive.
func withIdentity(raw []byte, id, extends string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("is not a mapping")
	}
	set := func(key, value string, check bool) error {
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value != key {
				continue
			}
			if check && m.Content[i+1].Value != "" && m.Content[i+1].Value != value {
				return fmt.Errorf("names %s %q, but this is %q", key, m.Content[i+1].Value, value)
			}
			m.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Value: value}
			return nil
		}
		m.Content = append([]*yaml.Node{{Kind: yaml.ScalarNode, Value: key}, {Kind: yaml.ScalarNode, Value: value}}, m.Content...)
		return nil
	}
	if extends != "" {
		if err := set("extends", extends, false); err != nil {
			return nil, err
		}
	}
	if err := set("name", id, true); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), enc.Close()
}

// place writes contents into a new dot-folder and renames it to rel, swapping out whatever was
// there. keep names a subfolder of the old folder to carry into the new one, such as a project's
// designs.
func (s *DirStore) place(contents map[string][]byte, rel, keep string) error {
	tmp, err := os.MkdirTemp(s.root, ".new-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for p, b := range contents {
		dst := filepath.Join(tmp, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
	}
	dst := filepath.Join(s.root, filepath.FromSlash(rel))
	if _, err := os.Stat(dst); errors.Is(err, fs.ErrNotExist) {
		return os.Rename(tmp, dst)
	}
	if keep != "" {
		if err := os.Rename(filepath.Join(dst, keep), filepath.Join(tmp, keep)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	old, err := os.MkdirTemp(s.root, ".old-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(old)
	if err := os.Rename(dst, filepath.Join(old, "x")); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// remove moves rel aside before deleting it, so a reader never lists a half-deleted folder.
func (s *DirStore) remove(rel string) error {
	old, err := os.MkdirTemp(s.root, ".old-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(old)
	return os.Rename(filepath.Join(s.root, filepath.FromSlash(rel)), filepath.Join(old, "x"))
}

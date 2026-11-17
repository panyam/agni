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

// DirStore is the folder adapter of service.DesignStore (agni issue 1693). It keeps each design as
// one folder under its root, `<design>/design.yaml` beside the design's files. A design ties to no
// project, so its `design.yaml` extends nothing, and the CLI reads a stored design as a plain folder.
//
// A write builds the new files in a dot-folder under the root and renames it into place, so a
// reader sees the old folder or the new one. Reads skip dot-folders, so a half-built one is never
// listed. One mutex serialises every write, since a write is rare beside reads.
type DirStore struct {
	root string
	tree Tree
	fs   *FSStore
	mu   sync.Mutex
}

// NewDirStore returns a store over root, created if absent. mount names the mount a server serves
// root under, and each design's URIs are written against it.
func NewDirStore(root, mount string) (*DirStore, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	t := Tree{Mount: mount, FS: os.DirFS(abs)}
	return &DirStore{root: abs, tree: t, fs: NewFSStore(t)}, nil
}

var _ service.DesignStore = (*DirStore)(nil)

// Design reads one stored design.
func (s *DirStore) Design(_ context.Context, id string) (*webapi.Design, error) {
	if !idPattern.MatchString(id) || !s.exists(path.Join(id, DesignDescriptor)) {
		return nil, fmt.Errorf("%w: no stored design %s", service.ErrNotFound, service.StandaloneDesignName(id))
	}
	_, d, err := s.fs.loadDesign(s.tree, id)
	if err != nil {
		return nil, err
	}
	d.Name = service.StandaloneDesignName(id)
	return d, nil
}

// Designs reads every stored design, ordered by name.
func (s *DirStore) Designs(ctx context.Context) ([]*webapi.Design, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var out []*webapi.Design
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || !s.exists(path.Join(e.Name(), DesignDescriptor)) {
			continue
		}
		d, err := s.Design(ctx, e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// CreateDesign writes a new design folder.
func (s *DirStore) CreateDesign(ctx context.Context, id string, files []*webapi.SnapshotFile, blobs service.BlobSource) (*webapi.Design, error) {
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: design id %q must be lowercase letters, digits, '.', '_' or '-'", service.ErrInvalidArgument, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exists(id) {
		return nil, fmt.Errorf("%w: %s", service.ErrAlreadyExists, service.StandaloneDesignName(id))
	}
	contents, err := s.contents(ctx, files, blobs, id)
	if err != nil {
		return nil, err
	}
	if err := s.place(contents, id); err != nil {
		return nil, err
	}
	return s.Design(ctx, id)
}

// ReplaceDesign swaps a stored design's whole file set.
func (s *DirStore) ReplaceDesign(ctx context.Context, id string, files []*webapi.SnapshotFile, blobs service.BlobSource) (*webapi.Design, error) {
	if _, err := s.Design(ctx, id); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	contents, err := s.contents(ctx, files, blobs, id)
	if err != nil {
		return nil, err
	}
	if err := s.place(contents, id); err != nil {
		return nil, err
	}
	return s.Design(ctx, id)
}

// DeleteDesign removes a stored design's folder.
func (s *DirStore) DeleteDesign(ctx context.Context, id string) error {
	if _, err := s.Design(ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remove(id)
}

func (s *DirStore) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(rel)))
	return err == nil
}

// contents checks a file set and returns each file's bytes by its clean path. The `design.yaml` is
// rewritten to name id, then parsed, so a set that would not read back is refused before anything is
// written.
func (s *DirStore) contents(ctx context.Context, files []*webapi.SnapshotFile, blobs service.BlobSource, id string) (map[string][]byte, error) {
	out := map[string][]byte{}
	var missing []string
	for _, f := range files {
		p, err := storedPath(f.GetPath())
		if err != nil {
			return nil, err
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
	raw, ok := out[DesignDescriptor]
	if !ok {
		return nil, fmt.Errorf("%w: the files hold no %s", service.ErrInvalidArgument, DesignDescriptor)
	}
	fixed, err := withIdentity(raw, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", service.ErrInvalidArgument, DesignDescriptor, err)
	}
	if _, _, _, err = ParseDesign(bytes.NewReader(fixed)); err != nil {
		return nil, fmt.Errorf("%w: %v", service.ErrInvalidArgument, err)
	}
	out[DesignDescriptor] = fixed
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

// withIdentity sets a descriptor's `name` to id. A name already naming another id is an error, and
// so is an `extends`, since a stored design stands alone and packs carry the rules a project would
// (agni issue 1704). It edits the YAML tree, so the descriptor's comments and order survive.
func withIdentity(raw []byte, id string) ([]byte, error) {
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
	named := false
	for i := 0; i+1 < len(m.Content); i += 2 {
		switch k, v := m.Content[i].Value, m.Content[i+1]; k {
		case "extends":
			return nil, fmt.Errorf("line %d: extends %q; a stored design stands alone, so it extends no project", m.Content[i].Line, v.Value)
		case "name":
			if v.Value != "" && v.Value != id {
				return nil, fmt.Errorf("names name %q, but this is %q", v.Value, id)
			}
			m.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Value: id}
			named = true
		}
	}
	if !named {
		m.Content = append([]*yaml.Node{{Kind: yaml.ScalarNode, Value: "name"}, {Kind: yaml.ScalarNode, Value: id}}, m.Content...)
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
// there.
func (s *DirStore) place(contents map[string][]byte, rel string) error {
	tmp, err := os.MkdirTemp(s.root, ".new-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for p, b := range contents {
		dst := filepath.Join(tmp, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o600); err != nil {
			return err
		}
	}
	dst := filepath.Join(s.root, filepath.FromSlash(rel))
	if _, err := os.Stat(dst); errors.Is(err, fs.ErrNotExist) {
		return os.Rename(tmp, dst)
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

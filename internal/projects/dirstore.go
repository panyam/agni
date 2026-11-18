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
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// DirStore is the folder adapter of service.DesignStore (agni issues 1693, 1696). It keeps each
// design as one folder under its root, and each artifact as a subfolder holding the artifact's files.
// The store writes the design's `design.yaml` itself, naming the artifact analysis reads as its entry
// and the others as companions, so a stored design reads as a plain folder to the CLI and to the
// mount the server serves it under. A design with no artifact a reader opens keeps its record in
// `design.draft.yaml` instead, since a `design.yaml` must name an entry, and FSStore never lists it.
//
// A write builds the whole new folder in a dot-folder under the root and renames it into place, so a
// reader sees the old design or the new one. Untouched artifacts are hard-linked into the new folder
// rather than copied. Reads skip dot-folders, so a half-built one is never listed. One mutex
// serialises every write, since a write is rare beside reads.
type DirStore struct {
	root string
	tree Tree
	fs   *FSStore
	mu   sync.Mutex
}

// draftDescriptor holds the record of a design with no artifact a reader opens.
const draftDescriptor = "design.draft.yaml"

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
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: no stored design %s", service.ErrNotFound, service.StandaloneDesignName(id))
	}
	var d *webapi.Design
	switch {
	case s.exists(path.Join(id, DesignDescriptor)):
		var err error
		if _, d, err = s.fs.loadDesign(s.tree, id); err != nil {
			return nil, err
		}
	case s.exists(path.Join(id, draftDescriptor)):
		rec, err := s.record(id)
		if err != nil {
			return nil, err
		}
		base, err := artifact.New(s.tree.Mount, id)
		if err != nil {
			return nil, err
		}
		d = &webapi.Design{Title: orName(rec.Title, id), Uri: base.String(), Series: rec.Series, FlattenHierarchy: rec.FlattenHierarchy}
		if fi, err := os.Stat(s.abs(path.Join(id, draftDescriptor))); err == nil {
			d.UpdateTime = timestamppb.New(fi.ModTime())
		}
	default:
		return nil, fmt.Errorf("%w: no stored design %s", service.ErrNotFound, service.StandaloneDesignName(id))
	}
	d.Name = service.StandaloneDesignName(id)
	arts, err := s.artifactIDs(id)
	if err != nil {
		return nil, err
	}
	for _, a := range arts {
		d.Artifacts = append(d.Artifacts, service.ArtifactName(id, a))
	}
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
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || !s.isDesign(e.Name()) {
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

// CreateDesign writes a new design folder holding only its record.
func (s *DirStore) CreateDesign(ctx context.Context, id string, rec service.DesignRecord) (*webapi.Design, error) {
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: design id %q must be lowercase letters, digits, '.', '_' or '-'", service.ErrInvalidArgument, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exists(id) {
		return nil, fmt.Errorf("%w: %s", service.ErrAlreadyExists, service.StandaloneDesignName(id))
	}
	if err := s.rebuild(ctx, id, rec, nil, "", nil); err != nil {
		return nil, err
	}
	return s.Design(ctx, id)
}

// UpdateDesign rewrites a design's record and keeps its artifacts.
func (s *DirStore) UpdateDesign(ctx context.Context, id string, update func(*service.DesignRecord)) (*webapi.Design, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.existing(id)
	if err != nil {
		return nil, err
	}
	update(&rec)
	have, err := s.artifactIDs(id)
	if err != nil {
		return nil, err
	}
	if err := s.rebuild(ctx, id, rec, nil, "", have); err != nil {
		return nil, err
	}
	return s.Design(ctx, id)
}

// DeleteDesign removes a stored design's folder.
func (s *DirStore) DeleteDesign(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.existing(id); err != nil {
		return err
	}
	return s.remove(id)
}

// Artifacts reads a design's artifacts, hashing each file and reading the files to find each one's
// type and entry. An artifact's update_time is the newest of its files' modification times. A write
// links untouched files into the new folder, so their times survive it.
func (s *DirStore) Artifacts(_ context.Context, id string) ([]*webapi.Artifact, error) {
	if _, err := s.existing(id); err != nil {
		return nil, err
	}
	return s.readArtifacts(s.abs(id), id)
}

// WriteArtifacts replaces or creates artifacts, in one rename of the design's folder.
func (s *DirStore) WriteArtifacts(ctx context.Context, id string, writes []service.ArtifactWrite, blobs service.BlobSource) ([]*webapi.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.existing(id)
	if err != nil {
		return nil, err
	}
	have, err := s.artifactIDs(id)
	if err != nil {
		return nil, err
	}
	put := map[string]map[string][]byte{}
	var missing []string
	for _, w := range writes {
		if !idPattern.MatchString(w.ID) || w.ID == DesignDescriptor || w.ID == draftDescriptor {
			return nil, fmt.Errorf("%w: artifact id %q must be lowercase letters, digits, '.', '_' or '-', and not a descriptor's name", service.ErrInvalidArgument, w.ID)
		}
		if !w.AllowMissing && !slices.Contains(have, w.ID) {
			return nil, fmt.Errorf("%w: no artifact %s", service.ErrNotFound, service.ArtifactName(id, w.ID))
		}
		files, lacking, err := s.contents(ctx, w, blobs)
		if err != nil {
			return nil, err
		}
		missing = append(missing, lacking...)
		put[w.ID] = files
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, &service.BlobsMissingError{Hashes: slices.Compact(missing)}
	}
	if err := s.rebuild(ctx, id, rec, put, "", have); err != nil {
		return nil, err
	}
	all, err := s.readArtifacts(s.abs(id), id)
	if err != nil {
		return nil, err
	}
	out := make([]*webapi.Artifact, 0, len(writes))
	for _, w := range writes {
		for _, a := range all {
			if a.GetName() == service.ArtifactName(id, w.ID) {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

// DeleteArtifact removes one artifact, rewriting the design's record without it.
func (s *DirStore) DeleteArtifact(ctx context.Context, id, artifactID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.existing(id)
	if err != nil {
		return err
	}
	have, err := s.artifactIDs(id)
	if err != nil {
		return err
	}
	if !slices.Contains(have, artifactID) {
		return fmt.Errorf("%w: no artifact %s", service.ErrNotFound, service.ArtifactName(id, artifactID))
	}
	return s.rebuild(ctx, id, rec, nil, artifactID, have)
}

func (s *DirStore) abs(rel string) string { return filepath.Join(s.root, filepath.FromSlash(rel)) }

func (s *DirStore) exists(rel string) bool {
	_, err := os.Stat(s.abs(rel))
	return err == nil
}

func (s *DirStore) isDesign(id string) bool {
	return s.exists(path.Join(id, DesignDescriptor)) || s.exists(path.Join(id, draftDescriptor))
}

// existing returns the record of a design this store keeps, and ErrNotFound for any other id.
func (s *DirStore) existing(id string) (service.DesignRecord, error) {
	if !idPattern.MatchString(id) || !s.isDesign(id) {
		return service.DesignRecord{}, fmt.Errorf("%w: no stored design %s", service.ErrNotFound, service.StandaloneDesignName(id))
	}
	return s.record(id)
}

// record reads the title, series and flatten_hierarchy a design's descriptor holds.
func (s *DirStore) record(id string) (service.DesignRecord, error) {
	name := path.Join(id, DesignDescriptor)
	if !s.exists(name) {
		name = path.Join(id, draftDescriptor)
	}
	raw, err := os.ReadFile(s.abs(name))
	if err != nil {
		return service.DesignRecord{}, err
	}
	var y descriptorYAML
	if err := yaml.Unmarshal(raw, &y); err != nil {
		return service.DesignRecord{}, fmt.Errorf("%s: %w", name, err)
	}
	return service.DesignRecord{Title: y.Title, Series: y.Series, FlattenHierarchy: y.FlattenHierarchy}, nil
}

// artifactIDs lists a design's artifact subfolders, ordered by name.
func (s *DirStore) artifactIDs(id string) ([]string, error) {
	entries, err := os.ReadDir(s.abs(id))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// contents checks one artifact's files and returns each file's bytes by its clean path, and the
// hashes blobs lacks.
func (s *DirStore) contents(ctx context.Context, w service.ArtifactWrite, blobs service.BlobSource) (map[string][]byte, []string, error) {
	out := map[string][]byte{}
	var missing []string
	for _, f := range w.Files {
		p, err := storedPath(f.GetPath())
		if err != nil {
			return nil, nil, err
		}
		if b := path.Base(p); b == DesignDescriptor || b == ProjectDescriptor || b == draftDescriptor {
			return nil, nil, fmt.Errorf("%w: artifact %s holds %q; the store writes a design's descriptor itself, and an artifact holds the design's own files", service.ErrInvalidArgument, w.ID, f.GetPath())
		}
		if _, dup := out[p]; dup {
			return nil, nil, fmt.Errorf("%w: %q is listed twice", service.ErrInvalidArgument, f.GetPath())
		}
		b, ok := blobs(ctx, f.GetSha256())
		if !ok {
			missing = append(missing, f.GetSha256())
			out[p] = nil
			continue
		}
		if hashOf(b) != f.GetSha256() || int64(len(b)) != f.GetSize() {
			return nil, nil, fmt.Errorf("%w: the contents sent for %q do not match its hash and size", service.ErrInvalidArgument, f.GetPath())
		}
		out[p] = b
	}
	return out, missing, nil
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// rebuild writes a design's whole new folder and renames it into place. The new folder holds every
// artifact in have except drop and those put replaces, linked from the old folder, then put's files,
// then a descriptor naming the composed entry and companions. A design no reader can open gets a
// draft descriptor. Nothing is renamed until the new folder reads back.
func (s *DirStore) rebuild(_ context.Context, id string, rec service.DesignRecord, put map[string]map[string][]byte, drop string, have []string) error {
	tmp, err := os.MkdirTemp(s.root, ".new-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	store, err := os.OpenRoot(s.root)
	if err != nil {
		return err
	}
	defer store.Close()
	for _, a := range have {
		if _, replaced := put[a]; replaced || a == drop {
			continue
		}
		if err := linkTree(store, path.Join(id, a), path.Join(filepath.Base(tmp), a)); err != nil {
			return err
		}
	}
	for a, files := range put {
		for p, b := range files {
			dst := filepath.Join(tmp, a, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
				return err
			}
			if err := os.WriteFile(dst, b, 0o600); err != nil {
				return err
			}
		}
	}
	arts, err := s.readArtifacts(tmp, id)
	if err != nil {
		return err
	}
	roles, err := rolesOf(tmp, arts)
	if err != nil {
		return err
	}
	entry, companions, err := composeDesign(roles)
	if err != nil {
		return err
	}
	y := descriptorYAML{Name: id, Title: rec.Title, Entry: entry, Companions: companions, Series: rec.Series, FlattenHierarchy: rec.FlattenHierarchy}
	raw, err := yaml.Marshal(&y)
	if err != nil {
		return err
	}
	name := draftDescriptor
	if entry != "" {
		name = DesignDescriptor
		if _, _, _, err := ParseDesign(bytes.NewReader(raw)); err != nil {
			return fmt.Errorf("%w: %v", service.ErrInvalidArgument, err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, name), raw, 0o600); err != nil {
		return err
	}
	return s.swap(tmp, id)
}

// readArtifacts reads the artifact subfolders of dir, a design folder, as the design id's artifacts.
// It reads through an os.Root over dir, so a symlink in the store cannot lead a read outside it.
func (s *DirStore) readArtifacts(dir, id string) ([]*webapi.Artifact, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	fsys := r.FS()
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var out []*webapi.Artifact
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		a := &webapi.Artifact{Name: service.ArtifactName(id, e.Name())}
		sub, err := fs.Sub(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		var newest time.Time
		var paths []string
		err = fs.WalkDir(sub, ".", func(p string, de fs.DirEntry, err error) error {
			if err != nil || de.IsDir() {
				return err
			}
			b, err := fs.ReadFile(sub, p)
			if err != nil {
				return err
			}
			paths = append(paths, p)
			a.Files = append(a.Files, &webapi.SnapshotFile{Path: p, Sha256: hashOf(b), Size: int64(len(b))})
			if fi, err := de.Info(); err == nil && fi.ModTime().After(newest) {
				newest = fi.ModTime()
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		role, err := classifyArtifact(e.Name(), paths, func(p string) ([]byte, error) { return fs.ReadFile(sub, p) })
		if err != nil {
			return nil, err
		}
		a.Type, a.Entry = role.typ, role.entry
		if !newest.IsZero() {
			a.UpdateTime = timestamppb.New(newest)
		}
		out = append(out, a)
	}
	return out, nil
}

// rolesOf classifies each artifact read from dir, through an os.Root over it.
func rolesOf(dir string, arts []*webapi.Artifact) ([]artifactRole, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out []artifactRole
	for _, a := range arts {
		_, id, _ := service.ArtifactID(a.GetName())
		var paths []string
		for _, f := range a.GetFiles() {
			paths = append(paths, f.GetPath())
		}
		sub, err := fs.Sub(r.FS(), id)
		if err != nil {
			return nil, err
		}
		role, err := classifyArtifact(id, paths, func(p string) ([]byte, error) { return fs.ReadFile(sub, p) })
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, nil
}

// linkTree hard-links every file under src into dst, both relative to r, copying a file where the
// filesystem refuses a link. Every operation goes through r, so a symlink in the store cannot lead
// one outside it.
func linkTree(r *os.Root, src, dst string) error {
	return fs.WalkDir(r.FS(), src, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		to := path.Join(dst, strings.TrimPrefix(strings.TrimPrefix(p, src), "/"))
		if de.IsDir() {
			return r.MkdirAll(to, 0o750)
		}
		if err := r.Link(p, to); err == nil {
			return nil
		}
		b, err := r.ReadFile(p)
		if err != nil {
			return err
		}
		return r.WriteFile(to, b, 0o600)
	})
}

// storedPath cleans a file's path and refuses one that leaves its artifact or hides in a dot-folder.
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

// swap renames the built folder tmp to rel, moving whatever was there aside first.
func (s *DirStore) swap(tmp, rel string) error {
	dst := s.abs(rel)
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
	return os.Rename(s.abs(rel), filepath.Join(old, "x"))
}

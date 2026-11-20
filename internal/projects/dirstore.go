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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/rules/intent"
	"github.com/panyam/goapplib/storage/blobs"
	"github.com/panyam/goapplib/storage/mounts"
)

// DirStore is the folder adapter of service.DesignStore (agni issues 1693, 1696, 1739). It keeps a
// design's record apart from its files.
//
// The record is one YAML file per design, `<id>.yaml` in the store's records folder. It holds the
// design's title, series and flatten_hierarchy, and each artifact's type, entry and manifest of paths
// and hashes. It holds no file bytes.
//
// The files live in the uploads mount. Each file's contents sit once in a BlobDir, by hash. A design
// reads through a view, `<id>/<rev>/<artifact>/<path>` under the mount, whose files are hard links to
// the blobs. The view also holds a `design.yaml` written from the record, so the engine finds the
// design's entry and companions beside its files, as it does in any folder. The record stays the only
// source, and the view is rebuilt from it and the blobs.
//
// A view is never changed once built. A write builds a new revision's view beside the old one, then
// renames the record over its old version, so a reader sees the old design or the new one. An old
// view stays until Sweep finds nothing pointing at it, so a snapshot read from it keeps answering.
// One mutex serialises every write, since a write is rare beside reads.
type DirStore struct {
	records string   // the records folder, one <id>.yaml per design
	views   string   // the uploads mount's root, "" when the server keeps no files
	blobs   *BlobDir // nil when the server keeps no files
	mount   string
	mu      sync.Mutex
	now     func() time.Time // the clock writes are stamped with; time.Now when nil
}

func (s *DirStore) clock() time.Time {
	if s.now != nil {
		return s.now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}

// draftDescriptor names nothing on disk now. It stays an artifact id the store refuses, since a
// folder store once kept a design with no readable artifact under that name.
const draftDescriptor = "design.draft.yaml"

// viewsDir is the folder under the uploads folder that a server serves as the uploads mount.
const viewsDir = "designs"

// NewDirStore returns a store keeping records in records and files in uploads, each created if
// absent. mount names the mount a server serves the views under, and each design's URIs are written
// against it. With uploads empty the store keeps records only, and a write of an artifact's files is
// FAILED_PRECONDITION.
func NewDirStore(records, uploads, mount string) (*DirStore, error) {
	recs, err := filepath.Abs(records)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(recs, 0o750); err != nil {
		return nil, err
	}
	s := &DirStore{records: recs, mount: mount}
	if uploads == "" {
		return s, nil
	}
	up, err := filepath.Abs(uploads)
	if err != nil {
		return nil, err
	}
	if s.blobs, err = NewBlobDir(up); err != nil {
		return nil, err
	}
	s.views = filepath.Join(up, viewsDir)
	if err := os.MkdirAll(s.views, 0o750); err != nil {
		return nil, err
	}
	return s, nil
}

// ViewsRoot is the folder a server serves as the uploads mount, or "" for a store with no uploads.
func ViewsRoot(uploads string) string {
	if uploads == "" {
		return ""
	}
	return filepath.Join(uploads, viewsDir)
}

// Blobs is the store's blob folder, or nil for a store with no uploads. A server reads and writes
// uploaded contents through it, so an upload outlives a restart.
func (s *DirStore) Blobs() *BlobDir { return s.blobs }

var _ service.DesignStore = (*DirStore)(nil)

// recordYAML is a design's record as the store writes it.
type recordYAML struct {
	Title            string           `yaml:"title,omitempty"`
	Series           string           `yaml:"series,omitempty"`
	FlattenHierarchy bool             `yaml:"flatten_hierarchy,omitempty"`
	Packs            []string         `yaml:"packs,omitempty"`
	Intent           yaml.Node        `yaml:"intent,omitempty"`
	Rev              string           `yaml:"rev,omitempty"`
	UpdateTime       time.Time        `yaml:"update_time"`
	Artifacts        []artifactRecord `yaml:"artifacts,omitempty"`
	// OnSourceChange is the design's webapi.SourceChange by name, empty for the default (agni issue
	// 1726).
	OnSourceChange string `yaml:"on_source_change,omitempty"`
}

type artifactRecord struct {
	ID         string       `yaml:"id"`
	Type       string       `yaml:"type"`
	Entry      string       `yaml:"entry,omitempty"`
	Companions []string     `yaml:"companions,omitempty"`
	UpdateTime time.Time    `yaml:"update_time"`
	Files      []fileRecord `yaml:"files"`
	// Source is where in a mount the files were read from, by a link (agni issue 1723) or when a
	// mounted folder's design was added (agni issue 1736). An uploaded artifact has none.
	Source *sourceRecord `yaml:"source,omitempty"`
}

// sourceRecord is an artifact's webapi.ArtifactSource as the record keeps it. An added design's
// artifact has a mount and path alone, and a link fills the rest.
type sourceRecord struct {
	Mount    string    `yaml:"mount"`
	Path     string    `yaml:"path,omitempty"`
	Ref      string    `yaml:"ref,omitempty"`
	Follow   bool      `yaml:"follow,omitempty"`
	Version  string    `yaml:"version,omitempty"`
	ReadTime time.Time `yaml:"read_time,omitempty"`
	ReadHash string    `yaml:"read_hash,omitempty"`
	// AccessSource is the private repo the files came from, kept after an unlink (agni issue 1728).
	AccessSource string `yaml:"access_source,omitempty"`
	// NewerVersion is a version the source moved to that the files aren't at yet, and NewerTime when
	// the server saw it (agni issue 1726). A write of new files makes a new record without them.
	NewerVersion string    `yaml:"newer_version,omitempty"`
	NewerTime    time.Time `yaml:"newer_time,omitempty"`
}

func sourceRecordOf(src *webapi.ArtifactSource) *sourceRecord {
	if src == nil {
		return nil
	}
	out := &sourceRecord{Mount: src.GetMount(), Path: src.GetPath(), Ref: src.GetRef(), Follow: src.GetFollow(), Version: src.GetVersion(), ReadHash: src.GetReadHash(), AccessSource: src.GetAccessSource()}
	if src.GetReadTime() != nil {
		out.ReadTime = src.GetReadTime().AsTime().UTC()
	}
	return out
}

func (r *sourceRecord) proto() *webapi.ArtifactSource {
	out := &webapi.ArtifactSource{Mount: r.Mount, Path: r.Path, Ref: r.Ref, Follow: r.Follow, Version: r.Version, ReadHash: r.ReadHash, AccessSource: r.AccessSource}
	if !r.ReadTime.IsZero() {
		out.ReadTime = timestamppb.New(r.ReadTime)
	}
	// A mark the files caught up with says nothing more.
	if r.NewerVersion != "" && r.NewerVersion != r.Version {
		out.NewerVersion = r.NewerVersion
		out.NewerTime = timestamppb.New(r.NewerTime)
	}
	return out
}

type fileRecord struct {
	Path   string `yaml:"path"`
	Sha256 string `yaml:"sha256"`
	Size   int64  `yaml:"size"`
}

func (a artifactRecord) role() artifactRole {
	return artifactRole{id: a.ID, typ: webapi.ArtifactType(webapi.ArtifactType_value[a.Type]), entry: a.Entry, companions: a.Companions}
}

// Design reads one stored design from its record.
func (s *DirStore) Design(_ context.Context, id string) (*webapi.Design, error) {
	rec, err := s.load(id)
	if err != nil {
		return nil, err
	}
	return s.designOf(id, rec)
}

func (s *DirStore) designOf(id string, rec recordYAML) (*webapi.Design, error) {
	entry, _, raw, err := describe(id, rec)
	if err != nil {
		return nil, err
	}
	var d *webapi.Design
	if entry == "" || rec.Rev == "" {
		base, err := mounts.NewURI(s.mount, id)
		if err != nil {
			return nil, err
		}
		d = &webapi.Design{Title: orName(rec.Title, id), Uri: base.String(), Series: rec.Series, FlattenHierarchy: rec.FlattenHierarchy}
		in, err := decodeIntent(rec.Intent)
		if err != nil {
			return nil, err
		}
		if in != nil {
			d.Config = &webapi.AnalysisConfig{Intent: in}
		}
	} else {
		if _, d, _, err = ParseDesign(bytes.NewReader(raw)); err != nil {
			return nil, err
		}
		d.Series = rec.Series
		base, err := mounts.NewURI(s.mount, path.Join(id, rec.Rev))
		if err != nil {
			return nil, err
		}
		join := func(rel string) (string, error) {
			u, err := base.Join(rel)
			if err != nil {
				return "", err
			}
			return u.String(), nil
		}
		d.Uri = base.String()
		if d.EntryUri, err = join(d.GetEntryUri()); err != nil {
			return nil, err
		}
		for i, c := range d.GetCompanionUris() {
			if d.CompanionUris[i], err = join(c); err != nil {
				return nil, err
			}
		}
		for _, dir := range symbolDirs(rec) {
			u, err := join(dir)
			if err != nil {
				return nil, err
			}
			if d.Config == nil {
				d.Config = &webapi.AnalysisConfig{}
			}
			d.Config.SymbolPathUris = append(d.Config.SymbolPathUris, u)
		}
	}
	d.Packs = rec.Packs
	d.OnSourceChange = webapi.SourceChange(webapi.SourceChange_value[rec.OnSourceChange])
	d.Name = service.StandaloneDesignName(id)
	if !rec.UpdateTime.IsZero() {
		d.UpdateTime = timestamppb.New(rec.UpdateTime)
	}
	for _, a := range rec.Artifacts {
		d.Artifacts = append(d.Artifacts, service.ArtifactName(id, a.ID))
	}
	return d, nil
}

// describe composes a record's entry and companions, and writes the descriptor its view holds. A
// record with no artifact a reader opens has an empty entry and no descriptor.
func describe(id string, rec recordYAML) (entry string, companions []string, raw []byte, err error) {
	roles := make([]artifactRole, 0, len(rec.Artifacts))
	for _, a := range rec.Artifacts {
		roles = append(roles, a.role())
	}
	if entry, companions, err = composeDesign(roles); err != nil || entry == "" {
		return entry, companions, nil, err
	}
	// The view's descriptor holds what reading the board needs. The series stays in the record alone,
	// so moving a design to another series keeps its view.
	y := descriptorYAML{Name: id, Title: rec.Title, Entry: entry, Companions: companions, FlattenHierarchy: rec.FlattenHierarchy, Intent: rec.Intent, Symbols: symbolDirs(rec)}
	if raw, err = yaml.Marshal(&y); err != nil {
		return "", nil, nil, err
	}
	if _, _, _, err := ParseDesign(bytes.NewReader(raw)); err != nil {
		return "", nil, nil, fmt.Errorf("%w: %v", service.ErrInvalidArgument, err)
	}
	return entry, companions, raw, nil
}

// Designs reads every stored design, ordered by name.
func (s *DirStore) Designs(ctx context.Context) ([]*webapi.Design, error) {
	ids, err := s.ids()
	if err != nil {
		return nil, err
	}
	out := make([]*webapi.Design, 0, len(ids))
	for _, id := range ids {
		d, err := s.Design(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// ids lists the designs the records folder holds, ordered by id.
func (s *DirStore) ids() ([]string, error) {
	entries, err := os.ReadDir(s.records)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".yaml")
		if ok && !e.IsDir() && idPattern.MatchString(id) {
			out = append(out, id)
		}
	}
	return out, nil
}

// CreateDesign writes a new design's record, with no artifacts.
func (s *DirStore) CreateDesign(_ context.Context, id string, rec service.DesignRecord) (*webapi.Design, error) {
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: design id %q must be lowercase letters, digits, '.', '_' or '-'", service.ErrInvalidArgument, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.load(id); err == nil {
		return nil, fmt.Errorf("%w: %s", service.ErrAlreadyExists, service.StandaloneDesignName(id))
	} else if !errors.Is(err, service.ErrNotFound) {
		return nil, err
	}
	r := recordYAML{Title: rec.Title, Series: rec.Series, FlattenHierarchy: rec.FlattenHierarchy, Packs: rec.Packs}
	var err error
	if r.Intent, err = encodeIntent(rec.Intent); err != nil {
		return nil, err
	}
	if err := s.commit(id, &r); err != nil {
		return nil, err
	}
	return s.designOf(id, r)
}

// UpdateDesign rewrites a design's record and keeps its artifacts.
func (s *DirStore) UpdateDesign(_ context.Context, id string, update func(*service.DesignRecord)) (*webapi.Design, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.load(id)
	if err != nil {
		return nil, err
	}
	rec := service.DesignRecord{Title: r.Title, Series: r.Series, FlattenHierarchy: r.FlattenHierarchy, Packs: r.Packs,
		OnSourceChange: webapi.SourceChange(webapi.SourceChange_value[r.OnSourceChange]), Newer: map[string]service.NewerVersion{}}
	if rec.Intent, err = decodeIntent(r.Intent); err != nil {
		return nil, err
	}
	for _, a := range r.Artifacts {
		if a.Source != nil && a.Source.NewerVersion != "" {
			rec.Newer[a.ID] = service.NewerVersion{Version: a.Source.NewerVersion, Seen: a.Source.NewerTime}
		}
	}
	update(&rec)
	r.Title, r.Series, r.FlattenHierarchy, r.Packs = rec.Title, rec.Series, rec.FlattenHierarchy, rec.Packs
	r.OnSourceChange = ""
	if rec.OnSourceChange != webapi.SourceChange_SOURCE_CHANGE_UNSPECIFIED {
		r.OnSourceChange = rec.OnSourceChange.String()
	}
	for i, a := range r.Artifacts {
		if a.Source == nil {
			continue
		}
		src := *a.Source
		n := rec.Newer[a.ID]
		src.NewerVersion, src.NewerTime = n.Version, n.Seen.UTC()
		r.Artifacts[i].Source = &src
	}
	if r.Intent, err = encodeIntent(rec.Intent); err != nil {
		return nil, err
	}
	if err := s.commit(id, &r); err != nil {
		return nil, err
	}
	return s.designOf(id, r)
}

// DeleteDesign removes a design's record. Its views and blobs stay until Sweep, so a snapshot read
// from them keeps answering until then.
func (s *DirStore) DeleteDesign(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.load(id); err != nil {
		return err
	}
	if err := os.Remove(s.recordPath(id)); err != nil {
		return err
	}
	// Its revisions go with it, and the next sweep removes their views and files.
	return os.RemoveAll(s.revisionsOf(id))
}

// Artifacts reads a design's artifacts from its record, ordered by id.
func (s *DirStore) Artifacts(_ context.Context, id string) ([]*webapi.Artifact, error) {
	rec, err := s.load(id)
	if err != nil {
		return nil, err
	}
	out := make([]*webapi.Artifact, 0, len(rec.Artifacts))
	for _, a := range rec.Artifacts {
		out = append(out, s.artifactOf(id, a))
	}
	return out, nil
}

// artifactOf is a's resource. An uploaded artifact's source is the uploads mount.
func (s *DirStore) artifactOf(id string, a artifactRecord) *webapi.Artifact {
	out := &webapi.Artifact{Name: service.ArtifactName(id, a.ID), Type: webapi.ArtifactType(webapi.ArtifactType_value[a.Type]), Entry: a.Entry}
	if a.Source != nil {
		out.Source = a.Source.proto()
	} else if s.mount != "" {
		out.Source = &webapi.ArtifactSource{Mount: s.mount}
	}
	if !a.UpdateTime.IsZero() {
		out.UpdateTime = timestamppb.New(a.UpdateTime)
	}
	for _, f := range a.Files {
		out.Files = append(out.Files, &webapi.SnapshotFile{Path: f.Path, Sha256: f.Sha256, Size: f.Size})
	}
	return out
}

// WriteArtifacts replaces or creates artifacts in one write of the design's record.
func (s *DirStore) WriteArtifacts(ctx context.Context, id string, writes []service.ArtifactWrite, blobs service.BlobSource) ([]*webapi.Artifact, error) {
	if s.blobs == nil {
		return nil, fmt.Errorf("%w: this server keeps no uploaded files; start it with --uploads", service.ErrFailedPrecondition)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.load(id)
	if err != nil {
		return nil, err
	}
	next, written, err := s.apply(ctx, id, rec, writes, blobs)
	if err != nil {
		return nil, err
	}
	if err := s.commit(id, &next); err != nil {
		return nil, err
	}
	// A link or a sync read a source, so the files it read are kept as a revision (agni issue 1725).
	if slices.ContainsFunc(writes, func(w service.ArtifactWrite) bool { return w.Source.GetReadHash() != "" }) && next.Rev != "" {
		if _, err := s.keepRevision(id, next, ""); err != nil {
			return nil, err
		}
	}
	out := make([]*webapi.Artifact, 0, len(writes))
	for _, w := range writes {
		out = append(out, s.artifactOf(id, written[w.ID]))
	}
	return out, nil
}

// apply returns rec with writes applied, and the artifacts it wrote by id. It stores each write's
// files as blobs and changes nothing on disk else.
func (s *DirStore) apply(ctx context.Context, id string, rec recordYAML, writes []service.ArtifactWrite, blobs service.BlobSource) (recordYAML, map[string]artifactRecord, error) {
	written := map[string]artifactRecord{}
	var missing []string
	now := s.clock()
	for _, w := range writes {
		if !idPattern.MatchString(w.ID) || w.ID == DesignDescriptor || w.ID == draftDescriptor {
			return recordYAML{}, nil, fmt.Errorf("%w: artifact id %q must be lowercase letters, digits, '.', '_' or '-', and not a descriptor's name", service.ErrInvalidArgument, w.ID)
		}
		if !w.AllowMissing && !slices.ContainsFunc(rec.Artifacts, func(a artifactRecord) bool { return a.ID == w.ID }) {
			return recordYAML{}, nil, fmt.Errorf("%w: no artifact %s", service.ErrNotFound, service.ArtifactName(id, w.ID))
		}
		files, lacking, err := s.contents(ctx, w, blobs)
		if err != nil {
			return recordYAML{}, nil, err
		}
		missing = append(missing, lacking...)
		if len(lacking) > 0 {
			continue
		}
		a, err := s.keep(w.ID, files, now)
		if err != nil {
			return recordYAML{}, nil, err
		}
		a.Source = sourceRecordOf(w.Source)
		written[w.ID] = a
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return recordYAML{}, nil, &service.BlobsMissingError{Hashes: slices.Compact(missing)}
	}
	next := rec
	next.Artifacts = nil
	for _, a := range rec.Artifacts {
		if _, replaced := written[a.ID]; !replaced {
			next.Artifacts = append(next.Artifacts, a)
		}
	}
	for _, a := range written {
		next.Artifacts = append(next.Artifacts, a)
	}
	slices.SortFunc(next.Artifacts, func(a, b artifactRecord) int { return strings.Compare(a.ID, b.ID) })
	return next, written, nil
}

// keep stores an artifact's files as blobs and classifies them into its record.
func (s *DirStore) keep(id string, files map[string][]byte, now time.Time) (artifactRecord, error) {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	a := artifactRecord{ID: id, UpdateTime: now}
	for _, p := range paths {
		h, err := s.blobs.Put(files[p])
		if err != nil {
			return artifactRecord{}, err
		}
		a.Files = append(a.Files, fileRecord{Path: p, Sha256: h, Size: int64(len(files[p]))})
	}
	role, err := classifyArtifact(id, paths, func(p string) ([]byte, error) {
		if b, ok := files[p]; ok {
			return b, nil
		}
		return nil, fs.ErrNotExist
	})
	if err != nil {
		return artifactRecord{}, err
	}
	a.Type, a.Entry, a.Companions = role.typ.String(), role.entry, role.companions
	return a, nil
}

// DeleteArtifact removes one artifact, rewriting the design's record without it.
func (s *DirStore) DeleteArtifact(_ context.Context, id, artifactID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.load(id)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(rec.Artifacts, func(a artifactRecord) bool { return a.ID == artifactID })
	if i < 0 {
		return fmt.Errorf("%w: no artifact %s", service.ErrNotFound, service.ArtifactName(id, artifactID))
	}
	rec.Artifacts = slices.Delete(slices.Clone(rec.Artifacts), i, i+1)
	return s.commit(id, &rec)
}

// commit builds rec's view if it has an entry, then writes rec over the design's old record. The
// view goes first, so a reader that reads the new record finds its files.
func (s *DirStore) commit(id string, rec *recordYAML) error {
	_, _, raw, err := describe(id, *rec)
	if err != nil {
		return err
	}
	rec.Rev = ""
	if raw != nil {
		if rec.Rev, err = s.buildView(id, *rec, raw); err != nil {
			return err
		}
	}
	rec.UpdateTime = s.clock()
	out, err := yaml.Marshal(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.records, ".put-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), s.recordPath(id))
}

// revOf names a view by what it holds: its descriptor and every artifact's paths and hashes. Two
// writes that leave a design holding the same files share one view.
func revOf(rec recordYAML, descriptor []byte) string {
	h := sha256.New()
	h.Write(descriptor)
	for _, a := range rec.Artifacts {
		for _, f := range a.Files {
			fmt.Fprintf(h, "\x00%s/%s\x00%s", a.ID, f.Path, f.Sha256)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// buildView writes the view for rec, unless one with its revision already exists, and returns the
// revision. It builds the view in a dot-folder and renames it into place, so no reader sees half of
// one.
func (s *DirStore) buildView(id string, rec recordYAML, descriptor []byte) (string, error) {
	if s.blobs == nil {
		return "", fmt.Errorf("%w: this server keeps no uploaded files; start it with --uploads", service.ErrFailedPrecondition)
	}
	rev := revOf(rec, descriptor)
	dst := filepath.Join(s.views, id, rev)
	if _, err := os.Stat(dst); err == nil {
		return rev, nil
	}
	if err := os.MkdirAll(filepath.Join(s.views, id), 0o750); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(filepath.Join(s.views, id), ".new-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	for _, a := range rec.Artifacts {
		for _, f := range a.Files {
			to := filepath.Join(tmp, a.ID, filepath.FromSlash(f.Path))
			if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
				return "", err
			}
			if err := s.linkBlob(f.Sha256, to); err != nil {
				return "", err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, DesignDescriptor), descriptor, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", err
	}
	return rev, nil
}

// linkBlob hard-links a blob to `to`, copying it where the filesystem refuses a link.
func (s *DirStore) linkBlob(hash, to string) error { return s.blobs.Link(hash, to) }

// Sweep removes views no record names and blobs nothing points at, each only once it is older than
// grace. keep adds hashes something else holds, such as a snapshot sent as files, and more adds
// referrers such as the mounts registry's snapshots (agni issue 1759). It returns how many views and
// blobs it removed.
func (s *DirStore) Sweep(keep map[string]bool, grace time.Duration, more ...blobs.Referrer) (views, removed int, err error) {
	if s.blobs == nil {
		return 0, 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	refs := map[string]bool{}
	for h := range keep {
		refs[h] = true
	}
	current := map[string]string{}
	ids, err := s.ids()
	if err != nil {
		return 0, 0, err
	}
	for _, id := range ids {
		rec, err := s.load(id)
		if err != nil {
			return 0, 0, err
		}
		current[id] = rec.Rev
		for _, a := range rec.Artifacts {
			for _, f := range a.Files {
				refs[f.Sha256] = true
			}
		}
	}
	// A kept revision keeps its view and its files (agni issue 1725).
	kept, err := s.revisionRefs(ids, refs)
	if err != nil {
		return 0, 0, err
	}
	cutoff := time.Now().Add(-grace)
	designs, err := os.ReadDir(s.views)
	if err != nil {
		return 0, 0, err
	}
	for _, d := range designs {
		// A folder whose name no design id can take, such as the packs' views, belongs to another
		// store, which sweeps it itself.
		if !d.IsDir() || !idPattern.MatchString(d.Name()) {
			continue
		}
		dir := filepath.Join(s.views, d.Name())
		revs, err := os.ReadDir(dir)
		if err != nil {
			return views, 0, err
		}
		left := len(revs)
		for _, r := range revs {
			if r.Name() == current[d.Name()] || kept[d.Name()][r.Name()] {
				continue
			}
			fi, err := r.Info()
			if err != nil || fi.ModTime().After(cutoff) {
				continue
			}
			if err := os.RemoveAll(filepath.Join(dir, r.Name())); err != nil {
				return views, 0, err
			}
			views++
			left--
		}
		if left == 0 {
			_ = os.Remove(dir)
		}
	}
	removed, err = s.blobs.Dir().Sweep(context.Background(), grace, append([]blobs.Referrer{keeping(refs)}, more...)...)
	return views, removed, err
}

func (s *DirStore) recordPath(id string) string { return filepath.Join(s.records, id+".yaml") }

// load reads a design's record, and ErrNotFound for an id the store does not hold. It reads through
// an os.Root over the records folder, so a symlink there cannot lead a read outside it.
func (s *DirStore) load(id string) (recordYAML, error) {
	if !idPattern.MatchString(id) {
		return recordYAML{}, fmt.Errorf("%w: no stored design %s", service.ErrNotFound, service.StandaloneDesignName(id))
	}
	r, err := os.OpenRoot(s.records)
	if err != nil {
		return recordYAML{}, err
	}
	defer r.Close()
	raw, err := r.ReadFile(id + ".yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return recordYAML{}, fmt.Errorf("%w: no stored design %s", service.ErrNotFound, service.StandaloneDesignName(id))
	}
	if err != nil {
		return recordYAML{}, err
	}
	var rec recordYAML
	if err := yaml.Unmarshal(raw, &rec); err != nil {
		return recordYAML{}, fmt.Errorf("%s.yaml: %w", id, err)
	}
	return rec, nil
}

// contents checks one artifact's files and returns each file's bytes by its clean path, and the
// hashes neither blobs nor the store holds.
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
			b, ok = s.blobs.Get(f.GetSha256())
		}
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

// encodeIntent writes an intent as the YAML a descriptor's `intent:` section holds, with each key a
// proto field name, so ParseDesign binds it back to the same message. Nil writes no section.
func encodeIntent(in *configpb.DesignIntent) (yaml.Node, error) {
	if in == nil {
		return yaml.Node{}, nil
	}
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(in)
	if err != nil {
		return yaml.Node{}, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return yaml.Node{}, err
	}
	if len(doc.Content) != 1 {
		return yaml.Node{}, fmt.Errorf("intent encoded to %d YAML documents", len(doc.Content))
	}
	return *styleBlock(doc.Content[0]), nil
}

// styleBlock clears the flow style JSON parses into, so a record and its view read as block YAML.
func styleBlock(n *yaml.Node) *yaml.Node {
	n.Style &^= yaml.FlowStyle
	for _, c := range n.Content {
		styleBlock(c)
	}
	return n
}

// decodeIntent reads a record's intent section, nil when it has none.
func decodeIntent(n yaml.Node) (*configpb.DesignIntent, error) {
	if n.IsZero() || n.Tag == "!!null" {
		return nil, nil
	}
	return intent.DecodeSection(&n)
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
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

// symbolDirs names the design's symbol folders in its view, one per artifact of type
// ARTIFACT_TYPE_SYMBOLS, each the artifact's own folder (agni issue 1756). The view's design.yaml
// lists them under `symbols:`, so a read searches them for its parts.
func symbolDirs(rec recordYAML) []string {
	var out []string
	for _, a := range rec.Artifacts {
		if a.Type == webapi.ArtifactType_ARTIFACT_TYPE_SYMBOLS.String() {
			out = append(out, a.ID)
		}
	}
	return out
}

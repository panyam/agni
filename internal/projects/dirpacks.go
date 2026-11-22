package projects

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/service"
	"github.com/panyam/goapplib/storage/mounts"
)

// packViewsDir is the folder under the uploads mount's root that holds each pack's files. No design
// id can start with '@', so it never meets a design's views, and DirStore's sweep passes over it.
const packViewsDir = "@packs"

// DirPacks keeps packs as one protojson record each, `<id>.json`, in a records.Store (agni issues
// 1704, 1733). A
// pack's files are kept by hash in the uploads' BlobDir, beside the designs' files. Each version of a
// pack's files is linked into a view, `@packs/<id>/<rev>/` under the uploads mount, which is never
// changed once built. So a pack's relative paths resolve against a mount:// folder on any host that
// serves the uploads mount.
//
// Each write replaces a record whole, so a reader sees the old record or the new one. One mutex
// serialises every write.
type DirPacks struct {
	recs  records.Store
	blobs *BlobDir // nil when the server keeps no files
	views string   // the uploads mount's root, "" when the server keeps no files
	mount string
	mu    sync.Mutex
	now   func() time.Time
}

var _ service.PackStore = (*DirPacks)(nil)

// NewDirPacks returns packs kept in dir, created if absent. blobs and views are the uploads store's
// blob folder and the root it serves as the mount named mount, and both are nil and "" on a server
// with no uploads. Such a store keeps packs whose config names only mount:// URIs.
func NewDirPacks(dir string, blobs *BlobDir, views, mount string) (*DirPacks, error) {
	d, err := records.NewDir(dir)
	if err != nil {
		return nil, err
	}
	return NewPacks(d, blobs, views, mount), nil
}

// NewPacks is NewDirPacks with the records in recs, each pack at `<id>.json` (agni issue 1733).
func NewPacks(recs records.Store, blobs *BlobDir, views, mount string) *DirPacks {
	return &DirPacks{recs: recs, blobs: blobs, views: views, mount: mount}
}

func (r *DirPacks) clock() time.Time {
	if r.now != nil {
		return r.now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}

func (r *DirPacks) key(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("%w: pack id %q is not a valid id", service.ErrInvalidArgument, id)
	}
	return id + ".json", nil
}

// Pack reads one pack, or answers ErrNotFound.
func (r *DirPacks) Pack(ctx context.Context, id string) (*webapi.Pack, error) {
	return r.load(ctx, id)
}

func (r *DirPacks) load(ctx context.Context, id string) (*webapi.Pack, error) {
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: no pack %s", service.ErrNotFound, service.PackName(id))
	}
	b, err := r.recs.Get(ctx, id+".json")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: no pack %s", service.ErrNotFound, service.PackName(id))
	}
	if err != nil {
		return nil, err
	}
	out := &webapi.Pack{}
	if err := protojson.Unmarshal(b, out); err != nil {
		return nil, fmt.Errorf("%s.json: %w", id, err)
	}
	return out, nil
}

// Packs reads every pack, ordered by name.
func (r *DirPacks) Packs(ctx context.Context) ([]*webapi.Pack, error) {
	entries, err := r.recs.List(ctx, "")
	if err != nil {
		return nil, err
	}
	var out []*webapi.Pack
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name, ".json")
		if !ok || e.Dir || !idPattern.MatchString(id) {
			continue
		}
		p, err := r.Pack(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, nil
}

// CreatePack keeps a new pack.
func (r *DirPacks) CreatePack(ctx context.Context, id string, p *webapi.Pack, blobs service.BlobSource) (*webapi.Pack, error) {
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: pack id %q must be lowercase letters, digits, '.', '_' or '-'", service.ErrInvalidArgument, id)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out, b, err := r.build(ctx, id, p, blobs)
	if err != nil {
		return nil, err
	}
	// The store refuses a taken id, across servers sharing it too (agni issue 1823).
	if err := r.recs.Create(ctx, id+".json", b); errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("%w: %s", service.ErrAlreadyExists, service.PackName(id))
	} else if err != nil {
		return nil, err
	}
	return out, nil
}

// UpdatePack changes a pack as one change of its record (records.Update): when another server wrote
// the pack in between, update runs again on what it wrote (agni issue 1823).
func (r *DirPacks) UpdatePack(ctx context.Context, id string, update func(*webapi.Pack) error, blobs service.BlobSource) (*webapi.Pack, error) {
	k, err := r.key(id)
	if err != nil {
		return nil, fmt.Errorf("%w: no pack %s", service.ErrNotFound, service.PackName(id))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out *webapi.Pack
	err = records.Update(ctx, r.recs, k, func(old []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("%w: no pack %s", service.ErrNotFound, service.PackName(id))
		}
		next := &webapi.Pack{}
		if err := protojson.Unmarshal(old, next); err != nil {
			return nil, fmt.Errorf("pack %s: %w", id, err)
		}
		if err := update(next); err != nil {
			return nil, err
		}
		p, b, err := r.build(ctx, id, next, blobs)
		out = p
		return b, err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeletePack removes a pack's record. Its views stay until the next Sweep, so a snapshot read under
// it keeps answering until it is read again.
func (r *DirPacks) DeletePack(ctx context.Context, id string) error {
	k, err := r.key(id)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.recs.Delete(ctx, k); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: no pack %s", service.ErrNotFound, service.PackName(id))
	} else if err != nil {
		return err
	}
	return nil
}

// Hashes returns the hash of every file a kept pack names, so the uploads sweep keeps their blobs.
func (r *DirPacks) Hashes(ctx context.Context) (map[string]bool, error) {
	all, err := r.Packs(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, p := range all {
		for _, f := range p.GetFiles() {
			out[f.GetSha256()] = true
		}
	}
	return out, nil
}

// build stores in's files and builds their view, and returns the pack's record and its bytes for the
// caller to write. The view goes first, so a reader of the new record finds its files.
func (r *DirPacks) build(ctx context.Context, id string, in *webapi.Pack, blobs service.BlobSource) (*webapi.Pack, []byte, error) {
	p := &webapi.Pack{
		Name:   service.PackName(id),
		Title:  in.GetTitle(),
		Config: in.GetConfig(),
		Source: in.GetSource(),
	}
	if p.Title == "" {
		p.Title = id
	}
	files, err := r.keepFiles(ctx, in.GetFiles(), blobs)
	if err != nil {
		return nil, nil, err
	}
	p.Files = files
	p.Hash = service.PackHash(p)
	if len(files) > 0 {
		rev := strings.TrimPrefix(p.GetHash(), "sha256:")[:12]
		if err := r.buildView(id, rev, files); err != nil {
			return nil, nil, err
		}
		u, err := mounts.NewURI(r.mount, path.Join(packViewsDir, id, rev))
		if err != nil {
			return nil, nil, err
		}
		p.Uri = u.String()
	}
	p.UpdateTime = timestamppb.New(r.clock())
	b, err := protojson.Marshal(p)
	if err != nil {
		return nil, nil, err
	}
	return p, b, nil
}

// keepFiles checks a pack's files against their hashes and stores each as a blob. It returns the
// files sorted by path, or a *BlobsMissingError naming the contents neither blobs nor the store holds.
// A nil blobs reads the store's own, as an update that keeps the pack's files does.
func (r *DirPacks) keepFiles(ctx context.Context, files []*webapi.SnapshotFile, blobs service.BlobSource) ([]*webapi.SnapshotFile, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if r.blobs == nil {
		return nil, fmt.Errorf("%w: this server keeps no uploaded files, so a pack holds mount:// URIs alone; start it with --uploads", service.ErrFailedPrecondition)
	}
	var out []*webapi.SnapshotFile
	var missing []string
	seen := map[string]bool{}
	for _, f := range files {
		p, err := storedPath(f.GetPath())
		if err != nil {
			return nil, err
		}
		if seen[p] {
			return nil, fmt.Errorf("%w: %q is listed twice", service.ErrInvalidArgument, f.GetPath())
		}
		seen[p] = true
		var b []byte
		ok := false
		if blobs != nil {
			b, ok = blobs(ctx, f.GetSha256())
		}
		if !ok {
			b, ok = r.blobs.Get(f.GetSha256())
		}
		if !ok {
			missing = append(missing, f.GetSha256())
			continue
		}
		if hashOf(b) != f.GetSha256() || int64(len(b)) != f.GetSize() {
			return nil, fmt.Errorf("%w: the contents sent for %q do not match its hash and size", service.ErrInvalidArgument, f.GetPath())
		}
		if _, err := r.blobs.Put(b); err != nil {
			return nil, err
		}
		out = append(out, &webapi.SnapshotFile{Path: p, Sha256: f.GetSha256(), Size: f.GetSize()})
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, &service.BlobsMissingError{Hashes: slices.Compact(missing)}
	}
	slices.SortFunc(out, func(a, b *webapi.SnapshotFile) int { return strings.Compare(a.GetPath(), b.GetPath()) })
	return out, nil
}

// buildView links a pack's files into `@packs/<id>/<rev>/`, unless that view exists. It builds the
// view in a dot-folder and renames it into place, so no reader sees half of one.
func (r *DirPacks) buildView(id, rev string, files []*webapi.SnapshotFile) error {
	parent := filepath.Join(r.views, packViewsDir, id)
	dst := filepath.Join(parent, rev)
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".new-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, f := range files {
		to := filepath.Join(tmp, filepath.FromSlash(f.GetPath()))
		if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
			return err
		}
		if err := r.blobs.Link(f.GetSha256(), to); err != nil {
			return err
		}
	}
	return os.Rename(tmp, dst)
}

// Sweep removes each view under `@packs/` that no kept pack reads now (agni issue 1746). That is a
// view of an earlier version of a pack, or every view of a deleted pack. A view newer than grace
// stays, so a snapshot read under a pack a moment before a restart keeps its files until it is read
// again. A pack's blobs stay, since the uploads sweep keeps the hashes Hashes returns. It reports how
// many views it removed.
func (r *DirPacks) Sweep(ctx context.Context, grace time.Duration) (int, error) {
	if r.views == "" {
		return 0, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current := map[string]string{}
	all, err := r.Packs(ctx)
	if err != nil {
		return 0, err
	}
	for _, p := range all {
		id, _ := service.PackID(p.GetName())
		current[id] = path.Base(p.GetUri())
	}
	root, err := os.OpenRoot(r.views)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	packs, err := fs.ReadDir(root.FS(), packViewsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-grace)
	removed := 0
	for _, p := range packs {
		if !p.IsDir() || !idPattern.MatchString(p.Name()) {
			continue
		}
		dir := path.Join(packViewsDir, p.Name())
		revs, err := fs.ReadDir(root.FS(), dir)
		if err != nil {
			return removed, err
		}
		left := len(revs)
		for _, rev := range revs {
			if rev.Name() == current[p.Name()] {
				continue
			}
			fi, err := rev.Info()
			if err != nil || fi.ModTime().After(cutoff) {
				continue
			}
			if err := root.RemoveAll(path.Join(dir, rev.Name())); err != nil {
				return removed, err
			}
			removed++
			left--
		}
		if left == 0 {
			_ = root.Remove(dir)
		}
	}
	return removed, nil
}

// RebuildViews builds the view of each pack's files that the uploads folder lacks, from the blobs,
// as DirStore.RebuildViews does for designs (agni issue 1730). It returns how many it built.
func (r *DirPacks) RebuildViews(ctx context.Context) (int, error) {
	if r.blobs == nil || r.views == "" {
		return 0, nil
	}
	all, err := r.Packs(ctx)
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	built := 0
	for _, p := range all {
		if len(p.GetFiles()) == 0 {
			continue
		}
		id := strings.TrimPrefix(p.GetName(), "packs/")
		rev := strings.TrimPrefix(p.GetHash(), "sha256:")[:12]
		if _, err := os.Stat(filepath.Join(r.views, packViewsDir, id, rev)); err == nil {
			continue
		}
		if err := r.buildView(id, rev, p.GetFiles()); err != nil {
			return built, fmt.Errorf("rebuilding the view of %s: %w", p.GetName(), err)
		}
		built++
	}
	return built, nil
}

package mountset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/panyam/goapplib/storage/blobs"
	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/goapplib/storage/mounts/folder"
	"github.com/panyam/goapplib/storage/mounts/fsstore"
	"github.com/panyam/goapplib/storage/mounts/github"

	"github.com/panyam/agni/internal/archivefs"
	"github.com/panyam/agni/service"
)

// RecordsDir is the folder under --store that holds one record per runtime mount.
const RecordsDir = "mounts"

// SnapshotsDir is the folder under --uploads that holds the views of runtime mounts. A view is
// <uploads>/mounts/<name>/<commit>/, hard links to the blobs laid out as the source had them.
const SnapshotsDir = "mounts"

// BlobsDir is the folder under --uploads that holds every blob, laid out as goapplib's blobs.Dir
// keeps them.
const BlobsDir = "blobs"

// Config says what a Table serves.
type Config struct {
	// Fixed are the mounts from the flags, in the order ListMounts shows them.
	Fixed []mounts.Mount
	// Uploads is the --uploads folder, "" for none. With Records, it holds runtime mounts' blobs and
	// views.
	Uploads string
	// UploadsMount names the fixed mount that serves the uploaded designs, which a link can't name.
	UploadsMount string
	// Records is the folder of runtime mount records under --store, "" for none.
	Records string
	// GitHubAPI and GitHubRaw are GitHub's API root and raw host, set by a test or for GitHub
	// Enterprise. Empty takes the library's defaults.
	GitHubAPI, GitHubRaw string
	// Limits caps a snapshot or a link. Zero takes the library's default of 256 MB and 100,000 files.
	Limits mounts.Limits
	// ReadOnly opens the records and blobs for reading alone, as agnids does over a store agni
	// serve writes. Open then neither rewrites old records nor builds snapshot views.
	ReadOnly bool
	// Log gets a line for each old record Open rewrites or can't, and each view it can't build.
	Log io.Writer
}

// Table is a server's live mount list over goapplib's mounts registry (agni issues 1735, 1759). The
// mounts from its flags are fixed. With Records and Uploads it also serves the GitHub mounts
// CreateMount made, each a record in --store, and every process sharing the store sees them within
// a second. Every OS-backed adapter resolves through a Table, so a mount made at runtime is
// reachable at once.
type Table struct {
	reg     *mounts.Registry
	fixed   []mounts.Mount
	blobs   *blobs.Dir
	uploads string
	upMount string
}

// NewTable returns a table serving fixed and nothing else, for the CLI and for a server with no
// store.
func NewTable(fixed ...mounts.Mount) *Table {
	t, _ := Open(Config{Fixed: fixed})
	return t
}

// Open returns the table cfg describes. With Records and Uploads it first rewrites any record an
// older agni wrote (see migrate).
func Open(cfg Config) (*Table, error) {
	t := &Table{fixed: slices.Clone(cfg.Fixed), uploads: cfg.Uploads, upMount: cfg.UploadsMount}
	folderOpts := []folder.Option{folder.WithLinksInside()}
	drivers := []mounts.Driver{}
	var store mounts.RecordStore
	if cfg.Uploads != "" {
		b, err := blobs.NewDir(filepath.Join(cfg.Uploads, BlobsDir))
		if err != nil {
			return nil, fmt.Errorf("--uploads %q: %w", cfg.Uploads, err)
		}
		t.blobs = b
		folderOpts = append(folderOpts, folder.WithBlobs(b))
		ghOpts := []github.Option{github.WithViews(filepath.Join(cfg.Uploads, SnapshotsDir)), github.WithHTTPClient(&http.Client{Timeout: 5 * time.Minute}), github.WithLimits(cfg.Limits)}
		if cfg.GitHubAPI != "" {
			ghOpts = append(ghOpts, github.WithBaseURL(cfg.GitHubAPI))
		}
		if cfg.GitHubRaw != "" {
			ghOpts = append(ghOpts, github.WithRawURL(cfg.GitHubRaw))
		}
		drivers = append(drivers, github.New(b, ghOpts...))
		if cfg.Records != "" && cfg.ReadOnly {
			// A read-only reader can't make the folder, so it waits for agni serve to (laterStore).
			store = &laterStore{dir: cfg.Records}
		} else if cfg.Records != "" {
			if err := migrate(context.Background(), cfg.Records, b, cfg.Log); err != nil {
				return nil, fmt.Errorf("--store: %w", err)
			}
			// #1758 kept git blob ids in <uploads>/links/git. goapplib keeps them as blob aliases,
			// so nothing reads that folder now (agni issue 1761).
			if err := os.RemoveAll(filepath.Join(cfg.Uploads, "links")); err != nil {
				return nil, fmt.Errorf("--uploads: %w", err)
			}
			if store, err = fsstore.New(cfg.Records); err != nil {
				return nil, fmt.Errorf("--store: %w", err)
			}
		}
	}
	drivers = append(drivers, folder.New(folderOpts...))
	t.reg = mounts.NewRegistry(store, drivers, mounts.WithFixed(cfg.Fixed...), mounts.WithLimits(cfg.Limits))
	if store != nil {
		all, err := t.reg.List(context.Background())
		if err != nil {
			return nil, fmt.Errorf("--store: %w", err)
		}
		// Each snapshot's view is built now rather than on first use, so a reader sharing the
		// uploads read-only, such as agnids, finds it. A view whose blobs are lost is reported, and
		// the other mounts still serve.
		for _, m := range all {
			if cfg.ReadOnly || t.Fixed(m.Name) || m.Kind != mounts.KindGitHub {
				continue
			}
			if _, err := t.reg.HostPath(context.Background(), mounts.URI{Mount: m.Name}); err != nil && cfg.Log != nil {
				fmt.Fprintf(cfg.Log, "note: the mount %q cannot be served: %v\n", m.Name, err)
			}
		}
	}
	return t, nil
}

// Registry is the goapplib registry the table serves from.
func (t *Table) Registry() *mounts.Registry { return t.reg }

// Blobs is the blob store under --uploads, nil without one.
func (t *Table) Blobs() *blobs.Dir { return t.blobs }

// List returns every mount the table serves: the fixed ones in configuration order, then the
// runtime ones by name. A store that fails after a good read keeps the last good list.
func (t *Table) List() []mounts.Mount {
	out := slices.Clone(t.fixed)
	all, _ := t.reg.List(context.Background())
	for _, m := range all {
		if !t.Fixed(m.Name) {
			out = append(out, m)
		}
	}
	return out
}

// Find returns the mount named name.
func (t *Table) Find(name string) (mounts.Mount, bool) { return Find(t.List(), name) }

// Fixed reports whether name is a mount from the flags.
func (t *Table) Fixed(name string) bool {
	_, ok := Find(t.fixed, name)
	return ok
}

// Resolve maps uri to a host path inside its mount, for the OS-backed adapters. An unknown mount is
// service.ErrNotFound and a path that leaves its mount is service.ErrInvalidPath. A path may run
// into an archive, which archivefs reads as a folder (agni issue 1591). The library resolves the
// archive file, and the rest of the path is joined on for archivefs to read.
func (t *Table) Resolve(uri mounts.URI) (string, error) {
	ctx := context.Background()
	p, err := t.reg.HostPath(ctx, uri)
	if err == nil {
		return p, nil
	}
	if errors.Is(err, mounts.ErrInvalidPath) {
		parts := strings.Split(uri.Path, "/")
		for i := len(parts) - 1; i > 0; i-- {
			if !archivefs.Candidate(parts[i-1]) {
				continue
			}
			head, herr := t.reg.HostPath(ctx, mounts.URI{Mount: uri.Mount, Path: strings.Join(parts[:i], "/")})
			if herr == nil {
				return filepath.Join(head, filepath.FromSlash(strings.Join(parts[i:], "/"))), nil
			}
		}
	}
	return "", Err(err)
}

// Root returns the host folder m serves from: a folder mount's own path, or a GitHub snapshot's
// view, built on first use. It returns "" for a mount with no folder here.
func (t *Table) Root(m mounts.Mount) string {
	if m.Kind == mounts.KindFolder {
		return m.Source
	}
	p, err := t.reg.HostPath(context.Background(), mounts.URI{Mount: m.Name})
	if err != nil {
		return ""
	}
	return p
}

// noMaker is why a table with no store refuses CreateMount and DeleteMount.
const noMaker = "this server makes no mounts at runtime; start it with --store and --uploads"

// CreateMount snapshots a github://<owner>/<repo> source at ref and serves it as name. A file://
// source is refused, because a client must not choose which folder on the server's disk is served.
func (t *Table) CreateMount(ctx context.Context, name, source, ref string) (service.MountInfo, error) {
	if t.blobs == nil {
		return service.MountInfo{}, fmt.Errorf("%w: %s", service.ErrFailedPrecondition, noMaker)
	}
	if scheme, _, ok := strings.Cut(source, "://"); !ok || scheme != "github" {
		if ok && scheme == "file" {
			return service.MountInfo{}, fmt.Errorf("%w: a folder on the server is mounted with --mount, not at runtime", service.ErrInvalidArgument)
		}
		return service.MountInfo{}, fmt.Errorf("%w: source %q: this build makes github://<owner>/<repo> mounts", service.ErrInvalidArgument, source)
	}
	if t.Fixed(name) {
		return service.MountInfo{}, fmt.Errorf("%w: a mount named %q is already served", service.ErrAlreadyExists, name)
	}
	m, err := t.reg.Create(ctx, mounts.Mount{Name: name, Kind: mounts.KindGitHub, Source: strings.TrimSuffix(source, "/"), Ref: ref, Mode: mounts.ModeSnapshot})
	if err != nil {
		return service.MountInfo{}, Err(err)
	}
	// The view is built now, so a reader sharing the uploads read-only, such as agnids, finds it.
	if _, err := t.reg.HostPath(ctx, mounts.URI{Mount: name}); err != nil {
		return service.MountInfo{}, Err(err)
	}
	return t.Info(m), nil
}

// DeleteMount forgets a runtime mount and removes its views. Its blobs stay until a sweep finds
// nothing naming them.
func (t *Table) DeleteMount(ctx context.Context, name string) error {
	if t.blobs == nil {
		return fmt.Errorf("%w: %s", service.ErrFailedPrecondition, noMaker)
	}
	if err := t.reg.Delete(ctx, name); err != nil {
		return Err(err)
	}
	if err := os.RemoveAll(filepath.Join(t.uploads, SnapshotsDir, name)); err != nil {
		return fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	return nil
}

var _ service.MountMaker = (*Table)(nil)

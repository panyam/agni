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
	"github.com/panyam/goapplib/storage/connections"
	connstore "github.com/panyam/goapplib/storage/connections/fsstore"
	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/goapplib/storage/mounts/folder"
	"github.com/panyam/goapplib/storage/mounts/fsstore"
	"github.com/panyam/goapplib/storage/mounts/github"
	"github.com/panyam/oneauth/outbound"
	"github.com/panyam/oneauth/stores/fs"
	"golang.org/x/oauth2"
	ghendpoint "golang.org/x/oauth2/github"

	"github.com/panyam/agni/internal/archivefs"
	"github.com/panyam/agni/service"
)

// RecordsDir is the folder under --store that holds one record per runtime mount.
const RecordsDir = "mounts"

// ConnectionsDir is the folder under --store that holds the connections private sources are read
// through (agni issue 1724). agni serve writes it, and agnids reads it.
const ConnectionsDir = "connections"

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
	// Connections is the folder of connection records under --store, "" for none. A runtime mount
	// naming a connection reads its source with that connection's token (agni issue 1724).
	Connections string
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
	// UserTokens is the folder under --store holding people's own tokens, sealed under
	// ConnectionsKey, "" for none (agni issue 1728).
	UserTokens string
	// ConnectionsKey is where the key that seals people's tokens is, as a secret reference such as
	// env:NAME. It holds 64 hex characters. Empty keeps no person's token.
	ConnectionsKey string
	// GitHubApp is the GitHub App's OAuth client the server signs people in through (agni issue
	// 1772). goapplib refreshes a signed-in person's token through it, and revokes it on a
	// disconnect. Nil keeps pasted tokens alone, which need neither.
	GitHubApp *oauth2.Config
	// HidePrivate leaves out every mount that reads through a connection, from List and from
	// Resolve, for a server that asks who its callers are but can't check their connections. agnids
	// with --auth and no --connections-key is one (agni issue 1728). With ReadOnly, Connections,
	// UserTokens and ConnectionsKey, a reader checks callers through CanRead instead (agni issue
	// 1778).
	HidePrivate bool
}

// Table is a server's live mount list over goapplib's mounts registry (agni issues 1735, 1759). The
// mounts from its flags are fixed. With Records and Uploads it also serves the GitHub mounts
// CreateMount made, each a record in --store, and every process sharing the store sees them within
// a second. Every OS-backed adapter resolves through a Table, so a mount made at runtime is
// reachable at once.
type Table struct {
	reg     *mounts.Registry
	conns   connections.Store
	fixed   []mounts.Mount
	blobs   *blobs.Dir
	uploads string
	upMount string
	// svc gives connections' tokens and asks GitHub whether a person can read a repo, and users
	// holds people's sealed tokens. Both are nil without Connections.
	svc   *connections.Service
	users outbound.UserTokenStore
	// hidePrivate is Config.HidePrivate.
	hidePrivate bool
	// gh is the GitHub driver, nil without Uploads. MountsOf asks it whether two sources match.
	gh *github.Driver
	// githubAPI is Config.GitHubAPI, the API a signed-in person's connection reads through.
	githubAPI string
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
	t := &Table{fixed: slices.Clone(cfg.Fixed), uploads: cfg.Uploads, upMount: cfg.UploadsMount, hidePrivate: cfg.HidePrivate}
	folderOpts := []folder.Option{folder.WithLinksInside()}
	drivers := []mounts.Driver{}
	var gh *github.Driver
	var store mounts.RecordStore
	if cfg.Uploads != "" {
		var b blobs.Linker
		if cfg.ReadOnly {
			// A read-only reader can't make the folder either, so it waits for agni serve to
			// (laterBlobs, agni issue 1767).
			b = &laterBlobs{root: filepath.Join(cfg.Uploads, BlobsDir)}
		} else {
			d, err := blobs.NewDir(filepath.Join(cfg.Uploads, BlobsDir))
			if err != nil {
				return nil, fmt.Errorf("--uploads %q: %w", cfg.Uploads, err)
			}
			t.blobs, b = d, d
		}
		folderOpts = append(folderOpts, folder.WithBlobs(b))
		ghOpts := []github.Option{github.WithViews(filepath.Join(cfg.Uploads, SnapshotsDir)), github.WithHTTPClient(&http.Client{Timeout: 5 * time.Minute}), github.WithLimits(cfg.Limits)}
		if cfg.GitHubAPI != "" {
			ghOpts = append(ghOpts, github.WithBaseURL(cfg.GitHubAPI))
		}
		if cfg.GitHubRaw != "" {
			ghOpts = append(ghOpts, github.WithRawURL(cfg.GitHubRaw))
		}
		gh = github.New(b, ghOpts...)
		drivers = append(drivers, gh)
		if cfg.Records != "" && cfg.ReadOnly {
			// A read-only reader can't make the folder, so it waits for agni serve to (laterStore).
			store = &laterStore{dir: cfg.Records}
		} else if cfg.Records != "" {
			if err := migrate(context.Background(), cfg.Records, t.blobs, cfg.Log); err != nil {
				return nil, fmt.Errorf("--store: %w", err)
			}
			// #1758 kept git blob ids in <uploads>/links/git. goapplib keeps them as blob aliases,
			// so nothing reads that folder now (agni issue 1761).
			if err := os.RemoveAll(filepath.Join(cfg.Uploads, "links")); err != nil {
				return nil, fmt.Errorf("--uploads: %w", err)
			}
			var err error
			if store, err = fsstore.New(cfg.Records); err != nil {
				return nil, fmt.Errorf("--store: %w", err)
			}
		}
	}
	drivers = append(drivers, folder.New(folderOpts...))
	opts := []mounts.Option{mounts.WithFixed(cfg.Fixed...), mounts.WithLimits(cfg.Limits)}
	if cfg.Connections != "" && cfg.ReadOnly && cfg.ConnectionsKey != "" && cfg.UserTokens != "" {
		// A reader answers CanRead through people's own connections, and writes neither the records
		// nor the tokens (see readonly.go). It hands no mount a token, since it builds no snapshot.
		cs, err := connstore.New(cfg.Connections)
		if err != nil {
			return nil, fmt.Errorf("--store: %w", err)
		}
		t.conns = readOnlyConnections{cs}
		t.users = readOnlyTokens{fs.NewFSUserTokenStore(cfg.UserTokens)}
		copts := []connections.Option{connections.WithUserTokens(t.users, cfg.ConnectionsKey, noRefresh)}
		if gh != nil {
			copts = append(copts, connections.WithReadCheck(gh.CanRead))
		}
		t.svc = connections.NewService(t.conns, copts...)
	}
	if cfg.Connections != "" && !cfg.ReadOnly {
		cs, err := connstore.New(cfg.Connections)
		if err != nil {
			return nil, fmt.Errorf("--store: %w", err)
		}
		t.conns = cs
		var copts []connections.Option
		if gh != nil {
			copts = append(copts, connections.WithReadCheck(gh.CanRead))
		}
		if cfg.ConnectionsKey != "" && cfg.UserTokens != "" {
			t.users = fs.NewFSUserTokenStore(cfg.UserTokens)
			app := &oauth2.Config{Endpoint: ghendpoint.Endpoint}
			if cfg.GitHubApp != nil {
				app = cfg.GitHubApp
			}
			copts = append(copts, connections.WithUserTokens(t.users, cfg.ConnectionsKey, app))
		}
		t.svc = connections.NewService(cs, copts...)
		opts = append(opts, mounts.WithCredentials(t.svc.TokenSource))
	}
	t.gh = gh
	t.githubAPI = cfg.GitHubAPI
	t.reg = mounts.NewRegistry(store, drivers, opts...)
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
		if !t.Fixed(m.Name) && !(t.hidePrivate && m.Connection != "") {
			out = append(out, m)
		}
	}
	return out
}

// MountsOf names every mount whose source is source, for service.MountsOf (agni issue 1726). It
// counts the mounts a viewer can't see too, since a push moves every design that follows it. GitHub's
// owner and repo names ignore case.
func (t *Table) MountsOf(ctx context.Context, source string) ([]string, error) {
	all, err := t.reg.List(ctx)
	if err != nil {
		return nil, Err(err)
	}
	var out []string
	for _, m := range all {
		same := m.Source == source
		if m.Kind == mounts.KindGitHub && t.gh != nil {
			same = t.gh.SameSource(m.Source, source)
		}
		if same {
			out = append(out, m.Name)
		}
	}
	return out, nil
}

var _ service.MountsOf = (*Table)(nil).MountsOf

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
	if t.hidePrivate && t.privateMount(uri.Mount) {
		return "", fmt.Errorf("%w: the mount %q reads a private repo, which this server can't check your access to", service.ErrPermissionDenied, uri.Mount)
	}
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
func (t *Table) CreateMount(ctx context.Context, name, source, ref, connection string) (service.MountInfo, error) {
	ctx = viewing(ctx)
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
	if connection != "" {
		if t.conns == nil {
			return service.MountInfo{}, fmt.Errorf("%w: this server keeps no connections", service.ErrFailedPrecondition)
		}
		// A mount reads through a server connection. A person's connection reads for its owner
		// alone, and a mount is everyone's (agni issue 1778).
		if _, err := t.conns.GetConnection(ctx, "", connection); err != nil {
			return service.MountInfo{}, service.ConnectionErr(err)
		}
	}
	m, err := t.reg.Create(ctx, mounts.Mount{Name: name, Kind: mounts.KindGitHub, Source: strings.TrimSuffix(source, "/"), Ref: ref, Mode: mounts.ModeSnapshot, Connection: connection})
	if err != nil {
		return service.MountInfo{}, service.ConnectionErr(Err(err))
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

// Connections is the store of connection records, nil for a table with none.
func (t *Table) Connections() connections.Store { return t.conns }

// MountsUsing names the runtime mounts that read through the connection id, sorted, so a connection
// in use isn't deleted under them.
func (t *Table) MountsUsing(_ context.Context, id string) ([]string, error) {
	var out []string
	for _, m := range t.List() {
		if m.Connection == id {
			out = append(out, m.Name)
		}
	}
	return out, nil
}

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

	"github.com/panyam/agni/internal/records"
	"github.com/panyam/goapplib/storage/drivers/gcs"
	"github.com/panyam/goapplib/storage/drivers/msgraph"
	"github.com/panyam/goapplib/storage/drivers/s3"

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
	// Stores, when set, hold the runtime mount records, the connections and people's tokens in place
	// of the Records, Connections and UserTokens folders, as `--store sqlite://` does (agni issue
	// 1733). A records folder that an older agni wrote is migrated, and a Stores is not, since no
	// older agni wrote one.
	Stores *Stores
	// HidePrivate leaves out every mount that reads through a connection, from List and from
	// Resolve, for a server that asks who its callers are but can't check their connections. agnids
	// with --auth and no --connections-key is one (agni issue 1728). With ReadOnly, Connections,
	// UserTokens and ConnectionsKey, a reader checks callers through CanRead instead (agni issue
	// 1778).
	HidePrivate bool
	// S3Endpoint, GCSEndpoint and GraphAPI point the S3, Cloud Storage and Microsoft Graph drivers
	// at another host: an S3-compatible store such as MinIO, a national cloud, or a test's fake.
	// Empty takes each service's public endpoint (agni issues 1729, 1730).
	S3Endpoint, GCSEndpoint, GraphAPI string
	// Blobs is the blob store under Uploads, when the server opened one itself, such as a bucket
	// behind the uploads folder for an s3:// or gs:// --uploads (agni issue 1730). Nil opens
	// <Uploads>/blobs.
	Blobs blobs.Linker
	// Subscriptions keeps the Graph change-notification subscriptions CreateWebhook makes on
	// SharePoint mounts, one record per mount, so RenewSubscriptions finds them after a restart.
	// Nil keeps none, and CreateWebhook refuses a SharePoint mount.
	Subscriptions records.Store
}

// Stores are the record stores a Table reads and writes in place of folders under --store. A nil
// field keeps none of that kind, as an empty folder name does.
type Stores struct {
	Mounts      mounts.RecordStore
	Connections connections.Store
	UserTokens  outbound.UserTokenStore
}

// Table is a server's live mount list over goapplib's mounts registry (agni issues 1735, 1759). The
// mounts from its flags are fixed. With Records and Uploads it also serves the GitHub, S3, Cloud
// Storage and SharePoint mounts CreateMount made (agni issues 1729, 1730), each a record in --store, and every process sharing the store sees them within
// a second. Every OS-backed adapter resolves through a Table, so a mount made at runtime is
// reachable at once.
type Table struct {
	reg     *mounts.Registry
	conns   connections.Store
	fixed   []mounts.Mount
	blobs   blobs.Linker
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
	// graph is the Microsoft Graph driver, nil without Uploads. MountsOf asks it whether a
	// notification's drive holds a mount, and CreateWebhook subscribes through it.
	graph *msgraph.Driver
	// subs is Config.Subscriptions.
	subs records.Store
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
		} else if cfg.Blobs != nil {
			// The server opened the blob store, such as a bucket behind the uploads folder (agni issue
			// 1730).
			t.blobs, b = cfg.Blobs, cfg.Blobs
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
		// A bucket or drive snapshot downloads every listed file into the blobs and gets a view
		// beside the GitHub snapshots', so the engine reads it through a folder (agni issues 1729,
		// 1730). Mount names are unique, so their views never meet.
		views := filepath.Join(cfg.Uploads, SnapshotsDir)
		s3Opts := []s3.Option{s3.WithViews(views), s3.WithLimits(cfg.Limits)}
		if cfg.S3Endpoint != "" {
			s3Opts = append(s3Opts, s3.WithEndpoint(cfg.S3Endpoint))
		}
		gcsOpts := []gcs.Option{gcs.WithViews(views), gcs.WithLimits(cfg.Limits)}
		if cfg.GCSEndpoint != "" {
			gcsOpts = append(gcsOpts, gcs.WithEndpoint(cfg.GCSEndpoint))
		}
		graphOpts := []msgraph.Option{msgraph.WithViews(views), msgraph.WithLimits(cfg.Limits), msgraph.WithHTTPClient(&http.Client{Timeout: 5 * time.Minute})}
		if cfg.GraphAPI != "" {
			graphOpts = append(graphOpts, msgraph.WithBaseURL(cfg.GraphAPI))
		}
		t.graph = msgraph.New(b, graphOpts...)
		drivers = append(drivers, s3.New(b, s3Opts...), gcs.New(b, gcsOpts...), t.graph)
		if cfg.Stores != nil {
			store = cfg.Stores.Mounts
		} else if cfg.Records != "" && cfg.ReadOnly {
			// A read-only reader can't make the folder, so it waits for agni serve to (laterStore).
			store = &laterStore{dir: cfg.Records}
		} else if cfg.Records != "" {
			// Records an older agni wrote sit beside a folder of blobs, never a bucket's.
			if d, ok := t.blobs.(*blobs.Dir); ok {
				if err := migrate(context.Background(), cfg.Records, d, cfg.Log); err != nil {
					return nil, fmt.Errorf("--store: %w", err)
				}
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
	var cs connections.Store
	var users outbound.UserTokenStore
	switch {
	case cfg.Stores != nil:
		cs, users = cfg.Stores.Connections, cfg.Stores.UserTokens
	case cfg.Connections != "":
		var err error
		if cs, err = connstore.New(cfg.Connections); err != nil {
			return nil, fmt.Errorf("--store: %w", err)
		}
		if cfg.UserTokens != "" {
			users = fs.NewFSUserTokenStore(cfg.UserTokens)
		}
	}
	if cs != nil && cfg.ReadOnly && cfg.ConnectionsKey != "" && users != nil {
		// A reader answers CanRead through people's own connections, and writes neither the records
		// nor the tokens (see readonly.go). It hands no mount a token, since it builds no snapshot.
		t.conns = readOnlyConnections{cs}
		t.users = readOnlyTokens{users}
		copts := []connections.Option{connections.WithUserTokens(t.users, cfg.ConnectionsKey, noRefresh)}
		if gh != nil {
			copts = append(copts, connections.WithReadCheckFor("github", gh.CanRead, connections.KindGitHubUser, connections.KindGitHubToken))
		}
		t.svc = connections.NewService(t.conns, copts...)
	}
	if cs != nil && !cfg.ReadOnly {
		t.conns = cs
		var copts []connections.Option
		if gh != nil {
			copts = append(copts, connections.WithReadCheckFor("github", gh.CanRead, connections.KindGitHubUser, connections.KindGitHubToken))
		}
		if cfg.ConnectionsKey != "" && users != nil {
			t.users = users
			app := &oauth2.Config{Endpoint: ghendpoint.Endpoint}
			if cfg.GitHubApp != nil {
				app = cfg.GitHubApp
			}
			copts = append(copts, connections.WithUserTokens(t.users, cfg.ConnectionsKey, app))
		}
		t.svc = connections.NewService(cs, copts...)
		opts = append(opts, mounts.WithCredentials(t.svc.Credentials))
	}
	t.gh = gh
	t.subs = cfg.Subscriptions
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
			if cfg.ReadOnly || t.Fixed(m.Name) || !snapshotKind(m.Kind) {
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
func (t *Table) Blobs() blobs.Linker { return t.blobs }

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
		switch {
		case m.Kind == mounts.KindGitHub && t.gh != nil:
			same = t.gh.SameSource(m.Source, source)
		case m.Kind == mounts.KindMSGraph && t.graph != nil:
			// Graph notifies per drive, so a change names the drive's root and holds every mount in
			// that drive.
			same = t.graph.ContainsSource(m.Source, source)
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

// sourceKinds is the mount kind each runtime source scheme makes (agni issues 1729, 1730).
var sourceKinds = map[string]mounts.Kind{
	"github":  mounts.KindGitHub,
	"s3":      mounts.KindS3,
	"gs":      mounts.KindGCS,
	"msgraph": mounts.KindMSGraph,
}

// snapshotKind reports whether a mount of kind k is a snapshot CreateMount makes, with a view under
// --uploads.
func snapshotKind(k mounts.Kind) bool {
	for _, sk := range sourceKinds {
		if k == sk {
			return true
		}
	}
	return false
}

// CreateMount snapshots a source and serves it as name: a github://<owner>/<repo> at ref, or an
// s3://, gs:// or msgraph:// folder with each file pinned at the version it has now (agni issues
// 1729, 1730). A bucket or a drive has no refs, so ref must be empty for one, and a Graph source
// always reads through a connection. A file:// source is refused, because a client must not choose
// which folder on the server's disk is served.
func (t *Table) CreateMount(ctx context.Context, name, source, ref, connection string) (service.MountInfo, error) {
	ctx = viewing(ctx)
	if t.blobs == nil {
		return service.MountInfo{}, fmt.Errorf("%w: %s", service.ErrFailedPrecondition, noMaker)
	}
	scheme, _, _ := strings.Cut(source, "://")
	kind, ok := sourceKinds[scheme]
	switch {
	case scheme == "file":
		return service.MountInfo{}, fmt.Errorf("%w: a folder on the server is mounted with --mount, not at runtime", service.ErrInvalidArgument)
	case !ok:
		return service.MountInfo{}, fmt.Errorf("%w: source %q: this build makes github://<owner>/<repo>, s3://<bucket>/<prefix>, gs://<bucket>/<prefix> and msgraph://sites/<site>/drives/<drive>/root/<folder> mounts", service.ErrInvalidArgument, source)
	case kind != mounts.KindGitHub && ref != "":
		return service.MountInfo{}, fmt.Errorf("%w: a %s:// source has no refs; leave ref empty, and the snapshot pins each file's version", service.ErrInvalidArgument, scheme)
	case kind == mounts.KindMSGraph && connection == "":
		return service.MountInfo{}, fmt.Errorf("%w: Microsoft Graph reads through a connection; make a CONNECTION_KIND_ENTRA one and name it", service.ErrInvalidArgument)
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
	m, err := t.reg.Create(ctx, mounts.Mount{Name: name, Kind: kind, Source: strings.TrimSuffix(source, "/"), Ref: ref, Mode: mounts.ModeSnapshot, Connection: connection})
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
	m, getErr := t.reg.Get(ctx, name)
	if err := t.reg.Delete(ctx, name); err != nil {
		return Err(err)
	}
	// A SharePoint mount's subscription goes with it (agni issue 1729).
	if getErr == nil && m.Kind == mounts.KindMSGraph {
		t.unsubscribe(ctx, m)
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

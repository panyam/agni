package storeurl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/goapplib/storage/blobs"
	gcsdrv "github.com/panyam/goapplib/storage/drivers/gcs"
	"github.com/panyam/goapplib/storage/drivers/gcs/gcsblobs"
	s3drv "github.com/panyam/goapplib/storage/drivers/s3"
	"github.com/panyam/goapplib/storage/drivers/s3/s3blobs"
	"github.com/panyam/goapplib/storage/mounts"
	"golang.org/x/oauth2/google"

	"cloud.google.com/go/datastore"

	gaestore "github.com/panyam/goapplib/storage/backends/gae"
	gormstore "github.com/panyam/goapplib/storage/backends/gorm"
	oagae "github.com/panyam/oneauth/stores/gae"
	oagorm "github.com/panyam/oneauth/stores/gorm"
	"gorm.io/gorm"

	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/gaerecords"
	"github.com/panyam/agni/internal/records/sqlrecords"
	"github.com/panyam/agni/mountset"
)

// Store is an opened --store. A file:// store keeps each record as a file under Dir, in the layout
// every earlier agni wrote. A sqlite:// or postgres:// store keeps every record in DB through GORM:
// agni's own records in the agni_records table, runtime mounts and connections in goapplib's
// tables, and tokens in oneauth's. A datastore:// store keeps the same records as Datastore
// entities, in goapplib's, oneauth's and agni's own kinds, under Namespace. None holds file bytes,
// which live in mounts and --uploads.
type Store struct {
	// URL is the --store value it was opened from.
	URL string
	// Dir is a file:// store's folder, "" for the others.
	Dir string
	// DB is a sqlite:// or postgres:// store's database, nil for the others.
	DB *gorm.DB
	// Datastore is a datastore:// store's client, nil for the others, and Namespace the namespace
	// its entities sit in, "" for the default one.
	Datastore *datastore.Client
	Namespace string
	// Records holds agni's own records, each kind under a folder of its own, such as "designs" or
	// "reviews" (Sub).
	Records records.Store
	// UploadsRemote is an s3:// or gs:// --uploads, which holds every uploaded and snapshotted file,
	// and "" for a file:// one (agni issue 1730). The uploads folder Open returns is then the local
	// cache in front of it, which also holds the views.
	UploadsRemote string
	uploadsDir    string
	opts          options
}

// options are Open's optional settings.
type options struct {
	uploadsCache            string
	s3Endpoint, gcsEndpoint string
	readOnly                bool
}

// Option sets one of Open's optional settings.
type Option func(*options)

// WithUploadsCache names the local folder in front of an s3:// or gs:// --uploads, which keeps
// copies of the files and every view (--uploads-cache). Empty takes a folder named for the URL under
// the user's cache folder.
func WithUploadsCache(dir string) Option { return func(o *options) { o.uploadsCache = dir } }

// ReadOnly opens a file:// store without making its folder, for a server that only reads another's
// store, such as agnids (agni issue 1847). A folder that isn't there yet reads as empty.
func ReadOnly() Option { return func(o *options) { o.readOnly = true } }

// WithEndpoints points an s3:// or gs:// --uploads at another S3 or Cloud Storage API, as
// --s3-endpoint and --gcs-endpoint do for mounts. Empty takes each service's own.
func WithEndpoints(s3Endpoint, gcsEndpoint string) Option {
	return func(o *options) { o.s3Endpoint, o.gcsEndpoint = s3Endpoint, gcsEndpoint }
}

// uploadsLocal turns an s3:// or gs:// --uploads into the file:// URL of its local cache, and
// returns the remote. A file:// or empty value comes back as it is, with no remote.
func uploadsLocal(uploads string, o options) (local, remote string, err error) {
	scheme, rest, _ := strings.Cut(uploads, "://")
	if scheme != "s3" && scheme != "gs" {
		return uploads, "", nil
	}
	if bucket, _, _ := strings.Cut(rest, "/"); bucket == "" || strings.ContainsAny(rest, "?#") {
		return "", "", fmt.Errorf("--uploads %q names no bucket; write it as %s://<bucket>/<prefix>", uploads, scheme)
	}
	dir := o.uploadsCache
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", "", fmt.Errorf("--uploads %q: no cache folder to keep its copies in; name one with --uploads-cache: %w", uploads, err)
		}
		sum := sha256.Sum256([]byte(uploads))
		dir = filepath.Join(base, "agni", "uploads", hex.EncodeToString(sum[:6]))
	} else if strings.Contains(dir, "://") {
		if dir, err = folderOf("uploads-cache", dir, map[string]string{"file": ""}); err != nil {
			return "", "", err
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	return fileURL(abs), strings.TrimSuffix(uploads, "/"), nil
}

// Open opens --store and reads --uploads. An empty store returns nil, and --uploads alone is
// refused, since uploads need a store to record what they hold. An s3:// or gs:// --uploads returns
// its local cache as the uploads folder, and UploadBlobs reaches the bucket (agni issue 1730). A file:// folder and a sqlite://
// file are created if absent. A postgres:// database and a Datastore project must exist, and Open
// creates the tables it needs in them. ReadOnly leaves a missing file:// folder alone.
func Open(store, uploads string, opts ...Option) (*Store, string, error) {
	var o options
	for _, f := range opts {
		f(&o)
	}
	uploads, remote, err := uploadsLocal(uploads, o)
	if err != nil {
		return nil, "", err
	}
	st, up, err := open(store, uploads, o)
	if st != nil {
		st.UploadsRemote, st.uploadsDir, st.opts = remote, up, o
	}
	return st, up, err
}

func open(store, uploads string, o options) (*Store, string, error) {
	scheme, _, _ := strings.Cut(store, "://")
	switch scheme {
	case "sqlite", "postgres", "postgresql", "datastore":
	default:
		dir, up, err := folders(store, uploads)
		if err != nil || dir == "" {
			return nil, up, err
		}
		newDir := records.NewDir
		if o.readOnly {
			newDir = records.OpenDir
		}
		recs, err := newDir(dir)
		if err != nil {
			return nil, "", fmt.Errorf("--store %q: %w", store, err)
		}
		return &Store{URL: store, Dir: dir, Records: recs}, up, nil
	}
	up, err := folderOf("uploads", uploads, uploadsSchemes)
	if err != nil {
		return nil, "", err
	}
	if scheme == "datastore" {
		project, ns, err := datastoreURL(store)
		if err != nil {
			return nil, "", err
		}
		client, err := datastore.NewClient(context.Background(), project)
		if err != nil {
			return nil, "", fmt.Errorf("--store %q: %w", store, err)
		}
		return &Store{URL: store, Datastore: client, Namespace: ns, Records: gaerecords.New(client, ns)}, up, nil
	}
	var db *gorm.DB
	if scheme == "sqlite" {
		path, err := sqlitePath(store)
		if err != nil {
			return nil, "", err
		}
		if db, err = sqlrecords.OpenSQLite(path); err != nil {
			return nil, "", fmt.Errorf("--store %q: %w", store, err)
		}
	} else {
		if err := checkPostgres(store); err != nil {
			return nil, "", err
		}
		if db, err = sqlrecords.OpenPostgres(store); err != nil {
			return nil, "", fmt.Errorf("--store %s: %w", redact(store), err)
		}
	}
	recs, err := sqlrecords.NewSQL(db)
	if err != nil {
		closeDB(db)
		return nil, "", fmt.Errorf("--store %q: %w", store, err)
	}
	return &Store{URL: store, DB: db, Records: recs}, up, nil
}

// checkPostgres refuses a postgres:// URL with no host or with a password in it. A password on the
// command line shows in ps and in shell history, so it comes from PGPASSWORD or a passfile
// (PGPASSFILE, or ~/.pgpass), which pgx reads as libpq does. Query options such as sslmode pass
// through to pgx.
func checkPostgres(store string) error {
	u, err := url.Parse(store)
	if err != nil {
		return fmt.Errorf("--store %s: %w", redact(store), err)
	}
	if _, has := u.User.Password(); has {
		return fmt.Errorf("--store %s names a password; put it in PGPASSWORD or a passfile (PGPASSFILE), so it doesn't show in ps or shell history", redact(store))
	}
	if u.Host == "" || strings.Trim(u.Path, "/") == "" {
		return fmt.Errorf("--store %q names no host or database; write it as postgres://agni@db.internal:5432/agni", store)
	}
	return nil
}

// redact is store with any password in it replaced, for an error message.
func redact(store string) string {
	if u, err := url.Parse(store); err == nil {
		return u.Redacted()
	}
	return "(an unreadable postgres URL)"
}

// datastoreURL reads datastore://<project>?namespace=<ns>. With no project it takes the one the
// environment names (DATASTORE_PROJECT_ID, or the credentials' project), as the Datastore client
// does. namespace is the only option, and "" keeps the default namespace.
func datastoreURL(store string) (project, namespace string, err error) {
	u, err := url.Parse(store)
	if err != nil {
		return "", "", fmt.Errorf("--store %q: %w", store, err)
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") {
		return "", "", fmt.Errorf("--store %q: write a Datastore store as datastore://<project>?namespace=<ns>", store)
	}
	for k := range u.Query() {
		if k != "namespace" {
			return "", "", fmt.Errorf("--store %q: a datastore:// URL takes namespace and nothing else", store)
		}
	}
	project = u.Host
	if project == "" {
		project = datastore.DetectProjectID
	}
	return project, u.Query().Get("namespace"), nil
}

// sqlitePath reads a sqlite:// URL, which names a database file by its absolute path:
// sqlite:///srv/agni.db. A relative path, a host, or a query is refused, since the server picks its
// own connection settings (sqlrecords.OpenSQLite).
func sqlitePath(store string) (string, error) {
	rest := strings.TrimPrefix(store, "sqlite://")
	if strings.ContainsAny(rest, "?#") {
		return "", fmt.Errorf("--store %q: a sqlite:// URL names a database file and takes no options; write it as sqlite:///srv/agni.db", store)
	}
	if !strings.HasPrefix(rest, "/") {
		return "", fmt.Errorf("--store %q names no absolute file; write it as sqlite:///srv/agni.db", store)
	}
	p := rest
	// A Windows path names its drive after the slash, sqlite:///C:/agni.db.
	if len(p) >= 3 && p[2] == ':' && filepath.VolumeName(p[1:]) != "" {
		p = p[1:]
	}
	p = filepath.Clean(filepath.FromSlash(p))
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return "", fmt.Errorf("--store %q names a folder; a sqlite:// URL names the database file in it, such as sqlite:///srv/agni/agni.db", store)
	}
	return p, nil
}

// Sub is the records of one kind, under the folder name.
func (s *Store) Sub(name string) records.Store { return records.Sub(s.Records, name) }

// Tokens is the store of the tokens `agni serve --auth tokens` checks.
func (s *Store) Tokens() (authgate.Keys, error) {
	if s.Datastore != nil {
		return authgate.GAEStore(s.Datastore, s.Namespace), nil
	}
	if s.DB != nil {
		return authgate.SQLStore(s.DB)
	}
	return authgate.Store(s.Dir), nil
}

// Accounts is the store of the local accounts `agni serve --local-accounts` signs people in with
// (agni issue 1815). It sits beside Tokens in the same --store.
func (s *Store) Accounts() (authgate.Accounts, error) {
	if s.Datastore != nil {
		return authgate.GAEAccounts(s.Datastore, s.Namespace), nil
	}
	if s.DB != nil {
		return authgate.SQLAccounts(s.DB)
	}
	return authgate.AccountsStore(s.Dir), nil
}

// TableStores returns the stores a mountset.Table keeps runtime mounts, connections and people's
// tokens in: goapplib's and oneauth's GORM stores for a database, and their Datastore stores for
// Datastore. A file:// store returns nil, and the table keeps them in folders under Dir as it
// always has.
func (s *Store) TableStores() (*mountset.Stores, error) {
	if s.Datastore != nil {
		return &mountset.Stores{
			Mounts:      gaestore.NewMountStore(s.Datastore, s.Namespace),
			Connections: gaestore.NewConnectionStore(s.Datastore, s.Namespace),
			UserTokens:  oagae.NewUserTokenStore(s.Datastore, s.Namespace),
		}, nil
	}
	if s.DB == nil {
		return nil, nil
	}
	ms, err := gormstore.NewMountStore(s.DB)
	if err != nil {
		return nil, fmt.Errorf("--store %q: %w", s.URL, err)
	}
	cs, err := gormstore.NewConnectionStore(s.DB)
	if err != nil {
		return nil, fmt.Errorf("--store %q: %w", s.URL, err)
	}
	if err := s.DB.AutoMigrate(&oagorm.UserTokenModel{}); err != nil {
		return nil, fmt.Errorf("--store %q: %w", s.URL, err)
	}
	return &mountset.Stores{Mounts: ms, Connections: cs, UserTokens: oagorm.NewUserTokenStore(s.DB)}, nil
}

// Close closes a database store's connections or a Datastore client. A file:// store holds nothing
// open.
func (s *Store) Close() error {
	switch {
	case s == nil:
		return nil
	case s.Datastore != nil:
		return s.Datastore.Close()
	case s.DB != nil:
		return closeDB(s.DB)
	}
	return nil
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// UploadBlobs opens the blob store under the uploads: the folder's own blobs for a file:// --uploads,
// or the bucket behind a local copy for an s3:// or gs:// one (agni issue 1730). The bucket store
// signs in with the server's own credentials: the AWS default chain, or Application Default
// Credentials for Cloud Storage. Every Put reaches the bucket before it returns, and a file the
// local folder lacks is fetched on demand, so a server that lost its cache refills it. It returns nil
// for a store with no uploads.
func (s *Store) UploadBlobs(ctx context.Context) (blobs.Linker, error) {
	if s == nil || s.uploadsDir == "" {
		return nil, nil
	}
	local, err := blobs.NewDir(filepath.Join(s.uploadsDir, "blobs"))
	if err != nil {
		return nil, fmt.Errorf("--uploads: %w", err)
	}
	var remote blobs.Store
	switch scheme, _, _ := strings.Cut(s.UploadsRemote, "://"); scheme {
	case "":
		return local, nil
	case "s3":
		var o []s3blobs.Option
		if s.opts.s3Endpoint != "" {
			o = append(o, s3blobs.WithS3Options(s3drv.WithEndpoint(s.opts.s3Endpoint)))
		}
		remote, err = s3blobs.New(ctx, s.UploadsRemote, mounts.AWSCredentials{DefaultChain: true}, o...)
	case "gs":
		var o []gcsblobs.Option
		if s.opts.gcsEndpoint != "" {
			o = append(o, gcsblobs.WithGCSOptions(gcsdrv.WithEndpoint(s.opts.gcsEndpoint)))
		}
		// Application Default Credentials sign the requests, with write access, since uploads put
		// files. A fake at --gcs-endpoint, as a test runs, takes no token.
		var creds mounts.Credentials
		if ts, tsErr := google.DefaultTokenSource(ctx, gcsReadWrite); tsErr == nil {
			creds = mounts.Bearer(ts)
		} else if s.opts.gcsEndpoint == "" {
			return nil, fmt.Errorf("--uploads %q: no Application Default Credentials: %w", s.UploadsRemote, tsErr)
		}
		remote, err = gcsblobs.New(ctx, s.UploadsRemote, creds, o...)
	}
	if err != nil {
		return nil, fmt.Errorf("--uploads %q: %w", s.UploadsRemote, err)
	}
	return blobs.NewCached(remote, local), nil
}

// gcsReadWrite is the OAuth scope a gs:// --uploads signs with, since it writes objects.
const gcsReadWrite = "https://www.googleapis.com/auth/devstorage.read_write"

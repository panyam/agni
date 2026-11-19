package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// defaultGitHubAPI is the GitHub REST API a github:// mount reads through.
const defaultGitHubAPI = "https://api.github.com"

// snapshotMaxBytes caps what one snapshot may hold, counted as the files' own sizes. A repo over it
// is ErrResourceExhausted rather than a disk filled by one request (agni issue 1735).
const snapshotMaxBytes int64 = 256 << 20

// snapshotMaxFiles caps the files one snapshot may hold, so a tarball of empty files cannot make
// millions of links.
const snapshotMaxFiles = 100_000

// githubName is what an owner or a repo name may hold on GitHub.
var githubName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

// mountMaker is the server's service.MountMaker (agni issue 1735). It makes a github:// mount by
// resolving the ref to a commit, streaming that commit's tarball into the uploads' blobs, and
// building a view of hard links under <uploads>/mounts/<name>/<commit>/. It keeps one record per
// mount in --store, and the table serves the view read-only.
type mountMaker struct {
	table   *mountset.Table
	records string
	uploads string
	blobs   *projects.BlobDir
	api     string
	http    *http.Client
	// maxBytes and maxFiles cap one snapshot, snapshotMaxBytes and snapshotMaxFiles unless a test
	// sets them.
	maxBytes int64
	maxFiles int
	// mu serializes creates and deletes, so two requests cannot both take one name.
	mu sync.Mutex
}

var _ service.MountMaker = (*mountMaker)(nil)

// newMountMaker reads the mount records into table and returns the maker that writes them.
func newMountMaker(table *mountset.Table, records, uploads, api string) (*mountMaker, error) {
	blobs, err := projects.NewBlobDir(uploads)
	if err != nil {
		return nil, fmt.Errorf("--uploads %q: %w", uploads, err)
	}
	if _, err := table.WithRecords(records, uploads); err != nil {
		return nil, fmt.Errorf("--store: %w", err)
	}
	return &mountMaker{
		table: table, records: records, uploads: uploads, blobs: blobs,
		api:      strings.TrimSuffix(api, "/"),
		http:     &http.Client{Timeout: 5 * time.Minute},
		maxBytes: snapshotMaxBytes, maxFiles: snapshotMaxFiles,
	}, nil
}

// restore rebuilds the view of any recorded mount whose view is missing, from the blobs its record
// lists. A blob the uploads lost leaves that mount's view unbuilt and is reported on log, so the
// server still starts and the other mounts still serve.
func (m *mountMaker) restore(log io.Writer) error {
	recs, err := mountset.ReadRecords(m.records)
	if err != nil {
		return err
	}
	for _, r := range recs {
		if m.table.Fixed(r.Name) {
			fmt.Fprintf(log, "note: the mount record %q is not served, since a --mount of that name is\n", r.Name)
			continue
		}
		view := mountset.SnapshotView(m.uploads, r.Name, r.Commit)
		if _, err := os.Stat(view); err == nil {
			continue
		}
		if err := m.buildView(r.Name, r.Commit, r.Files); err != nil {
			fmt.Fprintf(log, "note: the mount %q cannot be served: %v\n", r.Name, err)
		}
	}
	return nil
}

// buildView links files into a fresh view of name at commit, through a temp folder and a rename.
func (m *mountMaker) buildView(name, commit string, files []mountset.RecordFile) error {
	tmp, err := m.tempView(name)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, f := range files {
		to, err := mountset.SafeResolve(tmp, filepath.FromSlash(f.Path))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
			return err
		}
		if err := m.blobs.Link(f.Sha256, to); err != nil {
			return fmt.Errorf("%s: %w", f.Path, err)
		}
	}
	return m.placeView(tmp, name, commit)
}

// tempView makes a temp folder beside name's views.
func (m *mountMaker) tempView(name string) (string, error) {
	dir := filepath.Join(m.uploads, mountset.SnapshotsDir, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(dir, ".build-*")
}

// placeView renames a built temp folder into name's view at commit, replacing a stale one.
func (m *mountMaker) placeView(tmp, name, commit string) error {
	view := mountset.SnapshotView(m.uploads, name, commit)
	if err := os.RemoveAll(view); err != nil {
		return err
	}
	return os.Rename(tmp, view)
}

// CreateMount snapshots source at ref and serves it as name.
func (m *mountMaker) CreateMount(ctx context.Context, name, source, ref string) (service.MountInfo, error) {
	if err := mountset.ValidName(name); err != nil {
		return service.MountInfo{}, fmt.Errorf("%w: %s", service.ErrInvalidArgument, err)
	}
	owner, repo, err := githubSource(source)
	if err != nil {
		return service.MountInfo{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.table.Find(name); ok {
		return service.MountInfo{}, fmt.Errorf("%w: a mount named %q is already served", service.ErrAlreadyExists, name)
	}
	commit, err := m.resolveRef(ctx, owner, repo, ref)
	if err != nil {
		return service.MountInfo{}, err
	}
	files, err := m.snapshot(ctx, name, owner, repo, commit)
	if err != nil {
		return service.MountInfo{}, err
	}
	rec := mountset.Record{Name: name, Kind: mountset.KindGitHub, Source: "github://" + owner + "/" + repo, Ref: ref, Commit: commit, Created: time.Now().UTC(), Files: files}
	if err := mountset.WriteRecord(m.records, rec); err != nil {
		return service.MountInfo{}, fmt.Errorf("%w: keeping the mount's record: %s", service.ErrInternal, err)
	}
	if err := m.table.Reload(); err != nil {
		return service.MountInfo{}, fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	return mountset.Info(rec.Mount(m.uploads)), nil
}

// DeleteMount forgets name's record and removes its views. Its blobs stay until the next startup's
// sweep finds nothing naming them.
func (m *mountMaker) DeleteMount(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	mt, ok := m.table.Find(name)
	if !ok {
		return fmt.Errorf("%w: no mount named %q", service.ErrNotFound, name)
	}
	if !mt.Runtime {
		return fmt.Errorf("%w: the mount %q comes from the server's flags", service.ErrFailedPrecondition, name)
	}
	if err := mountset.RemoveRecord(m.records, name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	if err := m.table.Reload(); err != nil {
		return fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	if err := os.RemoveAll(filepath.Join(m.uploads, mountset.SnapshotsDir, name)); err != nil {
		return fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	return nil
}

// githubSource reads a github://<owner>/<repo> source. A file:// source is refused, because a client
// must not choose which folder on the server's disk is served.
func githubSource(source string) (owner, repo string, err error) {
	scheme, rest, ok := strings.Cut(source, "://")
	switch {
	case ok && scheme == "github":
	case ok && scheme == "file":
		return "", "", fmt.Errorf("%w: a folder on the server is mounted with --mount, not at runtime", service.ErrInvalidArgument)
	default:
		return "", "", fmt.Errorf("%w: source %q: this build makes github://<owner>/<repo> mounts", service.ErrInvalidArgument, source)
	}
	owner, repo, ok = strings.Cut(strings.TrimSuffix(rest, "/"), "/")
	repo = strings.TrimSuffix(repo, ".git")
	if !ok || !githubName.MatchString(owner) || !githubName.MatchString(repo) || owner == "." || owner == ".." || repo == "." || repo == ".." {
		return "", "", fmt.Errorf("%w: source %q is not github://<owner>/<repo>", service.ErrInvalidArgument, source)
	}
	return owner, repo, nil
}

// resolveRef asks GitHub which commit ref names. An empty ref is the repo's default branch.
func (m *mountMaker) resolveRef(ctx context.Context, owner, repo, ref string) (string, error) {
	if ref == "" {
		var info struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := m.getJSON(ctx, "/repos/"+owner+"/"+repo, &info); err != nil {
			return "", err
		}
		ref = info.DefaultBranch
	}
	var c struct {
		SHA string `json:"sha"`
	}
	if err := m.getJSON(ctx, "/repos/"+owner+"/"+repo+"/commits/"+url.PathEscape(ref), &c); err != nil {
		return "", err
	}
	if !mountset.ValidCommit(c.SHA) {
		return "", fmt.Errorf("%w: GitHub answered %q for %s/%s at %q, which is not a commit", service.ErrUnavailable, c.SHA, owner, repo, ref)
	}
	return c.SHA, nil
}

// getJSON reads one GitHub API answer into v.
func (m *mountMaker) getJSON(ctx context.Context, p string, v any) error {
	resp, err := m.get(ctx, p, "application/vnd.github+json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}

// get sends one GET to the API and classifies a failure: 404 and 422 are ErrNotFound, and anything
// else, a rate limit included, is ErrUnavailable.
func (m *mountMaker) get(ctx context.Context, p, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.api+p, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "agni")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := m.http.Do(req) // #nosec G704 -- the host is the operator's --github-api, and the path is built from validated names
	if err != nil {
		return nil, fmt.Errorf("%w: reaching GitHub: %s", service.ErrUnavailable, err)
	}
	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity {
		return nil, fmt.Errorf("%w: GitHub has no %s", service.ErrNotFound, p)
	}
	return nil, fmt.Errorf("%w: GitHub answered %s for %s", service.ErrUnavailable, resp.Status, p)
}

// snapshot streams the commit's tarball into the blobs and builds the mount's view from it. It
// returns the files it kept, sorted as the tarball lists them. Only regular files are kept. A link,
// a device or a path that would leave the view is skipped, so the view holds plain files alone.
func (m *mountMaker) snapshot(ctx context.Context, name, owner, repo, commit string) ([]mountset.RecordFile, error) {
	resp, err := m.get(ctx, "/repos/"+owner+"/"+repo+"/tarball/"+commit, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: the tarball of %s/%s is not gzip: %s", service.ErrUnavailable, owner, repo, err)
	}
	defer zr.Close()
	tmp, err := m.tempView(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	defer os.RemoveAll(tmp)
	tr := tar.NewReader(zr)
	var files []mountset.RecordFile
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: reading the tarball of %s/%s: %s", service.ErrUnavailable, owner, repo, err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		rel, ok := tarPath(hdr.Name)
		if !ok {
			continue
		}
		if len(files) >= m.maxFiles {
			return nil, fmt.Errorf("%w: %s/%s holds over %d files", service.ErrResourceExhausted, owner, repo, m.maxFiles)
		}
		left := m.maxBytes - total
		b, err := io.ReadAll(io.LimitReader(tr, left+1))
		if err != nil {
			return nil, fmt.Errorf("%w: reading the tarball of %s/%s: %s", service.ErrUnavailable, owner, repo, err)
		}
		total += int64(len(b))
		if total > m.maxBytes {
			return nil, fmt.Errorf("%w: %s/%s at %s holds over %d bytes", service.ErrResourceExhausted, owner, repo, commit[:12], m.maxBytes)
		}
		h, err := m.blobs.Put(b)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", service.ErrInternal, err)
		}
		to, err := mountset.SafeResolve(tmp, filepath.FromSlash(rel))
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
			return nil, fmt.Errorf("%w: %s", service.ErrInternal, err)
		}
		if err := m.blobs.Link(h, to); err != nil {
			return nil, fmt.Errorf("%w: %s", service.ErrInternal, err)
		}
		files = append(files, mountset.RecordFile{Path: rel, Sha256: h})
	}
	if err := m.placeView(tmp, name, commit); err != nil {
		return nil, fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	return files, nil
}

// tarPath is a tarball entry's path inside the repo. GitHub puts every entry under one folder named
// for the repo and commit, which is dropped. A path that is absolute, climbs out with "..", or
// names the top folder itself reports false.
func tarPath(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	_, rest, ok := strings.Cut(name, "/")
	if !ok || rest == "" || strings.HasPrefix(rest, "/") {
		return "", false
	}
	clean := path.Clean(rest)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != rest {
		return "", false
	}
	return clean, true
}

package main

import (
	"context"
	"crypto/sha1" // #nosec G505 -- git names a blob by SHA-1, and this checks a download against that name
	"crypto/sha256"
	"encoding/hex"
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
	"slices"
	"strings"
	"sync"

	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// defaultGitHubRaw serves a repo's files by commit and path, outside the REST API's rate limit.
const defaultGitHubRaw = "https://raw.githubusercontent.com"

// gitIndexDir is the folder under --uploads that maps a git blob id to the hash its contents are kept
// under, so a sync of a file it already holds downloads nothing (agni issue 1723).
const gitIndexDir = "links/git"

// treeCacheSize caps how many commits' file lists linkReaders keeps.
const treeCacheSize = 8

// linkReaders gives the service.MountReader for each mount an artifact can link to (agni issue
// 1723). A github mount is read through GitHub's trees API and its raw files. A folder from --mount
// is read from disk. The uploads mount holds uploaded files and can't be linked.
type linkReaders struct {
	table *mountset.Table
	gh    *mountMaker // GitHub's API client, caps and the blobs
	raw   string
	index string // <uploads>/links/git

	mu    sync.Mutex
	trees map[string][]service.MountFile // by owner/repo@commit
}

func newLinkReaders(table *mountset.Table, gh *mountMaker, raw string) *linkReaders {
	return &linkReaders{table: table, gh: gh, raw: strings.TrimSuffix(raw, "/"), index: filepath.Join(gh.uploads, filepath.FromSlash(gitIndexDir)), trees: map[string][]service.MountFile{}}
}

// reader is a service.MountReaders.
func (l *linkReaders) reader(_ context.Context, name string) (service.MountReader, error) {
	m, ok := l.table.Find(name)
	if !ok {
		return nil, fmt.Errorf("%w: no mount named %q", service.ErrNotFound, name)
	}
	switch m.Kind {
	case mountset.KindUploads:
		return nil, fmt.Errorf("%w: the mount %q holds uploaded files; UpdateArtifact puts them", service.ErrInvalidArgument, name)
	case mountset.KindGitHub:
		owner, repo, err := githubSource(m.Source)
		if err != nil {
			return nil, err
		}
		return &githubReader{l: l, owner: owner, repo: repo}, nil
	case mountset.KindFile:
		return &folderReader{root: m.Root, blobs: l.gh.blobs, maxBytes: l.gh.maxBytes, maxFiles: l.gh.maxFiles}, nil
	}
	return nil, fmt.Errorf("%w: the mount %q is of kind %q, which this server can't link", service.ErrInvalidArgument, name, m.Kind)
}

// githubReader reads one repo through linkReaders' client.
type githubReader struct {
	l           *linkReaders
	owner, repo string
}

// Resolve resolves ref to a commit. An empty ref and a branch are followed, and anything else is
// pinned.
func (g *githubReader) Resolve(ctx context.Context, ref string) (string, bool, error) {
	commit, err := g.l.gh.resolveRef(ctx, g.owner, g.repo, ref)
	if err != nil {
		return "", false, err
	}
	if ref == "" || ref == commit {
		return commit, ref == "", nil
	}
	resp, err := g.l.gh.get(ctx, "/repos/"+g.owner+"/"+g.repo+"/branches/"+url.PathEscape(ref), "application/vnd.github+json")
	if errors.Is(err, service.ErrNotFound) {
		return commit, false, nil
	}
	if err != nil {
		return "", false, err
	}
	resp.Body.Close()
	return commit, true, nil
}

// List lists the files at commit under the path under, from one call to the trees API per commit.
func (g *githubReader) List(ctx context.Context, commit, under string) ([]service.MountFile, error) {
	all, err := g.tree(ctx, commit)
	if err != nil {
		return nil, err
	}
	out, err := within(all, under, g.l.gh.maxBytes, g.l.gh.maxFiles)
	if err != nil {
		return nil, fmt.Errorf("%s/%s at %s: %w", g.owner, g.repo, commit, err)
	}
	return out, nil
}

// tree is every regular file in the repo at commit, sorted by path. A link and a submodule are left
// out, as #1735's snapshot leaves them out.
func (g *githubReader) tree(ctx context.Context, commit string) ([]service.MountFile, error) {
	key := g.owner + "/" + g.repo + "@" + commit
	g.l.mu.Lock()
	cached, ok := g.l.trees[key]
	g.l.mu.Unlock()
	if ok {
		return cached, nil
	}
	resp, err := g.l.gh.get(ctx, "/repos/"+g.owner+"/"+g.repo+"/git/trees/"+commit+"?recursive=1", "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var t struct {
		Tree []struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int64  `json:"size"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&t); err != nil {
		return nil, fmt.Errorf("%w: reading %s's file list: %s", service.ErrUnavailable, key, err)
	}
	if t.Truncated {
		return nil, fmt.Errorf("%w: GitHub lists %s only in part, since it holds too many files", service.ErrResourceExhausted, key)
	}
	var out []service.MountFile
	for _, e := range t.Tree {
		if e.Type != "blob" || e.Mode == "120000" || !gitBlobID(e.SHA) {
			continue
		}
		if _, ok := tarPath("top/" + e.Path); !ok {
			continue
		}
		out = append(out, service.MountFile{Path: e.Path, ID: e.SHA, Size: e.Size})
	}
	slices.SortFunc(out, func(a, b service.MountFile) int { return strings.Compare(a.Path, b.Path) })
	g.l.mu.Lock()
	if len(g.l.trees) >= treeCacheSize {
		clear(g.l.trees)
	}
	g.l.trees[key] = out
	g.l.mu.Unlock()
	return out, nil
}

// Fetch keeps f's contents in the blobs. A git blob id the index maps to a held blob downloads
// nothing. A download is checked against the blob id before it is kept.
func (g *githubReader) Fetch(ctx context.Context, commit string, f service.MountFile) (string, bool, error) {
	if !gitBlobID(f.ID) {
		return "", false, fmt.Errorf("%w: %s has no git blob id", service.ErrInternal, f.Path)
	}
	if h, err := os.ReadFile(filepath.Join(g.l.index, f.ID)); err == nil { // #nosec G304 -- a 40-hex name inside --uploads
		if p, ok := g.l.gh.blobs.Path(string(h)); ok {
			if _, err := os.Stat(p); err == nil { // #nosec G703 -- Path accepts only a well-formed sha256 and joins it under the blobs
				return string(h), false, nil
			}
		}
	}
	if f.Size > g.l.gh.maxBytes {
		return "", false, fmt.Errorf("%w: %s is %d bytes, over the %d a link may hold", service.ErrResourceExhausted, f.Path, f.Size, g.l.gh.maxBytes)
	}
	b, err := g.download(ctx, commit, f)
	if err != nil {
		return "", false, err
	}
	if id := gitBlobSHA(b); id != f.ID {
		return "", false, fmt.Errorf("%w: GitHub sent %s with git id %s, not the %s its file list names", service.ErrUnavailable, f.Path, id, f.ID)
	}
	h, err := g.l.gh.blobs.Put(b)
	if err != nil {
		return "", false, fmt.Errorf("%w: keeping %s: %s", service.ErrInternal, f.Path, err)
	}
	if err := writeIndex(g.l.index, f.ID, h); err != nil {
		return "", false, fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	return h, true, nil
}

// download reads one file at commit from the raw host.
func (g *githubReader) download(ctx context.Context, commit string, f service.MountFile) ([]byte, error) {
	segs := strings.Split(f.Path, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	u := g.l.raw + "/" + g.owner + "/" + g.repo + "/" + commit + "/" + strings.Join(segs, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", service.ErrInternal, err)
	}
	req.Header.Set("User-Agent", "agni")
	resp, err := g.l.gh.http.Do(req) // #nosec G704 -- the host is the operator's --github-raw, and the path comes from GitHub's own file list
	if err != nil {
		return nil, fmt.Errorf("%w: reaching GitHub: %s", service.ErrUnavailable, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("%w: GitHub has no %s at %s", service.ErrNotFound, f.Path, commit)
	default:
		return nil, fmt.Errorf("%w: GitHub answered %s for %s", service.ErrUnavailable, resp.Status, f.Path)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, f.Size+1))
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %s", service.ErrUnavailable, f.Path, err)
	}
	return b, nil
}

// writeIndex records that git blob id is kept under hash, through a temp file and a rename.
func writeIndex(dir, id, hash string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".put-*")
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(hash); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, id))
}

// gitBlobID reports whether s is a 40-hex git object id.
func gitBlobID(s string) bool { return mountset.ValidCommit(s) }

// gitBlobSHA is the id git gives contents b.
func gitBlobSHA(b []byte) string {
	h := sha1.New() // #nosec G401 -- git's own object id, not a security check
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// folderReader reads a folder a --mount serves. It has no history, so it names no version and every
// link to it is followed.
type folderReader struct {
	root     string
	blobs    *projects.BlobDir
	maxBytes int64
	maxFiles int
}

func (r *folderReader) Resolve(context.Context, string) (string, bool, error) { return "", true, nil }

// List walks the folder at under, or returns the one file it names. A link and anything else that
// is not a regular file is left out.
func (r *folderReader) List(_ context.Context, _, under string) ([]service.MountFile, error) {
	at, err := mountset.SafeResolve(r.root, filepath.FromSlash(under))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", service.ErrInvalidArgument, err)
	}
	info, err := os.Lstat(at)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s", service.ErrUnavailable, err)
	}
	if info.Mode().IsRegular() {
		return []service.MountFile{{Path: under, Size: info.Size()}}, nil
	}
	if !info.IsDir() {
		return nil, nil
	}
	var all []service.MountFile
	err = filepath.WalkDir(at, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.root, p)
		if err != nil {
			return err
		}
		all = append(all, service.MountFile{Path: filepath.ToSlash(rel), Size: fi.Size()})
		if len(all) > r.maxFiles {
			return fmt.Errorf("%w: %s holds over %d files", service.ErrResourceExhausted, under, r.maxFiles)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, service.ErrResourceExhausted) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", service.ErrUnavailable, err)
	}
	return within(all, under, r.maxBytes, r.maxFiles)
}

// Fetch reads f from disk and keeps it in the blobs. fetched is true when the blobs held no copy.
func (r *folderReader) Fetch(_ context.Context, _ string, f service.MountFile) (string, bool, error) {
	at, err := mountset.SafeResolve(r.root, filepath.FromSlash(f.Path))
	if err != nil {
		return "", false, fmt.Errorf("%w: %s", service.ErrInvalidArgument, err)
	}
	b, err := os.ReadFile(at) // #nosec G304 -- resolved inside the mount's root
	if err != nil {
		return "", false, fmt.Errorf("%w: %s", service.ErrUnavailable, err)
	}
	sum := sha256.Sum256(b)
	h := "sha256:" + hex.EncodeToString(sum[:])
	held := false
	if p, ok := r.blobs.Path(h); ok {
		_, err := os.Stat(p) // #nosec G703 -- Path accepts only a well-formed sha256 and joins it under the blobs
		held = err == nil
	}
	if !held {
		if _, err := r.blobs.Put(b); err != nil {
			return "", false, fmt.Errorf("%w: keeping %s: %s", service.ErrInternal, f.Path, err)
		}
	}
	return h, !held, nil
}

// within keeps the files at or under the path under, sorted by path, and refuses a set over the
// caps a link may hold.
func within(all []service.MountFile, under string, maxBytes int64, maxFiles int) ([]service.MountFile, error) {
	var out []service.MountFile
	var total int64
	for _, f := range all {
		if under != "" && f.Path != under && !strings.HasPrefix(f.Path, under+"/") {
			continue
		}
		out = append(out, f)
		total += f.Size
	}
	if len(out) > maxFiles {
		return nil, fmt.Errorf("%w: %s holds %d files, over the %d a link may hold", service.ErrResourceExhausted, displayPath(under), len(out), maxFiles)
	}
	if total > maxBytes {
		return nil, fmt.Errorf("%w: %s holds %d bytes, over the %d a link may hold", service.ErrResourceExhausted, displayPath(under), total, maxBytes)
	}
	return out, nil
}

func displayPath(p string) string {
	if p == "" {
		return "the mount's root"
	}
	return path.Clean(p)
}

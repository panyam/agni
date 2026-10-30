// Package webbundle finds the viewer for an agni binary that runs outside a checkout (agni WS16-003).
//
// The server reads the viewer from a directory, so both sources end as one under the user cache. A
// release binary carries the bundle (package web, built with the embedweb tag) and writes it out
// once. A `go install` binary carries none, since the built JS is not in the Go module, and downloads
// the bundle its own release attached, checked against that release's checksums.txt. A build that
// is not a release (a checkout's `make agni`, `go install ...@main`) has no bundle to name, so it
// gets ErrNoBundle and keeps today's answer: `open` refuses and `serve` serves the API alone.
package webbundle

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ErrNoBundle means this binary has no viewer to offer, neither embedded nor a release to fetch from.
var ErrNoBundle = errors.New("this agni build carries no viewer")

// DefaultReleases is where a release's assets download from, as <base>/<tag>/<asset>.
const DefaultReleases = "https://github.com/panyam/agni/releases/download"

// AssetName is the viewer bundle a release attaches beside its binaries.
func AssetName(tag string) string { return "agni-web-" + tag + ".tar.gz" }

// ChecksumsName is the sha256 list a release attaches, one "<hex>  <file>" line per asset.
const ChecksumsName = "checksums.txt"

// maxBundle caps what a download or an extracted file may hold. The bundle is about 1 MB gzipped.
const maxBundle = 64 << 20

// releaseTag is the only version shape a release has, so a pseudo-version or a dirty build never
// names a download that does not exist.
var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// Source says where a viewer may come from and where to keep it.
type Source struct {
	// Embedded is the bundle the binary carries, nil when it carries none.
	Embedded fs.FS
	// Version is the binary's own version. A download is tried only for a release tag.
	Version string
	// Releases is the download base, DefaultReleases unless a test points it elsewhere.
	Releases string
	// CacheRoot holds one directory per bundle, under web/.
	CacheRoot string
	Client    *http.Client
	// Notes receives one line before a download, since a command that reaches the network should say so.
	Notes io.Writer
}

// Dir returns a directory holding the viewer's templates/ and static/, and how it got there
// ("embedded" or "downloaded"). A directory written by an earlier call is reused. An error wraps
// ErrNoBundle when the binary has nothing to offer; any other error is a download or a write that
// failed, which a caller reports.
func (s Source) Dir(ctx context.Context) (dir, how string, err error) {
	if s.Embedded != nil {
		key, err := treeHash(s.Embedded)
		if err != nil {
			return "", "", err
		}
		dir, err := s.materialize("embedded-"+key, func(tmp string) error { return copyTree(s.Embedded, tmp) })
		return dir, "embedded", err
	}
	if !releaseTag.MatchString(s.Version) {
		return "", "", fmt.Errorf("%w: %s is not a release, so there is no bundle to download", ErrNoBundle, s.Version)
	}
	dir, err = s.materialize(s.Version, func(tmp string) error { return s.download(ctx, tmp) })
	return dir, "downloaded", err
}

// materialize returns CacheRoot/web/<key>, filling it through fill once. It fills a temporary
// sibling and renames it into place, so a run interrupted halfway leaves nothing a later run reads as
// complete, and two runs racing both end with one whole directory.
func (s Source) materialize(key string, fill func(tmp string) error) (string, error) {
	root := filepath.Join(s.CacheRoot, "web")
	dir := filepath.Join(root, key)
	if _, err := os.Stat(filepath.Join(dir, "templates", "ViewerPage.html")); err == nil {
		return dir, nil
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(root, ".partial-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := fill(tmp); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(tmp, "templates", "ViewerPage.html")); err != nil {
		return "", fmt.Errorf("the viewer bundle has no templates/ViewerPage.html")
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another run finished first, which is as good as this one finishing.
		if _, serr := os.Stat(filepath.Join(dir, "templates", "ViewerPage.html")); serr == nil {
			return dir, nil
		}
		return "", err
	}
	return dir, nil
}

func (s Source) download(ctx context.Context, into string) error {
	base := strings.TrimSuffix(s.Releases, "/") + "/" + s.Version + "/"
	name := AssetName(s.Version)
	sums, err := s.fetch(ctx, base+ChecksumsName)
	if err != nil {
		return err
	}
	want, err := checksumFor(sums, name)
	if err != nil {
		return fmt.Errorf("%s%s: %w", base, ChecksumsName, err)
	}
	if s.Notes != nil {
		fmt.Fprintf(s.Notes, "note: this agni carries no viewer, so it is downloading the %s viewer once from %s\n", s.Version, base+name)
	}
	body, err := s.fetch(ctx, base+name)
	if err != nil {
		return err
	}
	got := sha256.Sum256(body)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("%s does not match its checksum in %s", base+name, ChecksumsName)
	}
	return untar(body, into)
}

func (s Source) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req) // #nosec G107 G704 -- the URL is the release base plus a fixed asset name
	if err != nil {
		return nil, fmt.Errorf("downloading the viewer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading the viewer: %s answered %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBundle+1))
	if err != nil {
		return nil, fmt.Errorf("downloading the viewer: %w", err)
	}
	if len(body) > maxBundle {
		return nil, fmt.Errorf("downloading the viewer: %s is larger than %d bytes", url, maxBundle)
	}
	return body, nil
}

// checksumFor reads name's sha256 from a sha256sum-format list.
func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum for %s", name)
}

// untar writes a gzipped tar's regular files under into. An entry naming a path outside into, a link
// or anything but a file or directory is refused, since the archive came over the network.
func untar(gz []byte, into string) error {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return fmt.Errorf("the viewer bundle: %w", err)
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("the viewer bundle: %w", err)
		}
		name := path.Clean(strings.TrimPrefix(h.Name, "./"))
		if name == "." {
			continue
		}
		if !fs.ValidPath(name) {
			return fmt.Errorf("the viewer bundle names %q, outside its own directory", h.Name)
		}
		dest := filepath.Join(into, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o750); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(dest, io.LimitReader(tr, maxBundle)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("the viewer bundle holds %q, which is not a file or a directory", h.Name)
		}
	}
}

func copyTree(src fs.FS, into string) error {
	return fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == "." {
			return err
		}
		dest := filepath.Join(into, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dest, 0o750)
		}
		f, err := src.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		return writeFile(dest, f)
	})
}

func writeFile(dest string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- dest is under the cache dir, checked by the caller
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// treeHash names an embedded bundle by its contents, so two builds of one version with different
// viewers never share a cache directory.
func treeHash(src fs.FS) (string, error) {
	var paths []string
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			paths = append(paths, p)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		b, err := fs.ReadFile(src, p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

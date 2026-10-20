package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// A design a client sends as files rather than naming files the host serves (agni issue 1061, step
// 4b). The snapshot's definition lists them as a manifest, each path within the design's mount with
// its hash, and the contents travel apart, as blobs keyed by hash, so a client sends only what the
// host does not already hold. Each manifest is read through a host of its own, over those files
// alone, with a cache of its own: the files keep the client's own paths and mount name, which may be
// one this host also serves, and a cache keys a read on its URI.

// ErrBlobsMissing is an Ingest naming files whose contents the host does not hold. The error is a
// *BlobsMissingError, which names them.
var ErrBlobsMissing = errors.New("blobs missing")

// BlobsMissingError names the contents an Ingest needs sent, which a transport carries as a
// webapi.BlobsMissing detail.
type BlobsMissingError struct{ Hashes []string }

func (e *BlobsMissingError) Error() string {
	return fmt.Sprintf("%d of the design's files have not been sent; send them with UploadBlobs and ingest again", len(e.Hashes))
}

func (e *BlobsMissingError) Unwrap() error { return ErrBlobsMissing }

// Detail is the error as the wire carries it.
func (e *BlobsMissingError) Detail() *webapi.BlobsMissing {
	return &webapi.BlobsMissing{Sha256: e.Hashes}
}

// FilesWithheldError is a question that read a file whose contents its host was not given, which the
// browser engine does with a design's board until a question needs it (agni issue 992). Hashes are
// the withheld files' content hashes, and a transport carries them as a webapi.BlobsMissing detail,
// the same shape an Ingest naming unsent files answers with, so a client sends those files and asks
// again.
type FilesWithheldError struct{ Hashes []string }

func (e *FilesWithheldError) Error() string {
	return fmt.Sprintf("%d of the design's files were held back from this engine and the question reads them; send them and ask again", len(e.Hashes))
}

// Detail is the error as the wire carries it.
func (e *FilesWithheldError) Detail() *webapi.BlobsMissing {
	return &webapi.BlobsMissing{Sha256: e.Hashes}
}

// An UploadHost is what a host reads a client's files through: a loader over files, keyed by their
// path within the one mount named mount, and a resolver over the projects those files declare. It
// is the host's to build, since only a host can make a file system (C13); a nil UploadHost is a host
// that takes no files from its clients.
type UploadHost func(mount string, files map[string][]byte) (CacheableLoader, *ProjectResolver)

// defaultUploadBytes is how much uploaded content a host keeps when UploadMaxBytes is unset.
const defaultUploadBytes = 1 << 30

// uploadCacheEntries is how many reads each upload's own cache keeps: one design, its drawing, its
// board and a model or two, as the browser engine's cache holds for one design.
const uploadCacheEntries = 10

// uploadHosts is how many manifests' hosts are kept for a later ingest of the same files. A snapshot
// keeps its own host however many come after it.
const uploadHosts = 16

// uploadHost is one manifest's files and what reads them.
type uploadHost struct {
	key      string
	loader   *CachingLoader
	resolver *ProjectResolver
	used     uint64
}

// uploads is a host's blobs and the hosts built over them, held on the ProjectResolver its services
// share, as its snapshots are.
type uploads struct {
	mu    sync.Mutex
	blobs map[string][]byte
	order map[string]uint64 // blob -> when it was last stored or used
	bytes int64
	max   int64
	hosts map[string]*uploadHost
	tick  uint64
}

func (r *ProjectResolver) uploadsOf() *uploads {
	r.snapMu.Lock()
	defer r.snapMu.Unlock()
	if r.ups == nil {
		max := r.UploadMaxBytes
		if max <= 0 {
			max = defaultUploadBytes
		}
		r.ups = &uploads{blobs: map[string][]byte{}, order: map[string]uint64{}, max: max, hosts: map[string]*uploadHost{}}
	}
	return r.ups
}

// put stores b under its hash and answers the hash, evicting the least recently used blobs past the
// limit. A blob larger than the limit is refused rather than evicting everything for nothing.
func (u *uploads) put(b []byte) (string, error) {
	sum := sha256.Sum256(b)
	h := "sha256:" + hex.EncodeToString(sum[:])
	u.mu.Lock()
	defer u.mu.Unlock()
	if int64(len(b)) > u.max {
		return "", fmt.Errorf("%w: a file of %d bytes is over this host's %d-byte upload limit", ErrResourceExhausted, len(b), u.max)
	}
	u.tick++
	u.order[h] = u.tick
	if _, ok := u.blobs[h]; ok {
		return h, nil
	}
	u.blobs[h] = b
	u.bytes += int64(len(b))
	for u.bytes > u.max {
		var oldest string
		var at uint64
		for k, t := range u.order {
			if k != h && (oldest == "" || t < at) {
				oldest, at = k, t
			}
		}
		if oldest == "" {
			break
		}
		u.bytes -= int64(len(u.blobs[oldest]))
		delete(u.blobs, oldest)
		delete(u.order, oldest)
	}
	return h, nil
}

// uploaded is the host for a manifest of files in mount, built over the blobs the first time it is
// asked for and kept for the next ingest of the same files. A manifest naming contents the host does
// not hold is a *BlobsMissingError.
func (r *ProjectResolver) uploaded(mount string, files []*webapi.SnapshotFile) (*uploadHost, error) {
	if r.Uploads == nil {
		return nil, fmt.Errorf("%w: this host takes no files from its clients; name a design it serves", ErrInvalidArgument)
	}
	var total int64
	seen := map[string]bool{}
	for _, f := range files {
		// A path is taken as the client wrote it only when it is already clean and stays in the
		// mount, so two spellings of one file cannot both be listed.
		if a, err := artifact.New(mount, f.GetPath()); err != nil || a.Path != f.GetPath() || f.GetPath() == "" {
			return nil, fmt.Errorf("%w: %q is not a clean path within a mount", ErrInvalidPath, f.GetPath())
		}
		if seen[f.GetPath()] {
			return nil, fmt.Errorf("%w: %q is listed twice", ErrInvalidArgument, f.GetPath())
		}
		if !strings.HasPrefix(f.GetSha256(), "sha256:") {
			return nil, fmt.Errorf("%w: %q has no sha256", ErrInvalidArgument, f.GetPath())
		}
		seen[f.GetPath()] = true
		total += f.GetSize()
	}
	if total > DesignFilesMaxBytes {
		return nil, fmt.Errorf("%w: the design's files are %d bytes, over the %d a design sent as files may be", ErrInvalidArgument, total, int64(DesignFilesMaxBytes))
	}
	key := manifestKey(mount, files)
	u := r.uploadsOf()
	u.mu.Lock()
	defer u.mu.Unlock()
	u.tick++
	if h, ok := u.hosts[key]; ok {
		h.used = u.tick
		return h, nil
	}
	contents := map[string][]byte{}
	var missing []string
	for _, f := range files {
		b, ok := u.blobs[f.GetSha256()]
		if !ok {
			missing = append(missing, f.GetSha256())
			continue
		}
		u.order[f.GetSha256()] = u.tick
		contents[f.GetPath()] = b
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, &BlobsMissingError{Hashes: slicesCompact(missing)}
	}
	loader, resolver := r.Uploads(mount, contents)
	h := &uploadHost{key: key, loader: NewCachingLoader(loader, NewDesignCache(uploadCacheEntries)), resolver: resolver, used: u.tick}
	u.hosts[key] = h
	for len(u.hosts) > uploadHosts {
		var oldest string
		var at uint64
		for k, v := range u.hosts {
			if oldest == "" || v.used < at {
				oldest, at = k, v.used
			}
		}
		delete(u.hosts, oldest)
	}
	return h, nil
}

// manifestKey identifies a manifest by its mount and every path and hash in it.
func manifestKey(mount string, files []*webapi.SnapshotFile) string {
	lines := make([]string, 0, len(files))
	for _, f := range files {
		lines = append(lines, f.GetPath()+"\x00"+f.GetSha256())
	}
	sort.Strings(lines)
	h := sha256.Sum256([]byte(mount + "\x01" + strings.Join(lines, "\x01")))
	return hex.EncodeToString(h[:])
}

func slicesCompact(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// UploadBlobs stores the contents of a design's files a client sends, keyed by their SHA-256, and
// answers each one's hash.
func (s *SnapshotService) UploadBlobs(ctx context.Context, req *webapi.UploadBlobsRequest) (*webapi.UploadBlobsResponse, error) {
	if s.projects.Uploads == nil {
		return nil, fmt.Errorf("%w: this host takes no files from its clients", ErrInvalidArgument)
	}
	u := s.projects.uploadsOf()
	resp := &webapi.UploadBlobsResponse{}
	for _, b := range req.GetContents() {
		h, err := u.put(b)
		if err != nil {
			return nil, err
		}
		resp.Sha256 = append(resp.Sha256, h)
	}
	return resp, nil
}

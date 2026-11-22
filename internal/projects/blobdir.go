package projects

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/panyam/goapplib/storage/blobs"

	"github.com/panyam/agni/service"
)

// blobsDir is the folder under the uploads folder that holds the blobs.
const blobsDir = "blobs"

// BlobDir is the uploads' blob store with the byte-slice calls agni's stores and service.UploadStore
// use (agni issues 1739, 1759). It is goapplib's blobs.Dir at <uploads>/blobs, or a bucket behind
// that folder for an s3:// or gs:// --uploads (agni issue 1730). A server reads and writes uploaded
// contents through it, so an upload outlives a restart.
type BlobDir struct{ d blobs.Linker }

// NewBlobDir opens the blob folder under uploads, creating it if absent. Its layout is
// sha256/ab/cdef..., which agni has always used, so blobs from an older agni stay where they are.
func NewBlobDir(uploads string) (*BlobDir, error) {
	d, err := blobs.NewDir(filepath.Join(uploads, blobsDir))
	if err != nil {
		return nil, err
	}
	return &BlobDir{d: d}, nil
}

// NewBlobDirOver wraps a blob store the server opened, such as a bucket behind a local folder.
func NewBlobDirOver(l blobs.Linker) *BlobDir { return &BlobDir{d: l} }

// Store is the library's store, for a sweep and for the mounts registry.
func (b *BlobDir) Store() blobs.Linker { return b.d }

// Put stores p and returns its hash, "sha256:<hex>". Storing contents already held writes nothing.
func (b *BlobDir) Put(p []byte) (string, error) {
	h, err := b.d.Put(context.Background(), bytes.NewReader(p))
	return string(h), err
}

// Get returns the contents stored under hash. A hash the folder lacks, a malformed one, or contents
// that no longer match it report false.
func (b *BlobDir) Get(hash string) ([]byte, bool) {
	r, err := b.d.Open(context.Background(), blobs.Hash(hash))
	if err != nil {
		return nil, false
	}
	defer r.Close()
	p, err := io.ReadAll(r)
	if err != nil {
		return nil, false
	}
	return p, true
}

// Link makes the file at to hold hash's contents, hard-linking where the filesystem allows. A view
// built from links keeps its files when a sweep later removes the blob.
func (b *BlobDir) Link(hash, to string) error {
	err := b.d.Link(blobs.Hash(hash), to)
	if errors.Is(err, blobs.ErrInvalidHash) {
		return fmt.Errorf("%w: %q is not a sha256 hash", service.ErrInvalidArgument, hash)
	}
	return err
}

// keeping is a blobs.Referrer naming every hash in keep.
func keeping(keep map[string]bool) blobs.Referrer {
	return func(_ context.Context, add func(blobs.Hash)) error {
		for h := range keep {
			add(blobs.Hash(h))
		}
		return nil
	}
}

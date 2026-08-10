package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/panyam/agni/core/results"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/internal/service"
)

// osReviewStore is the filesystem-backed service.ReviewStore: one results document per run, written
// into the directory `agni serve --review-store` names. That directory is a WRITABLE volume, mounted
// separately from the read-only design mounts, so persisting runs never turns a design mount into a
// write surface.
//
// One flat directory of files is the whole design, and it is enough because of what an id is. Ids
// lead with a UTC timestamp, so the filenames sort chronologically as plain strings, which means a
// listing is a directory read plus a sort with no document opened. Only the page a client actually
// asked for is parsed. An index would be a second source of truth to keep consistent with the files,
// and the files are already the truth.
type osReviewStore struct{ dir string }

// newOSReviewStore returns a store over dir, creating it when absent so a fresh volume works on first
// boot rather than failing the first create. A path that exists and is not a directory is an error at
// startup, where an operator can still fix it.
func newOSReviewStore(dir string) (*osReviewStore, error) {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("--review-store %s: %w", dir, err)
		}
	case err != nil:
		return nil, fmt.Errorf("--review-store %s: %w", dir, err)
	case !info.IsDir():
		return nil, fmt.Errorf("--review-store %s: not a directory", dir)
	}
	return &osReviewStore{dir: dir}, nil
}

// reviewFileSuffix is what marks a file in the store as a run. Anything else in the directory is
// ignored rather than treated as a corrupt run, so an operator's stray note beside the volume does
// not break a listing.
const reviewFileSuffix = ".results.json"

func (s *osReviewStore) path(id string) string { return filepath.Join(s.dir, id+reviewFileSuffix) }

// Create writes the document under a fresh id and returns its resource name and creation time.
//
// The id is "<UTC yyyymmddThhmmss>.<nanoseconds>Z-<8 hex>", and the nanoseconds are load-bearing
// rather than decorative. Ordering is a plain string sort over these ids, so any two runs that share
// a prefix fall through to the random tail and come back in an arbitrary order. At second
// resolution that is not an edge case: a CI job reviewing several boards fires them inside the same
// second, and a listing that claims to be newest-first would silently shuffle them. The random tail
// then covers the residual tie, and O_EXCL means even an exact collision refuses rather than
// clobbers.
func (s *osReviewStore) Create(_ context.Context, doc *checkspb.CheckResults) (string, string, error) {
	now := time.Now().UTC()
	createdAt := now.Format(time.RFC3339)
	// Stamp before marshalling: the file on disk has to carry the same creation time the caller is
	// handed back, or a re-read of the run would disagree with the response that created it.
	stamped, err := withCreatedAt(doc, createdAt)
	if err != nil {
		return "", "", err
	}
	b, err := results.Marshal(stamped)
	if err != nil {
		return "", "", err
	}
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", "", err
	}
	id := fmt.Sprintf("%s.%09dZ-%s", now.Format("20060102T150405"), now.Nanosecond(), hex.EncodeToString(suffix[:]))
	f, err := os.OpenFile(s.path(id), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	if _, err := f.Write(b); err != nil {
		return "", "", err
	}
	return service.ReviewName(id), createdAt, nil
}

func (s *osReviewStore) Get(_ context.Context, name string) (*checkspb.CheckResults, error) {
	id, ok := service.ReviewID(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a review name", service.ErrInvalidArgument, name)
	}
	b, err := os.ReadFile(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: no review %q", service.ErrNotFound, name)
	}
	if err != nil {
		return nil, err
	}
	return results.Parse(b)
}

func (s *osReviewStore) List(_ context.Context, pageSize int, pageToken, designFilter string) ([]*checkspb.CheckResults, []string, string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, nil, "", err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if id, ok := strings.CutSuffix(e.Name(), reviewFileSuffix); ok {
			ids = append(ids, id)
		}
	}
	// Ids lead with a timestamp, so a plain reverse string sort IS newest-first.
	service.SortReviewIDsDescending(ids)
	// A document that will not parse is SKIPPED rather than failing the listing. One corrupt file in a
	// long-lived volume would otherwise make every run unlistable, and the failure a user meets would
	// name pagination rather than the bad file.
	return service.PageReviews(ids, pageSize, pageToken, designFilter, func(id string) *checkspb.CheckResults {
		b, err := os.ReadFile(s.path(id))
		if err != nil {
			return nil
		}
		doc, err := results.Parse(b)
		if err != nil {
			return nil
		}
		return doc
	})
}

func (s *osReviewStore) Delete(_ context.Context, name string) error {
	id, ok := service.ReviewID(name)
	if !ok {
		return fmt.Errorf("%w: %q is not a review name", service.ErrInvalidArgument, name)
	}
	err := os.Remove(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: no review %q", service.ErrNotFound, name)
	}
	return err
}

// withCreatedAt returns a copy of doc with meta.created_at set, leaving the caller's document alone.
// Copying matters because the caller still holds the original and sets the same field from the value
// this store returns; mutating in place would make the two paths look interchangeable right up until
// one of them is dropped.
func withCreatedAt(doc *checkspb.CheckResults, createdAt string) (*checkspb.CheckResults, error) {
	b, err := results.Marshal(doc)
	if err != nil {
		return nil, err
	}
	// Marshal/Parse rather than proto.Clone: this also proves the document round-trips through the
	// very encoding it is about to be stored in, so a document that cannot be read back is caught
	// while the run is still in hand rather than at some later Get.
	clone, err := results.Parse(b)
	if err != nil {
		return nil, err
	}
	if clone.Meta == nil {
		clone.Meta = &checkspb.ResultsMeta{}
	}
	clone.Meta.CreatedAt = createdAt
	return clone, nil
}

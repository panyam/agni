package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/panyam/agni/core/results"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/encoding/protojson"
)

// osReviewStore is the service.ReviewStore under `agni serve --store`, keeping one results document
// per run as a record under the store's reviews folder, in files or in sqlite (agni issue 1733). That directory is a WRITABLE volume, mounted
// separately from the read-only design mounts, so persisting runs never writes into a design mount.
//
// There is no index. Ids lead with a UTC timestamp, so filenames sort chronologically as plain
// strings and a listing is a directory read plus a sort, parsing only the page a client asked for.
type osReviewStore struct {
	recs records.Store
	now  func() time.Time // the clock, which a test freezes

	mu   sync.Mutex
	last time.Time // the latest creation time this store handed out
}

// newOSReviewStore returns a store over dir, creating it when absent so a fresh volume works on first
// boot rather than failing the first create. A path that exists and is not a directory is an error at
// startup, where an operator can still fix it.
func newOSReviewStore(dir string) (*osReviewStore, error) {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("--store, its reviews folder %s: %w", dir, err)
		}
	case err != nil:
		return nil, fmt.Errorf("--store, its reviews folder %s: %w", dir, err)
	case !info.IsDir():
		return nil, fmt.Errorf("--store, its reviews folder %s: not a directory", dir)
	}
	d, err := records.NewDir(dir)
	if err != nil {
		return nil, fmt.Errorf("--store, its reviews folder %s: %w", dir, err)
	}
	return newReviewStore(d), nil
}

// newReviewStore returns a store keeping each run as a record in recs, as a folder or sqlite keeps
// them (agni issue 1733).
func newReviewStore(recs records.Store) *osReviewStore {
	return &osReviewStore{recs: recs, now: time.Now}
}

// reviewFileSuffix is what marks a file in the store as a run. Anything else in the directory is
// ignored rather than treated as a corrupt run, so an operator's stray note beside the volume does
// not break a listing.
const reviewFileSuffix = ".results.json"

// key is where a run lives, at the store root when it belongs to no project and in a per-project
// folder when it does. Runs written before projects existed sit at the root and so read back as
// belonging to no project, which is correct and needs no migration.
func (s *osReviewStore) key(parent, id string) string {
	if dir := s.dirFor(parent); dir != "" {
		return records.Join(dir, id+reviewFileSuffix)
	}
	return id + reviewFileSuffix
}

// dirFor returns the folder that holds a parent's runs, "" for the root. A parent that is not a
// well-formed project name resolves to the root. It cannot escape the store, because
// SplitReviewName has already rejected any parent that is not "projects/{id}", a project id cannot
// contain a separator, and the records store refuses a key that climbs out.
func (s *osReviewStore) dirFor(parent string) string {
	id, ok := service.ProjectID(parent)
	if !ok || id == "" {
		return ""
	}
	return id
}

// Create writes the document under a fresh id and returns its resource name and creation time.
//
// The id is "<UTC yyyymmddThhmmss>.<nanoseconds>Z-<8 hex>". Listing order is a plain string sort
// over ids, so the nanoseconds matter. A CI job reviewing several boards creates them inside one
// second, and at second resolution they would sort by the random tail instead of by time. A coarse
// clock, such as Windows' can be, gives runs made back to back the same nanosecond too, so each
// creation time is at least a nanosecond after the last one this store handed out. The tail covers a
// tie between two processes, and the store's exclusive create makes an exact collision fail rather
// than overwrite.
func (s *osReviewStore) Create(ctx context.Context, parent string, doc *checkspb.CheckResults) (string, string, error) {
	now := s.after(s.now().UTC())
	createdAt := now.Format(time.RFC3339)
	// Stamp before marshalling, so the file on disk holds the same creation time the caller gets.
	stamped, err := withCreatedAt(doc, createdAt)
	if err != nil {
		return "", "", err
	}
	// Compact, since a run holds every verdict and runs to megabytes (agni issue 1823). It is still
	// a results document results.Parse reads.
	b, err := protojson.Marshal(stamped)
	if err != nil {
		return "", "", err
	}
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", "", err
	}
	id := fmt.Sprintf("%s.%09dZ-%s", now.Format("20060102T150405"), now.Nanosecond(), hex.EncodeToString(suffix[:]))
	if err := s.recs.Create(ctx, s.key(parent, id), b); err != nil {
		return "", "", err
	}
	return service.ReviewName(parent, id), createdAt, nil
}

func (s *osReviewStore) Get(ctx context.Context, name string) (*checkspb.CheckResults, error) {
	parent, id, ok := service.SplitReviewName(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a review name", service.ErrInvalidArgument, name)
	}
	b, err := s.recs.Get(ctx, s.key(parent, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: no review %q", service.ErrNotFound, name)
	}
	if err != nil {
		return nil, err
	}
	return results.Parse(b)
}

func (s *osReviewStore) List(ctx context.Context, parent string, pageSize int, pageToken, designFilter string) ([]*checkspb.CheckResults, []string, string, error) {
	// Read one project's directory, or the root plus every project's when parent is empty. No
	// document is opened until a page is built.
	owners := []string{parent}
	if parent == "" {
		subs, err := s.recs.List(ctx, "")
		if err != nil {
			return nil, nil, "", err
		}
		for _, e := range subs {
			if e.Dir {
				owners = append(owners, service.ProjectName(e.Name))
			}
		}
	}
	var ids []string
	owner := map[string]string{}
	for _, o := range owners {
		// A project with no runs yet lists empty rather than failing.
		entries, err := s.recs.List(ctx, s.dirFor(o))
		if err != nil {
			return nil, nil, "", err
		}
		for _, e := range entries {
			if e.Dir {
				continue
			}
			if id, ok := strings.CutSuffix(e.Name, reviewFileSuffix); ok {
				ids = append(ids, id)
				owner[id] = o
			}
		}
	}
	// Ids lead with a timestamp, so a reverse string sort IS newest-first, across projects too.
	service.SortReviewIDsDescending(ids)
	// A document that will not parse is SKIPPED rather than failing the listing, so one corrupt file
	// in a long-lived volume cannot make every run unlistable.
	return service.PageReviews(ids, pageSize, pageToken, designFilter, func(id string) *checkspb.CheckResults {
		b, err := s.recs.Get(ctx, s.key(owner[id], id))
		if err != nil {
			return nil
		}
		doc, err := results.Parse(b)
		if err != nil {
			return nil
		}
		return doc
	}, func(id string) string { return service.ReviewName(owner[id], id) })
}

func (s *osReviewStore) Delete(ctx context.Context, name string) error {
	parent, id, ok := service.SplitReviewName(name)
	if !ok {
		return fmt.Errorf("%w: %q is not a review name", service.ErrInvalidArgument, name)
	}
	err := s.recs.Delete(ctx, s.key(parent, id))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: no review %q", service.ErrNotFound, name)
	}
	return err
}

// withCreatedAt returns a copy of doc with meta.created_at set, leaving the caller's document alone.
// The caller sets the same field on its original from the value this store returns.
func withCreatedAt(doc *checkspb.CheckResults, createdAt string) (*checkspb.CheckResults, error) {
	b, err := results.Marshal(doc)
	if err != nil {
		return nil, err
	}
	// Marshal/Parse rather than proto.Clone, so a document that cannot be read back in its stored
	// encoding fails here rather than at some later Get.
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

// after returns now, or a nanosecond after the last creation time when the clock has not moved past
// it, and records the time it returns.
func (s *osReviewStore) after(now time.Time) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !now.After(s.last) {
		now = s.last.Add(time.Nanosecond)
	}
	s.last = now
	return now
}

package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"google.golang.org/protobuf/proto"
)

// ReviewStore persists review runs (WS9-053). Like PartSpecStore and AnnotationStore, the interface
// lives here and the os-backed adapter in cmd/agni owns all I/O (C1/C13). Unlike them it is not keyed
// by an artifact.URI holding ONE current value, because a design accumulates MANY runs, so this store
// mints identities and lists them.
//
// Create owns identity AND time. The os adapter derives an id from the creation instant, which lets a
// listing sort chronologically without opening a document, so the two cannot be assigned separately.
// It also keeps ReviewService free of a clock, so its output is fixed for given inputs.
type ReviewStore interface {
	// Create stores a completed run under a parent project ("" for a design that belongs to none) and
	// returns its assigned name and stamped creation time. The caller supplies neither, so it cannot
	// mint a colliding id or backdate a run.
	Create(ctx context.Context, parent string, results *checkspb.CheckResults) (name string, createdAt string, err error)
	// Get returns a stored run. A name that names nothing is ErrNotFound.
	Get(ctx context.Context, name string) (*checkspb.CheckResults, error)
	// List returns runs newest first, at most pageSize of them, starting after pageToken (empty starts
	// at the newest).
	//
	// parent narrows to one project's runs, and EMPTY means every run the store holds, parented or
	// not, so a client needs one call across both name shapes. designFilter, when non-empty, keeps
	// only runs whose DesignRef.source matches it exactly. The returned token is empty on the last
	// page.
	List(ctx context.Context, parent string, pageSize int, pageToken, designFilter string) (results []*checkspb.CheckResults, names []string, nextPageToken string, err error)
	// Delete removes a stored run. Deleting an absent run is ErrNotFound rather than a success, since
	// a client removing something that is not there holds a stale view.
	Delete(ctx context.Context, name string) error
}

// reviewsSegment is the collection name every stored run carries, per AIP-122.
const reviewsSegment = "reviews/"

// ReviewName builds a resource name from a parent and a bare store id, and SplitReviewName is its
// inverse. A store deals in (parent, id) and the API in names, so both go through these two rather
// than spelling the shape by hand and ending up storing a name as an id.
//
// An empty parent yields the unparented form, "reviews/{id}", for a design that belongs to no
// project.
func ReviewName(parent, id string) string {
	if parent == "" {
		return reviewsSegment + id
	}
	return parent + "/" + reviewsSegment + id
}

// SplitReviewName splits a review resource name into its parent project name (empty when the run is
// unparented) and its store id, reporting whether the name was well formed.
//
// An empty id, a separator inside the id, or a `.`/`..` id is rejected, because an id reaches a
// filesystem-backed adapter and must not steer it out of the store directory. The parent, when
// present, is validated as a project resource name for the same reason.
func SplitReviewName(name string) (parent, id string, ok bool) {
	rest, found := strings.CutPrefix(name, reviewsSegment)
	if found {
		return "", rest, validReviewID(rest)
	}
	cut := strings.LastIndex(name, "/"+reviewsSegment)
	if cut < 0 {
		return "", "", false
	}
	parent, id = name[:cut], name[cut+len("/"+reviewsSegment):]
	if _, ok := ProjectID(parent); !ok {
		return "", "", false
	}
	return parent, id, validReviewID(id)
}

func validReviewID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\")
}

// MemReviewStore is an in-memory ReviewStore. It backs `agni review`, a thin client of CreateReview
// that must leave no files behind (--results-out is how it writes a document), and most tests.
//
// Ids are the insertion ordinal rather than a timestamp, which keeps tests deterministic and still
// lists newest first.
type MemReviewStore struct {
	mu   sync.Mutex
	seq  int
	ids  []string // insertion order, oldest first
	docs map[string]*checkspb.CheckResults
	// parents keys each stored id to its project, "" for an unparented run. It sits beside the
	// document rather than inside it because a parent is where a run LIVES, not something the run
	// recorded about itself.
	parents map[string]string
	// Clock, when set, stamps created_at. Nil leaves it empty, which the CLI wants for a run it never
	// persists.
	Clock func() string
}

// NewMemReviewStore returns an empty in-memory store.
func NewMemReviewStore() *MemReviewStore {
	return &MemReviewStore{docs: map[string]*checkspb.CheckResults{}, parents: map[string]string{}}
}

func (m *MemReviewStore) Create(_ context.Context, parent string, results *checkspb.CheckResults) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	id := fmt.Sprintf("%08d", m.seq)
	var createdAt string
	if m.Clock != nil {
		createdAt = m.Clock()
	}
	// Store a CLONE, so a later edit through the caller's pointer cannot rewrite a stored run.
	stored := proto.Clone(results).(*checkspb.CheckResults)
	m.docs[id] = stored
	m.parents[id] = parent
	m.ids = append(m.ids, id)
	return ReviewName(parent, id), createdAt, nil
}

func (m *MemReviewStore) Get(_ context.Context, name string) (*checkspb.CheckResults, error) {
	_, id, ok := SplitReviewName(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a review name", ErrInvalidArgument, name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, found := m.docs[id]
	if !found {
		return nil, fmt.Errorf("%w: no review %q", ErrNotFound, name)
	}
	return proto.Clone(doc).(*checkspb.CheckResults), nil
}

func (m *MemReviewStore) List(_ context.Context, parent string, pageSize int, pageToken, designFilter string) ([]*checkspb.CheckResults, []string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	newest := make([]string, 0, len(m.ids))
	for i := len(m.ids) - 1; i >= 0; i-- {
		id := m.ids[i]
		// An empty parent lists everything, parented or not; a set one narrows to that project.
		if parent != "" && m.parents[id] != parent {
			continue
		}
		newest = append(newest, id)
	}
	return PageReviews(newest, pageSize, pageToken, designFilter, func(id string) *checkspb.CheckResults {
		return proto.Clone(m.docs[id]).(*checkspb.CheckResults)
	}, func(id string) string { return ReviewName(m.parents[id], id) })
}

func (m *MemReviewStore) Delete(_ context.Context, name string) error {
	_, id, ok := SplitReviewName(name)
	if !ok {
		return fmt.Errorf("%w: %q is not a review name", ErrInvalidArgument, name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, found := m.docs[id]; !found {
		return fmt.Errorf("%w: no review %q", ErrNotFound, name)
	}
	delete(m.docs, id)
	delete(m.parents, id)
	m.ids = slicesDelete(m.ids, id)
	return nil
}

// PageReviews applies the filter, the page token, and the page size to an ids slice that is ALREADY
// newest-first, loading each kept document through load. Every ReviewStore pages through it, so no
// adapter can disagree about whether a token is inclusive and make a client skip or repeat a run at
// a page boundary.
//
// The token is the id to resume AFTER, so it stays valid when runs are created or deleted between
// pages, which an offset would not. nameOf builds each kept run's resource name, because only the
// store knows which parent an id lives under.
func PageReviews(newestFirst []string, pageSize int, pageToken, designFilter string, load func(string) *checkspb.CheckResults, nameOf func(string) string) ([]*checkspb.CheckResults, []string, string, error) {
	if pageSize <= 0 {
		pageSize = defaultReviewPageSize
	}
	if pageSize > maxReviewPageSize {
		pageSize = maxReviewPageSize
	}
	start := 0
	if pageToken != "" {
		found := false
		for i, id := range newestFirst {
			if id == pageToken {
				start, found = i+1, true
				break
			}
		}
		if !found {
			return nil, nil, "", fmt.Errorf("%w: page_token %q does not name a review in this listing", ErrInvalidArgument, pageToken)
		}
	}
	var docs []*checkspb.CheckResults
	var names []string
	last := ""
	for _, id := range newestFirst[start:] {
		doc := load(id)
		if doc == nil {
			continue
		}
		if designFilter != "" && doc.GetDesign().GetSource() != designFilter {
			continue
		}
		if len(docs) == pageSize {
			// One kept run beyond the page proves there is a next page, so a token never leads to an
			// empty page.
			return docs, names, last, nil
		}
		docs = append(docs, doc)
		names = append(names, nameOf(id))
		last = id
	}
	return docs, names, "", nil
}

const (
	// defaultReviewPageSize is what an unset page_size means, and maxReviewPageSize caps what a client
	// may ask for, so one request cannot make the server load an unbounded number of documents.
	defaultReviewPageSize = 50
	maxReviewPageSize     = 500
)

func slicesDelete(ids []string, want string) []string {
	out := ids[:0]
	for _, id := range ids {
		if id != want {
			out = append(out, id)
		}
	}
	return out
}

// SortReviewIDsDescending orders ids newest-first for a store whose ids are time-sortable strings,
// such as the os-backed adapter's, so it lists chronologically with no document reads.
func SortReviewIDsDescending(ids []string) {
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
}

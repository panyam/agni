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

// ReviewStore persists review runs (WS9-053). It is the third injected port in this package, after
// PartSpecStore and AnnotationStore, and follows their shape: the interface lives here, the
// os-backed adapter lives in cmd/agni and owns all I/O (C1/C13).
//
// It differs from those two in the way that matters. They are keyed by (mount, path) and hold ONE
// current value per artifact, so they need no identity and no listing. A design accumulates MANY
// review runs over time, and the history is the point, so this store mints identities and lists.
//
// Create owns identity AND time, deliberately. An id is derived from the creation instant so a
// directory listing sorts chronologically without opening a single document, which means the two
// cannot be assigned independently without them disagreeing. Putting both in the adapter also keeps
// the service pure: ReviewService composes a document from its inputs and never calls a clock, so its
// output for given inputs is fixed and its tests need no injected time.
type ReviewStore interface {
	// Create stores a completed run under a parent project ("" for a design that belongs to none) and
	// returns it with its assigned name and stamped creation time. The document passed in has no name
	// and no created_at; the store fills both, so a caller cannot mint an id that collides or backdate
	// a run.
	Create(ctx context.Context, parent string, results *checkspb.CheckResults) (name string, createdAt string, err error)
	// Get returns a stored run. A name that names nothing is ErrNotFound.
	Get(ctx context.Context, name string) (*checkspb.CheckResults, error)
	// List returns runs newest first, at most pageSize of them, starting after pageToken (empty starts
	// at the newest).
	//
	// parent narrows to one project's runs; EMPTY means every run the store holds, parented or not,
	// which is what keeps the two name shapes from costing a client an extra call. designFilter, when
	// non-empty, keeps only runs whose DesignRef.source matches it exactly. The returned token is
	// empty when the last page has been reached.
	List(ctx context.Context, parent string, pageSize int, pageToken, designFilter string) (results []*checkspb.CheckResults, names []string, nextPageToken string, err error)
	// Delete removes a stored run. Deleting an absent run is ErrNotFound, not a silent success: a
	// client asking to remove something that is not there holds a stale view and is better told.
	Delete(ctx context.Context, name string) error
}

// reviewsSegment is the collection name every stored run carries, per AIP-122.
const reviewsSegment = "reviews/"

// ReviewName builds a resource name from a parent and a bare store id, and SplitReviewName is its
// inverse. They exist so the shape is written once: a store deals in (parent, id), the API deals in
// names, and having both spell the boundary by hand is how one of them ends up storing a name as an
// id.
//
// An empty parent yields the unparented form, "reviews/{id}". That is a real state rather than a
// missing value: a design that belongs to no project has runs that belong to no project either.
func ReviewName(parent, id string) string {
	if parent == "" {
		return reviewsSegment + id
	}
	return parent + "/" + reviewsSegment + id
}

// SplitReviewName splits a review resource name into its parent project name (empty when the run is
// unparented) and its store id, reporting whether the name was well formed.
//
// An empty id, a separator inside the id, or a `.`/`..` id is rejected: an id reaches a
// filesystem-backed adapter, so a caller must not be able to steer it out of the store directory.
// The parent, when present, is validated as a project resource name for the same reason.
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

// MemReviewStore is an in-memory ReviewStore. It backs `agni review`, which is a thin client of
// CreateReview but must not leave files behind: a local run reports to stdout, and --results-out is
// the explicit way to write a document. It is also what most tests want.
//
// Ids are the insertion ordinal rather than a timestamp, which keeps every test deterministic. The
// ordering contract is what callers actually depend on (newest first), and this honors it.
type MemReviewStore struct {
	mu   sync.Mutex
	seq  int
	ids  []string // insertion order, oldest first
	docs map[string]*checkspb.CheckResults
	// parents keys each stored id to the project it belongs to, "" for an unparented run. It is kept
	// beside the document rather than inside it because a parent is where a run LIVES, not something
	// the run recorded about itself: the document already names the design it scored.
	parents map[string]string
	// Clock, when set, stamps created_at. Nil leaves it empty, which is what the CLI wants: a run it
	// never persists has no meaningful creation record to invent.
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
	// Store a CLONE. The caller keeps a pointer to the document it passed in, and a store that aliased
	// it would let a later edit through that pointer rewrite history in place.
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
// newest-first, loading each kept document through load. It is shared by every ReviewStore
// implementation so the paging contract has exactly one definition: an adapter that re-derived it
// would be free to disagree about whether a token is inclusive, and a client would silently skip or
// repeat a run at every page boundary.
//
// The token is the id to resume AFTER, so it stays valid when runs are created or deleted between
// pages. An offset would shift under exactly that, and a review store is append-mostly, so new runs
// arriving mid-pagination is the normal case rather than the edge one.
// nameOf builds each kept run's resource name, which the store supplies because only it knows which
// parent an id lives under. It is a parameter rather than a fixed ReviewName call so an adapter that
// stores parented and unparented runs together does not have to re-derive the shape and get it
// subtly different from this one.
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
			// One kept run beyond the page proves there is a next page. Emitting a token without that
			// proof would hand back a token that returns nothing, which reads to a client as data loss.
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

// SortReviewIDsDescending orders ids newest-first for a store whose ids are time-sortable strings. It is the
// property the os-backed adapter's id scheme buys: chronological order with no document reads.
func SortReviewIDsDescending(ids []string) {
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
}

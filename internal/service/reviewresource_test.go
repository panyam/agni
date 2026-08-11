package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/internal/artifact"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// dirReviewLoader reads its checklist from a real directory, so a test can EDIT the manifest between
// a create and a get. That is the only way to exercise what the snapshot exists for: with a manifest
// value alone there is no file to go stale.
type dirReviewLoader struct{ dir string }

func (l dirReviewLoader) Design(context.Context, artifact.URI, ...ReadOption) (*ir.Design, error) {
	return &ir.Design{}, nil
}
func (l dirReviewLoader) Board(context.Context, artifact.URI) (*geom.BoardGeometry, error) {
	return nil, nil
}
func (l dirReviewLoader) DesignHash(context.Context, artifact.URI) (string, error) {
	return "sha256:fixed", nil
}
func (l dirReviewLoader) Manifest(_ context.Context, uri artifact.URI) (review.Manifest, error) {
	f, err := os.Open(filepath.Join(l.dir, uri.Path))
	if err != nil {
		return review.Manifest{}, err
	}
	defer f.Close()
	return review.Load(f)
}

func writeManifest(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dirReviewSvc(t *testing.T, store ReviewStore) (*ReviewService, string) {
	t.Helper()
	dir := t.TempDir()
	return NewReviewService(dirReviewLoader{dir: dir}, store, check.DefaultCatalog(), nil, nil, testReviewEnv, ""), dir
}

// TestStoredRunKeepsTheChecklistItScored is the test this whole feature exists to make possible.
//
// A run is created against a checklist, and the checklist file is then EDITED: an item is removed and
// another's title is rewritten. Refetching the stored run must still describe what was actually asked
// at the time, not what the file says now.
//
// Before the snapshot, a document recorded only the manifest's NAME, so re-rendering an archived run
// meant re-reading whatever that name resolves to today. The failure is quiet and total: last
// quarter's review renders against this quarter's checklist, with every outcome intact and the
// questions swapped underneath it. That is worse than losing the run.
func TestStoredRunKeepsTheChecklistItScored(t *testing.T) {
	svc, dir := dirReviewSvc(t, NewMemReviewStore())
	ctx := context.Background()
	writeManifest(t, dir, "review.yaml", `
name: gateway review
areas:
  - name: Power
    items:
      - {id: "P1", title: "every rail carries a bulk capacitor", rule: bulk-cap}
      - {id: "P2", title: "reviewed by hand", note: "the EE signs this off"}
`)
	man, err := svc.GetReviewManifest(ctx, &webapi.GetReviewManifestRequest{Uri: "mount://m/review.yaml"})
	if err != nil {
		t.Fatalf("GetReviewManifest: %v", err)
	}
	created, err := svc.CreateReview(ctx, &webapi.CreateReviewRequest{
		Manifest: man.GetManifest(), DesignUri: "mount://m/board.edn",
	})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}

	// The checklist changes after the run: P2 is dropped and P1 is reworded.
	writeManifest(t, dir, "review.yaml", `
name: gateway review
areas:
  - name: Power
    items:
      - {id: "P1", title: "COMPLETELY DIFFERENT QUESTION", rule: bulk-cap}
`)

	got, err := svc.GetReview(ctx, &webapi.GetReviewRequest{Name: created.GetName()})
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	snap := got.GetResults().GetManifestSnapshot()
	if snap == nil {
		t.Fatal("the stored run carries no manifest snapshot, so it cannot say what it asked")
	}
	items := snap.GetAreas()[0].GetItems()
	if len(items) != 2 {
		t.Fatalf("snapshot has %d items, want the 2 the run actually scored", len(items))
	}
	if got := items[0].GetTitle(); got != "every rail carries a bulk capacitor" {
		t.Errorf("snapshot item P1 title = %q; the archived run adopted the edited checklist", got)
	}
	if got := items[1].GetId(); got != "P2" {
		t.Errorf("snapshot item 1 = %q, want P2: an item removed after the run vanished from its own record", got)
	}
	// The outcomes must still line up with the snapshot, or the document contradicts itself.
	if n := len(got.GetResults().GetAreas()[0].GetItems()); n != 2 {
		t.Errorf("stored run has %d outcomes for %d asked items", n, len(items))
	}
}

// TestCreateReviewRecordsProvenance: a stored run carries what a reader needs to trust it later, and
// each field here is one a consumer would otherwise have to guess. The design hash is the revision
// identity a later diff joins on; the catalog snapshot is the difference between a clean design and a
// run that checked nothing; RunConfig.params says whether a datasheet-backed item COULD have resolved.
func TestCreateReviewRecordsProvenance(t *testing.T) {
	svc, dir := dirReviewSvc(t, NewMemReviewStore())
	writeManifest(t, dir, "m.yaml", "name: t\nareas: [{name: A, items: [{id: i, rule: bulk-cap}]}]\n")
	ctx := context.Background()
	man, err := svc.GetReviewManifest(ctx, &webapi.GetReviewManifestRequest{Uri: "mount://m/m.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	rv, err := svc.CreateReview(ctx, &webapi.CreateReviewRequest{Manifest: man.GetManifest(), DesignUri: "mount://m/board.edn", RatifiedFloor: 0.5})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	doc := rv.GetResults()
	if doc.GetDesign().GetSource() != "board.edn" || doc.GetDesign().GetContentHash() != "sha256:fixed" {
		t.Errorf("design ref = %+v, want the ref and its hash", doc.GetDesign())
	}
	if doc.GetMeta().GetProducerVersion() != "test" || !doc.GetMeta().GetCoverageAxis() {
		t.Errorf("meta = %+v, want the producer version and a declared coverage axis", doc.GetMeta())
	}
	if len(doc.GetCatalog()) == 0 {
		t.Error("catalog snapshot is empty: a reader cannot tell a clean design from a run that asked nothing")
	}
	if doc.GetRun().GetRatifiedFloor() != 0.5 || doc.GetRun().GetParams() {
		t.Errorf("run config = %+v, want the floor recorded and params false (no corpus wired)", doc.GetRun())
	}
	if doc.GetManifest() != "t" {
		t.Errorf("manifest name = %q, want it alongside the snapshot for cheap listing", doc.GetManifest())
	}
}

// TestReviewResourceLifecycle walks create, get, list, delete over one service, which is the sequence
// a client actually performs.
func TestReviewResourceLifecycle(t *testing.T) {
	svc, dir := dirReviewSvc(t, NewMemReviewStore())
	writeManifest(t, dir, "m.yaml", "name: t\nareas: [{name: A, items: [{id: i, rule: bulk-cap}]}]\n")
	ctx := context.Background()
	man, err := svc.GetReviewManifest(ctx, &webapi.GetReviewManifestRequest{Uri: "mount://m/m.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	create := func(design string) string {
		t.Helper()
		rv, err := svc.CreateReview(ctx, &webapi.CreateReviewRequest{Manifest: man.GetManifest(), DesignUri: design})
		if err != nil {
			t.Fatalf("CreateReview(%s): %v", design, err)
		}
		return rv.GetName()
	}
	first := create("a.edn")
	second := create("b.edn")
	third := create("a.edn")

	// Newest first, so the most recent run is what a client sees without paging.
	list, err := svc.ListReviews(ctx, &webapi.ListReviewsRequest{})
	if err != nil {
		t.Fatalf("ListReviews: %v", err)
	}
	if got := names(list); len(got) != 3 || got[0] != third || got[2] != first {
		t.Fatalf("listing = %v, want newest first (%s, %s, %s)", got, third, second, first)
	}

	// The design filter answers the question a client actually has.
	only, err := svc.ListReviews(ctx, &webapi.ListReviewsRequest{Filter: `design="a.edn"`})
	if err != nil {
		t.Fatalf("ListReviews(filter): %v", err)
	}
	if got := names(only); len(got) != 2 || got[0] != third || got[1] != first {
		t.Errorf("filtered listing = %v, want the two a.edn runs newest first", got)
	}

	if _, err := svc.DeleteReview(ctx, &webapi.DeleteReviewRequest{Name: second}); err != nil {
		t.Fatalf("DeleteReview: %v", err)
	}
	if _, err := svc.GetReview(ctx, &webapi.GetReviewRequest{Name: second}); err == nil {
		t.Error("a deleted review still resolves")
	}
	if _, err := svc.DeleteReview(ctx, &webapi.DeleteReviewRequest{Name: second}); err == nil {
		t.Error("deleting an absent review reported success; a client acting on a stale listing is never told")
	}
}

// TestListReviewsPaging: a page carries at most page_size, the token resumes AFTER the last item
// without repeating or skipping it, and the final page reports no token. A token is only emitted when
// there is genuinely something after it, so a client never follows one into an empty page.
func TestListReviewsPaging(t *testing.T) {
	svc, dir := dirReviewSvc(t, NewMemReviewStore())
	writeManifest(t, dir, "m.yaml", "name: t\nareas: [{name: A, items: [{id: i, rule: bulk-cap}]}]\n")
	ctx := context.Background()
	man, err := svc.GetReviewManifest(ctx, &webapi.GetReviewManifestRequest{Uri: "mount://m/m.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	var created []string
	for i := range 5 {
		rv, err := svc.CreateReview(ctx, &webapi.CreateReviewRequest{Manifest: man.GetManifest(), DesignUri: fmt.Sprintf("d%d.edn", i)})
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, rv.GetName())
	}
	var walked []string
	token := ""
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("paging did not terminate")
		}
		page, err := svc.ListReviews(ctx, &webapi.ListReviewsRequest{PageSize: 2, PageToken: token})
		if err != nil {
			t.Fatalf("ListReviews: %v", err)
		}
		if len(page.GetReviews()) > 2 {
			t.Fatalf("page carried %d reviews, want at most 2", len(page.GetReviews()))
		}
		walked = append(walked, names(page)...)
		if page.GetNextPageToken() == "" {
			break
		}
		token = page.GetNextPageToken()
	}
	if len(walked) != 5 {
		t.Fatalf("walked %d reviews across pages, want 5 with none skipped or repeated: %v", len(walked), walked)
	}
	for i, name := range walked {
		if want := created[len(created)-1-i]; name != want {
			t.Errorf("page walk position %d = %s, want %s (newest first)", i, name, want)
		}
	}
}

// TestReviewResourcesNeedAStore: without a configured store every resource method reports it, and the
// message names the flag. A create that ran the whole sweep and silently dropped the result would cost
// the most and leave the least.
func TestReviewResourcesNeedAStore(t *testing.T) {
	svc := NewReviewService(dirReviewLoader{dir: t.TempDir()}, nil, check.DefaultCatalog(), nil, nil, testReviewEnv, "")
	ctx := context.Background()
	man := &checkspb.ReviewManifest{Name: "t", Areas: []*checkspb.ManifestArea{{
		Name: "A", Items: []*checkspb.ManifestItem{{Id: "i"}},
	}}}
	calls := map[string]func() error{
		"CreateReview": func() error {
			_, err := svc.CreateReview(ctx, &webapi.CreateReviewRequest{Manifest: man, DesignUri: "mount://m/d.edn"})
			return err
		},
		"GetReview": func() error {
			_, err := svc.GetReview(ctx, &webapi.GetReviewRequest{Name: "reviews/x"})
			return err
		},
		"ListReviews": func() error {
			_, err := svc.ListReviews(ctx, &webapi.ListReviewsRequest{})
			return err
		},
		"DeleteReview": func() error {
			_, err := svc.DeleteReview(ctx, &webapi.DeleteReviewRequest{Name: "reviews/x"})
			return err
		},
	}
	for name, call := range calls {
		err := call()
		if err == nil {
			t.Errorf("%s with no store: want an error, got nil", name)
			continue
		}
		if !errors.Is(err, ErrReviewStoreNotConfigured) {
			t.Errorf("%s with no store: err = %v, want ErrReviewStoreNotConfigured so the transport can say which flag is missing", name, err)
		}
	}
}

// TestParseReviewFilter: the supported filter parses with or without quotes, and anything else is
// REJECTED. Silently ignoring an unsupported filter is the dangerous case, because a client that
// believed it had narrowed to its own board would read another board's verdicts as its own.
func TestParseReviewFilter(t *testing.T) {
	ok := map[string]string{
		``:                   "",
		`design="a/b.edn"`:   "a/b.edn",
		`design=a/b.edn`:     "a/b.edn",
		` design="a/b.edn" `: "a/b.edn",
	}
	for in, want := range ok {
		got, err := parseReviewFilter(in)
		if err != nil {
			t.Errorf("parseReviewFilter(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseReviewFilter(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{`manifest="t"`, `design`, `design=""`, `design = "a"`, `name!="x"`} {
		if _, err := parseReviewFilter(bad); err == nil {
			t.Errorf("parseReviewFilter(%q): want an error, got nil (an ignored filter reads as a narrowed one)", bad)
		}
	}
}

// TestReviewNameRoundTrip: a name survives the id boundary, and a name that could steer a
// filesystem-backed store out of its directory is rejected rather than resolved.
func TestReviewNameRoundTrip(t *testing.T) {
	if id, ok := ReviewID(ReviewName("abc")); !ok || id != "abc" {
		t.Errorf("ReviewID(ReviewName(abc)) = %q,%v", id, ok)
	}
	for _, bad := range []string{"abc", "reviews/", "reviews/../etc/passwd", "reviews/a/b", `reviews/a\b`, "reviews/.", "reviews/.."} {
		if _, ok := ReviewID(bad); ok {
			t.Errorf("ReviewID(%q) accepted a name that must not reach a store", bad)
		}
	}
}

func names(list *webapi.ListReviewsResponse) []string {
	var out []string
	for _, rv := range list.GetReviews() {
		out = append(out, rv.GetName())
	}
	return out
}

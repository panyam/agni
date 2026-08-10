package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/core/results"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/internal/service"
)

func storeDoc(design string) *checkspb.CheckResults {
	return &checkspb.CheckResults{
		Meta:     &checkspb.ResultsMeta{Schema: results.Schema, Producer: results.Producer, CoverageAxis: true},
		Design:   &checkspb.DesignRef{Source: design},
		Manifest: "t",
		Areas: []*checkspb.ReviewArea{{
			Name:  "A",
			Items: []*checkspb.ReviewItem{{Id: "i", Title: "an item", Outcome: "pass"}},
		}},
		ManifestSnapshot: &checkspb.ReviewManifest{Name: "t", Areas: []*checkspb.ManifestArea{{
			Name: "A", Items: []*checkspb.ManifestItem{{Id: "i", Title: "an item"}},
		}}},
	}
}

// TestOSReviewStoreRoundTrip: a document written to the volume comes back identical in the fields a
// consumer renders, and it is on disk as a readable results document rather than a private encoding,
// so the volume stays inspectable with the same tools that read `--results-out` output.
func TestOSReviewStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := newOSReviewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	name, createdAt, err := st.Create(ctx, storeDoc("board.edn"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if createdAt == "" {
		t.Error("Create returned no creation time; the store owns the clock")
	}
	got, err := st.Get(ctx, name)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.GetDesign().GetSource() != "board.edn" || got.GetManifest() != "t" {
		t.Errorf("round trip lost identity: %+v", got)
	}
	if got.GetManifestSnapshot().GetAreas()[0].GetItems()[0].GetTitle() != "an item" {
		t.Errorf("round trip lost the manifest snapshot: %+v", got.GetManifestSnapshot())
	}
	// The stamped time must be what is ON DISK, not only what the caller was handed back, or a re-read
	// of the run would disagree with the response that created it.
	if got.GetMeta().GetCreatedAt() != createdAt {
		t.Errorf("stored created_at = %q, returned %q", got.GetMeta().GetCreatedAt(), createdAt)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*"+reviewFileSuffix))
	if len(files) != 1 {
		t.Fatalf("volume holds %d run files, want 1: %v", len(files), files)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := results.Parse(b); err != nil {
		t.Errorf("the file on the volume is not a readable results document: %v", err)
	}
}

// TestOSReviewStoreListsNewestFirst is the property the id scheme buys: ids lead with a UTC timestamp,
// so ordering a listing is a string sort over filenames with no document opened.
func TestOSReviewStoreListsNewestFirst(t *testing.T) {
	st, err := newOSReviewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var created []string
	for _, d := range []string{"a.edn", "b.edn", "c.edn"} {
		name, _, err := st.Create(ctx, storeDoc(d))
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, name)
	}
	_, names, _, err := st.List(ctx, 0, "", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("listed %d runs, want 3", len(names))
	}
	for i, name := range names {
		if want := created[len(created)-1-i]; name != want {
			t.Errorf("position %d = %s, want %s (newest first)", i, name, want)
		}
	}
	_, filtered, _, err := st.List(ctx, 0, "", "b.edn")
	if err != nil {
		t.Fatalf("List(filter): %v", err)
	}
	if len(filtered) != 1 || filtered[0] != created[1] {
		t.Errorf("filtered listing = %v, want only the b.edn run", filtered)
	}
}

// TestOSReviewStoreSkipsUnreadableFiles: a long-lived volume accumulates whatever an operator or a
// half-finished write leaves there. One bad file must not make every OTHER run unlistable, because the
// error a user would meet would name pagination rather than the file, and the runs that are fine would
// look lost.
func TestOSReviewStoreSkipsUnreadableFiles(t *testing.T) {
	dir := t.TempDir()
	st, err := newOSReviewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	good, _, err := st.Create(ctx, storeDoc("a.edn"))
	if err != nil {
		t.Fatal(err)
	}
	// A truncated write, and an operator's note that is not a run at all.
	if err := os.WriteFile(filepath.Join(dir, "20990101T000000.000000000Z-deadbeef"+reviewFileSuffix), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("this volume holds review runs"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, names, _, err := st.List(ctx, 0, "", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 1 || names[0] != good {
		t.Errorf("listing = %v, want only the readable run %s", names, good)
	}
}

// TestOSReviewStoreMissingAndBadNames: an absent run is ErrNotFound on both Get and Delete, and a name
// that would escape the store directory is refused before it reaches the filesystem.
func TestOSReviewStoreMissingAndBadNames(t *testing.T) {
	dir := t.TempDir()
	st, err := newOSReviewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := st.Get(ctx, service.ReviewName("nope")); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Get(absent) = %v, want ErrNotFound", err)
	}
	if err := st.Delete(ctx, service.ReviewName("nope")); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Delete(absent) = %v, want ErrNotFound", err)
	}
	// The traversal attempt must fail as a bad NAME, and must not have written or read anything outside
	// the store. Checking the parent directory is untouched is the part that matters.
	outside := filepath.Join(dir, "..", "escaped"+reviewFileSuffix)
	if _, err := st.Get(ctx, "reviews/../escaped"); err == nil {
		t.Error("Get accepted a traversing name")
	}
	if _, err := os.Stat(outside); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a traversing name touched %s", outside)
	}
}

// TestNewOSReviewStoreCreatesAndValidates: a fresh volume works on first boot, and a path that is not
// a directory fails at startup where an operator can still fix it rather than at the first create.
func TestNewOSReviewStoreCreatesAndValidates(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "reviews", "runs")
	if _, err := newOSReviewStore(nested); err != nil {
		t.Fatalf("a fresh volume path should be created, got %v", err)
	}
	if info, err := os.Stat(nested); err != nil || !info.IsDir() {
		t.Errorf("store directory was not created: %v", err)
	}
	file := filepath.Join(base, "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newOSReviewStore(file); err == nil {
		t.Error("--review-store at a regular file must fail at startup")
	}
}

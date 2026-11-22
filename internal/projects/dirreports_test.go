package projects

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/agni/service"
)

// A report the store names sorts by when it was kept, the listing is newest first, and a design's
// reports go with DeleteReports (agni issue 1704).
func TestDirReportsKeepsAndListsNewestFirst(t *testing.T) {
	ctx := context.Background()
	rs := NewReports(recordstest.New(t))
	clock := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	rs.now = func() time.Time { return clock }
	first, err := rs.CreateReport(ctx, "gw", "", &webapi.Report{Packs: []string{"packs/a"}, DesignChanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^designs/gw/reports/20261118t100000-[0-9a-f]{8}$`).MatchString(first.GetName()) || first.GetDesignChanged() {
		t.Errorf("minted report = %s, changed %v; want a time-sorted id, and no stale mark kept", first.GetName(), first.GetDesignChanged())
	}
	clock = clock.Add(time.Minute)
	if _, err := rs.CreateReport(ctx, "gw", "later", &webapi.Report{}); err != nil {
		t.Fatal(err)
	}
	if _, err := rs.CreateReport(ctx, "gw", "later", &webapi.Report{}); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("a second create of later = %v; want ErrAlreadyExists", err)
	}
	all, err := rs.Reports(ctx, "gw")
	if err != nil || len(all) != 2 || all[0].GetName() != "designs/gw/reports/later" || all[1].GetName() != first.GetName() {
		t.Fatalf("Reports = %v, %v; want later, then the first", all, err)
	}
	if got, err := rs.Report(ctx, "gw", "later"); err != nil || got.GetCreateTime().AsTime() != clock {
		t.Errorf("Report(later) = %v, %v; want it stamped with the store's clock", got, err)
	}
	if err := rs.DeleteReport(ctx, "gw", "later"); err != nil {
		t.Fatal(err)
	}
	if err := rs.DeleteReport(ctx, "gw", "later"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("a second delete = %v; want ErrNotFound", err)
	}
	if err := rs.DeleteReports(ctx, "gw"); err != nil {
		t.Fatal(err)
	}
	if left, err := rs.Reports(ctx, "gw"); err != nil || len(left) != 0 {
		t.Errorf("after DeleteReports, Reports = %v, %v; want none", left, err)
	}
	if _, err := rs.CreateReport(ctx, "../x", "r", &webapi.Report{}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a report under ../x = %v; want ErrInvalidArgument", err)
	}
	if _, err := rs.Report(ctx, "gw", "../r"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Report(../r) = %v; want ErrNotFound", err)
	}
}

// A stored report, pack or project is compact JSON. A report on a real board runs to megabytes, and
// indentation added about a third to it (agni issue 1823).
func TestStoredRecordsAreCompactJSON(t *testing.T) {
	ctx := context.Background()
	recs := recordstest.New(t)
	if _, err := NewReports(records.Sub(recs, "reports")).CreateReport(ctx, "gw", "r", &webapi.Report{Packs: []string{"packs/a", "packs/b"}}); err != nil {
		t.Fatal(err)
	}
	if err := NewProjects(records.Sub(recs, "projects")).Put(ctx, "p", &webapi.Project{Name: "projects/p", Designs: []string{"designs/a", "designs/b"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPacks(records.Sub(recs, "packs"), nil, "", "uploads").CreatePack(ctx, "k", &webapi.Pack{Title: "K"}, nil); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"reports/gw/r.json", "projects/p.json", "packs/k.json"} {
		b, err := recs.Get(ctx, k)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "\n") {
			t.Errorf("%s is indented:\n%s", k, b)
		}
	}
}

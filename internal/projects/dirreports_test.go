package projects

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// A report the store names sorts by when it was kept, the listing is newest first, and a design's
// reports go with DeleteReports (agni issue 1704).
func TestDirReportsKeepsAndListsNewestFirst(t *testing.T) {
	ctx := context.Background()
	rs, err := NewDirReports(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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

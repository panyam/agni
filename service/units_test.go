package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// A relation whose numbers all share a unit declares it, so the engine refuses an ordering
// comparison between two units before the query runs rather than answering nothing.
func TestComparingTwoDeclaredUnitsIsRefused(t *testing.T) {
	svc, uri := tutorialQueries()
	ask := func(q string) error {
		_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: uri, Query: q})
		return err
	}
	err := ask(`net.max_voltage(?n, ?v), board.track_width(?n, ?w), ?v < ?w => ?n`)
	if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "number[V]") || !strings.Contains(err.Error(), "number[m]") {
		t.Errorf("volts < metres: err = %v, want an invalid argument naming both units", err)
	}
	if err := ask(`net.max_voltage(?n, ?v), net.nominal_voltage(?n, ?w), ?v < ?w => ?n`); err != nil {
		t.Errorf("volts < volts: err = %v, want it answered", err)
	}
}

// A sum over one part's limits in volts and amps has no single unit, so the cell is absent and the
// row says why, through RunQuery and through the table every csv encodes.
func TestAnAbsentAggregateSaysWhy(t *testing.T) {
	project := filepath.Join("..", "examples", "tutorial-project")
	specs, err := param.LoadSet(os.DirFS(filepath.Join(project, "params")))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewQueryService(fsQueryLoader{base: project}, specs, nil)
	uri := "mount://m/designs/gateway/gateway.edn"
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: uri, Query: `param.max(?m, ?s, ?v) => ?m, sum(?v)`})
	if err != nil {
		t.Fatal(err)
	}
	mixed, single := 0, 0
	for _, r := range resp.GetRows() {
		why := r.GetCellWhy()
		if r.GetCells()[1] == "" {
			mixed++
			if len(why) != 2 || !strings.Contains(why[1], "mixed units") {
				t.Errorf("%s: absent sum has cell_why %q, want its reason in column 1", r.GetCells()[0], why)
			}
		} else {
			single++
			if len(why) != 0 {
				t.Errorf("%s: sum %s carries cell_why %q", r.GetCells()[0], r.GetCells()[1], why)
			}
		}
	}
	if mixed == 0 || single == 0 {
		t.Fatalf("%d mixed-unit and %d single-unit sums on the fixture; want at least one of each", mixed, single)
	}
	for _, r := range QueryTable("query", resp).GetRows() {
		if r.GetCells()[1] == "" && (len(r.GetCellWhy()) != len(r.GetCells()) || r.GetCellWhy()[1] == "") {
			t.Errorf("table row %q lost the reason: cell_why %q", r.GetCells()[0], r.GetCellWhy())
		}
	}
}

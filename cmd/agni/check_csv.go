package main

import (
	"io"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// checkCSVColumns is the header of `check --format csv`, the findings table's columns
// (service.FindingColumns), named here for the tests that bind to it.
var checkCSVColumns = columnNames(service.FindingColumns)

// writeCheckCSV emits the findings table in the order the run produced it. Run order is already
// deterministic (catalog order, then entity order within a rule), so it keeps the csv in the same
// order as the json (TestCheckCSVMatchesJSON) unless --order-by asks otherwise.
func writeCheckCSV(w io.Writer, findings []*checkspb.Finding, orderBy ...string) error {
	return writeTableCSV(w, service.FindingsTable(findings), orderBy)
}

// writeTableCSV encodes a projected table as csv, ordered by orderBy first. The rows are the
// TableService's, so a csv and a workbook built from one answer carry the same cells (agni issue
// 862); escaping against spreadsheet formulas happens in service.WriteTableCSV, the encoder the
// Encode rpc shares, so a file the viewer saves is these bytes.
func writeTableCSV(w io.Writer, t *webapi.Table, orderBy []string) error {
	if err := service.OrderTables([]*webapi.Table{t}, orderBy, nil); err != nil {
		return err
	}
	return service.WriteTableCSV(w, t)
}

func columnNames(cs []webapi.TableColumn) []string {
	out := make([]string, 0, len(cs))
	for i := range cs {
		out = append(out, cs[i].Name)
	}
	return out
}

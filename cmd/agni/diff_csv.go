package main

import (
	"io"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// diffCSVColumns is the header of `diff --format csv`, the diff table's columns
// (service.DiffColumns), named here for the tests that bind to it.
var diffCSVColumns = columnNames(service.DiffColumns)

// writeDiffCSV encodes the diff table the TableService projects from the DiffDesigns response, so the
// csv and a workbook of one diff carry the same rows (agni issue 862). Its order is the diff's own
// unless --order-by asks otherwise.
func writeDiffCSV(w io.Writer, resp *webapi.DiffDesignsResponse, orderBy []string) error {
	return writeTableCSV(w, service.DiffTable(resp), orderBy)
}

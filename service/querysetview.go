package service

import (
	"fmt"

	"github.com/panyam/agni/core/query"
	rpt "github.com/panyam/agni/core/report"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// QueryResultTable is one query's answer as the report model renders it, the projection `query
// --format html|markdown` writes and the check report's Queries sections reuse. Per-cell sheet badges
// and locate reasons are panel navigation and mean nothing in a file, so they stop here.
//
// source is the design's URI, never a host path, since a saved view gets committed and pasted into
// tickets (agni issue 501).
func QueryResultTable(resp *webapi.RunQueryResponse, title, queryText, source string) rpt.Table {
	t := rpt.Table{Title: title, Query: queryText, Source: source, Columns: resp.GetColumns(),
		Bindings: FormatBindings(resp.GetBindings()), Total: int(resp.GetTotal())}
	for _, r := range resp.GetRows() {
		t.Rows = append(t.Rows, rpt.TableRow{Cells: r.GetCells(), Cites: RowCites(resp, r), Why: r.GetCellWhy(), Witnesses: RowWitnesses(resp, r)})
	}
	return t
}

// QuerySetTable is a query set's answers as one report table set, a section per query in the set's
// order. failed gives the text and bindings of a query that has no result, which the response does
// not echo; nil leaves a failed section with its error alone.
func QuerySetTable(resp *webapi.RunQueriesResponse, failed func(i int) rpt.Table) rpt.TableSet {
	ts := rpt.TableSet{Title: resp.GetTitle(), Source: resp.GetSource(), Preamble: resp.GetPreamble()}
	for i, r := range resp.GetResults() {
		sec := rpt.TableSection{Name: r.GetName(), Description: r.GetDescription(), Error: r.GetError()}
		if at := r.GetErrorAt(); at != nil {
			sec.Error += fmt.Sprintf(" (line %d, column %d of the query)", at.GetLine(), at.GetColumn())
		}
		switch {
		case r.GetResult() != nil:
			sec.Table = QueryResultTable(r.GetResult(), "", r.GetResult().GetQuery(), "")
		case failed != nil:
			sec.Table = failed(i)
		}
		ts.Sections = append(ts.Sections, sec)
	}
	return ts
}

// FormatBindings spells an answer's echoed bindings as a document states them beside its query, or
// gives the reason they could not be read.
func FormatBindings(m map[string]*webapi.QueryValue) []string {
	b, err := BindingsFromProto(m)
	if err != nil {
		return []string{err.Error()}
	}
	return query.FormatBindings(b)
}
